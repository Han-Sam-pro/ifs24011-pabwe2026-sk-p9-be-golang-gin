package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"app/internal/database"
	"app/internal/features/post"
)

// Error domain AI generator.
var (
	ErrAIUnavailable     = errors.New("layanan AI tidak dapat dihubungi")
	ErrInvalidAIResponse = errors.New("respons AI tidak dapat diproses")
)

// DefaultMaxSources adalah jumlah sumber penelusuran default.
const DefaultMaxSources = 5

// Service adalah kontrak AI Blog Generator.
type Service interface {
	Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error)
}

type service struct {
	search Searcher
	llm    Completer
}

// NewService membuat service AI generator.
func NewService(search Searcher, llm Completer) Service {
	return &service{search: search, llm: llm}
}

// BuildMessages menyusun prompt sistem + topik + konteks hasil penelusuran.
func BuildMessages(req GenerateRequest, category string, sources []SearchResult) []Message {
	system := "Anda adalah penulis blog profesional berbahasa Indonesia untuk platform Delcom Blog. " +
		"Tulis artikel orisinal, akurat, terstruktur, dan mudah dibaca dalam format Markdown " +
		"(gunakan heading ##, paragraf, daftar, dan penekanan bila perlu). " +
		"Balas HANYA dengan satu objek JSON valid tanpa teks lain dengan kunci: " +
		`"title" (judul menarik), "category" (salah satu dari: ` + strings.Join(database.Categories, ", ") + `), ` +
		`"excerpt" (ringkasan pemikat 1-2 kalimat), "content" (isi artikel Markdown lengkap, 600-1200 kata, tanpa judul H1).`

	var b strings.Builder
	fmt.Fprintf(&b, "Topik: %s\nKategori: %s\n", req.Topic, category)
	if req.Tone != "" {
		fmt.Fprintf(&b, "Gaya bahasa: %s\n", req.Tone)
	}
	if req.Keywords != "" {
		fmt.Fprintf(&b, "Kata kunci: %s\n", req.Keywords)
	}
	if len(sources) > 0 {
		b.WriteString("\nGunakan hasil penelusuran mesin pencari terkini berikut sebagai konteks fakta. " +
			"Kutip dengan penanda [n] di dalam teks dan akhiri konten dengan bagian \"## Sumber Rujukan\" " +
			"berisi daftar tautan Markdown sesuai nomor.\n")
		for i, s := range sources {
			fmt.Fprintf(&b, "[%d] %s\nURL: %s\nCuplikan: %s\n", i+1, s.Title, s.URL, s.Snippet)
		}
	}
	return []Message{{Role: "system", Content: system}, {Role: "user", Content: b.String()}}
}

// ParseArticle mengekstrak objek JSON artikel dari jawaban model
// (tahan terhadap pembungkus ```json ... ``` maupun teks tambahan).
func ParseArticle(raw string) (*GeneratedArticle, error) {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end <= start {
		return nil, ErrInvalidAIResponse
	}
	var a GeneratedArticle
	if err := json.Unmarshal([]byte(raw[start:end+1]), &a); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidAIResponse, err)
	}
	if strings.TrimSpace(a.Content) == "" {
		return nil, ErrInvalidAIResponse
	}
	return &a, nil
}

// AppendSources menambahkan daftar sumber rujukan jika model belum menuliskannya.
func AppendSources(content string, sources []SearchResult) string {
	if len(sources) == 0 || strings.Contains(strings.ToLower(content), "sumber rujukan") {
		return content
	}
	var b strings.Builder
	b.WriteString(strings.TrimRight(content, "\n"))
	b.WriteString("\n\n## Sumber Rujukan\n\n")
	for i, s := range sources {
		title := s.Title
		if title == "" {
			title = s.URL
		}
		fmt.Fprintf(&b, "%d. [%s](%s)\n", i+1, title, s.URL)
	}
	return b.String()
}

func (s *service) complete(ctx context.Context, req GenerateRequest, category string, sources []SearchResult) (*GeneratedArticle, error) {
	raw, err := s.llm.Complete(ctx, BuildMessages(req, category, sources))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAIUnavailable, err)
	}
	return ParseArticle(raw)
}

// FallbackArticle menyusun draf awal terstruktur dari topik dan sumber web.
func FallbackArticle(topic, category string, sources []SearchResult) *GeneratedArticle {
	var b strings.Builder
	fmt.Fprintf(&b, "## Pendahuluan\n\nArtikel ini membahas **%s**. Lengkapi bagian ini dengan latar belakang dan alasan topik ini penting bagi pembaca.\n\n", topic)
	b.WriteString("## Pembahasan\n\n- Poin utama pertama\n- Poin utama kedua\n- Poin utama ketiga\n\n")
	for i, s := range sources {
		fmt.Fprintf(&b, "> %s [%d]\n\n", s.Snippet, i+1)
	}
	b.WriteString("## Kesimpulan\n\nRangkum gagasan utama dan ajak pembaca untuk berdiskusi.\n")
	return &GeneratedArticle{
		Title:    topic,
		Category: category,
		Excerpt:  fmt.Sprintf("Draf awal artikel %s tentang %s.", strings.ToLower(category), topic),
		Content:  b.String(),
	}
}

func (s *service) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error) {
	category, err := post.NormalizeCategory(req.Category)
	if err != nil {
		return nil, err
	}
	limit := req.MaxSources
	if limit <= 0 {
		limit = DefaultMaxSources
	}

	res := &GenerateResponse{Sources: []SearchResult{}}
	if req.IncludeWebSearch {
		sources, err := s.search.Search(ctx, req.Topic, limit)
		if err != nil {
			res.SearchWarning = "Penelusuran mesin pencari gagal, artikel dibuat tanpa referensi web: " + err.Error()
		} else {
			res.Sources = sources
			res.WebSearchUsed = len(sources) > 0
		}
	}

	article, err := s.complete(ctx, req, category, res.Sources)
	if err != nil {
		// Layanan AI gagal/tidak dikonfigurasi: tetap kembalikan draf awal
		// berbasis template agar penulis dapat melanjutkan di editor.
		res.AIWarning = "Layanan AI tidak tersedia, draf dibuat dari template: " + err.Error()
		article = FallbackArticle(req.Topic, category, res.Sources)
	} else {
		res.AIUsed = true
	}

	res.Title = strings.TrimSpace(article.Title)
	if res.Title == "" {
		res.Title = req.Topic
	}
	res.Category = category
	if c, err := post.NormalizeCategory(article.Category); err == nil && article.Category != "" && req.Category == "" {
		res.Category = c
	}
	res.Excerpt = strings.TrimSpace(article.Excerpt)
	res.Content = AppendSources(article.Content, res.Sources)
	return res, nil
}
