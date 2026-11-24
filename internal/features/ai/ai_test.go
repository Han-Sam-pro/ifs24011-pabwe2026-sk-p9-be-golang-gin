package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"app/internal/features/post"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errBoom = errors.New("boom")

func server(status int, body string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

// ---------- Search client ----------

func TestSearchClient(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"data":{"results":[{"title":"A","url":"https://a","snippet":"sa"},{"name":"B","link":"https://b","description":"sb"}]}}`))
	}))
	defer srv.Close()

	res, err := NewSearchClient(srv.URL).Search(context.Background(), "golang", 5)
	require.NoError(t, err)
	assert.Equal(t, "golang", got["query"])
	require.Len(t, res, 2)
	assert.Equal(t, SearchResult{"B", "https://b", "sb"}, res[1])
}

func TestSearchClientErrors(t *testing.T) {
	_, err := NewSearchClient("://bad").Search(context.Background(), "q", 1)
	assert.Error(t, err)

	_, err = NewSearchClient("http://127.0.0.1:1").Search(context.Background(), "q", 1)
	assert.Error(t, err)

	s := server(500, "oops")
	defer s.Close()
	_, err = NewSearchClient(s.URL).Search(context.Background(), "q", 1)
	assert.ErrorContains(t, err, "500")

	s2 := server(200, "not json")
	defer s2.Close()
	_, err = NewSearchClient(s2.URL).Search(context.Background(), "q", 1)
	assert.Error(t, err)
}

func TestParseSearchResults(t *testing.T) {
	var raw any
	_ = json.Unmarshal([]byte(`{"web":{"results":[
		"skip-me",
		{"foo":"no title/url"},
		{"title":"  T1 ","href":"u1","text":"x"},
		{"title":"T2","url":"u2","content":"y"},
		{"title":"T3","url":"u3"}
	]}}`), &raw)
	res := ParseSearchResults(raw, 2)
	require.Len(t, res, 2)
	assert.Equal(t, "T1", res[0].Title)
	assert.Equal(t, "u1", res[0].URL)

	assert.Len(t, ParseSearchResults(raw, 0), 3)
	_ = json.Unmarshal([]byte(`[{"title":"x","url":"y"}]`), &raw)
	assert.Len(t, ParseSearchResults(raw, 0), 1)
	_ = json.Unmarshal([]byte(`{"results":"nope","items":[{"title":"z"}]}`), &raw)
	assert.Len(t, ParseSearchResults(raw, 0), 1)
	assert.Empty(t, ParseSearchResults("text", 0))
}

// ---------- OpenAI client ----------

func TestOpenAIClient(t *testing.T) {
	var auth, path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth, path = r.Header.Get("Authorization"), r.URL.Path
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"halo"}}]}`))
	}))
	defer srv.Close()

	out, err := NewOpenAIClient(srv.URL+"/", "k", "m").Complete(context.Background(), []Message{{"user", "hi"}})
	require.NoError(t, err)
	assert.Equal(t, "halo", out)
	assert.Equal(t, "Bearer k", auth)
	assert.Equal(t, "/chat/completions", path)
}

func TestOpenAIClientErrors(t *testing.T) {
	ctx := context.Background()
	_, err := NewOpenAIClient("://bad", "k", "m").Complete(ctx, nil)
	assert.Error(t, err)
	_, err = NewOpenAIClient("http://127.0.0.1:1", "k", "m").Complete(ctx, nil)
	assert.Error(t, err)

	cases := []struct {
		status int
		body   string
		want   string
	}{
		{401, `{"error":{"message":"invalid key"}}`, "invalid key"},
		{500, `bad`, "Internal Server Error"},
		{200, `bad`, "tidak valid"},
		{200, `{"choices":[]}`, "tidak berisi"},
	}
	for _, c := range cases {
		s := server(c.status, c.body)
		_, err := NewOpenAIClient(s.URL, "k", "m").Complete(ctx, nil)
		assert.ErrorContains(t, err, c.want)
		s.Close()
	}
}

// ---------- Service ----------

type fakeSearch struct {
	res []SearchResult
	err error
}

func (f *fakeSearch) Search(context.Context, string, int) ([]SearchResult, error) {
	return f.res, f.err
}

type fakeLLM struct {
	out  string
	err  error
	seen []Message
}

func (f *fakeLLM) Complete(_ context.Context, m []Message) (string, error) {
	f.seen = m
	return f.out, f.err
}

const goodJSON = "```json\n{\"title\":\"Judul AI\",\"category\":\"Bisnis\",\"excerpt\":\" Ringkas \",\"content\":\"## Isi\\n\\nTeks [1].\"}\n```"

func TestBuildMessages(t *testing.T) {
	m := BuildMessages(GenerateRequest{Topic: "AI", Tone: "santai", Keywords: "llm"}, "Teknologi",
		[]SearchResult{{"S", "https://s", "cuplikan"}})
	require.Len(t, m, 2)
	assert.Contains(t, m[1].Content, "Gaya bahasa: santai")
	assert.Contains(t, m[1].Content, "Kata kunci: llm")
	assert.Contains(t, m[1].Content, "[1] S")

	m = BuildMessages(GenerateRequest{Topic: "AI"}, "Umum", nil)
	assert.NotContains(t, m[1].Content, "Gaya bahasa")
}

func TestParseArticle(t *testing.T) {
	a, err := ParseArticle(goodJSON)
	require.NoError(t, err)
	assert.Equal(t, "Judul AI", a.Title)

	for _, raw := range []string{"no json", "} {", `{"title": }`, `{"title":"x","content":" "}`} {
		_, err := ParseArticle(raw)
		assert.ErrorIs(t, err, ErrInvalidAIResponse, raw)
	}
}

func TestAppendSources(t *testing.T) {
	src := []SearchResult{{"", "https://a", ""}, {"B", "https://b", ""}}
	out := AppendSources("isi\n", src)
	assert.Contains(t, out, "## Sumber Rujukan")
	assert.Contains(t, out, "1. [https://a](https://a)")
	assert.Contains(t, out, "2. [B](https://b)")
	assert.Equal(t, "isi", AppendSources("isi", nil))
	assert.Equal(t, "x\n## Sumber Rujukan", AppendSources("x\n## Sumber Rujukan", src))
}

func TestServiceGenerate(t *testing.T) {
	ctx := context.Background()
	search := &fakeSearch{res: []SearchResult{{"S", "https://s", "c"}}}
	llm := &fakeLLM{out: goodJSON}
	res, err := NewService(search, llm).Generate(ctx, GenerateRequest{Topic: "Bisnis UMKM", IncludeWebSearch: true})
	require.NoError(t, err)
	assert.Equal(t, "Judul AI", res.Title)
	assert.True(t, res.AIUsed)
	assert.Empty(t, res.AIWarning)
	assert.Equal(t, "Bisnis", res.Category) // kategori dari AI dipakai jika user tidak memilih
	assert.Equal(t, "Ringkas", res.Excerpt)
	assert.True(t, res.WebSearchUsed)
	assert.Contains(t, res.Content, "## Sumber Rujukan")
	assert.Contains(t, llm.seen[1].Content, "https://s")

	// kategori pilihan user dipertahankan, tanpa web search
	res, err = NewService(search, llm).Generate(ctx, GenerateRequest{Topic: "x y z", Category: "Opini", MaxSources: 2})
	require.NoError(t, err)
	assert.Equal(t, "Opini", res.Category)
	assert.False(t, res.WebSearchUsed)
	assert.Empty(t, res.Sources)

	// web search gagal -> tetap jalan dengan peringatan, judul fallback ke topik
	llm.out = `{"title":"","content":"isi"}`
	res, err = NewService(&fakeSearch{err: errBoom}, llm).Generate(ctx, GenerateRequest{Topic: "Topik", IncludeWebSearch: true})
	require.NoError(t, err)
	assert.Equal(t, "Topik", res.Title)
	assert.Equal(t, "Umum", res.Category)
	assert.Contains(t, res.SearchWarning, "boom")

	_, err = NewService(search, llm).Generate(ctx, GenerateRequest{Topic: "x", Category: "Olahraga"})
	assert.ErrorIs(t, err, post.ErrInvalidCategory)

	// AI gagal / respons tidak valid -> tetap 200 dengan draf template
	res, err = NewService(search, &fakeLLM{err: errBoom}).Generate(ctx, GenerateRequest{Topic: "Energi Surya", IncludeWebSearch: true})
	require.NoError(t, err)
	assert.False(t, res.AIUsed)
	assert.Contains(t, res.AIWarning, "boom")
	assert.Equal(t, "Energi Surya", res.Title)
	assert.NotEmpty(t, res.Excerpt)
	assert.Contains(t, res.Content, "## Pendahuluan")
	assert.Contains(t, res.Content, "> c [1]")
	assert.Contains(t, res.Content, "## Sumber Rujukan")

	res, err = NewService(search, &fakeLLM{out: "nope"}).Generate(ctx, GenerateRequest{Topic: "x"})
	require.NoError(t, err)
	assert.False(t, res.AIUsed)
	assert.Contains(t, res.AIWarning, "respons AI tidak dapat diproses")
}

func TestFallbackArticle(t *testing.T) {
	a := FallbackArticle("Topik", "Bisnis", nil)
	assert.Equal(t, "Topik", a.Title)
	assert.Equal(t, "Bisnis", a.Category)
	assert.Equal(t, "Draf awal artikel bisnis tentang Topik.", a.Excerpt)
	assert.NotContains(t, a.Content, "> ")
}

// ---------- Handler ----------

type fakeSvc struct{ err error }

func (f *fakeSvc) Generate(context.Context, GenerateRequest) (*GenerateResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &GenerateResponse{Title: "T"}, nil
}

func TestHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	call := func(svc Service, body string) int {
		r := gin.New()
		r.POST("/gen", NewHandler(svc).Generate)
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/gen", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		return w.Code
	}
	ok := `{"topic":"Topik artikel","include_web_search":true}`
	assert.Equal(t, http.StatusOK, call(&fakeSvc{}, ok))
	assert.Equal(t, http.StatusBadRequest, call(&fakeSvc{}, `{}`))
	assert.Equal(t, http.StatusBadRequest, call(&fakeSvc{err: post.ErrInvalidCategory}, ok))
	assert.Equal(t, http.StatusInternalServerError, call(&fakeSvc{err: errBoom}, ok))
}
