// Package ai menyediakan AI Blog Generator yang terintegrasi dengan
// Search Engine (penelusuran fakta terkini) dan OpenAI Chat Completions.
package ai

// GenerateRequest adalah payload pembuatan draf artikel via AI.
type GenerateRequest struct {
	Topic            string `json:"topic" binding:"required,min=3,max=300" example:"Tren kecerdasan buatan di dunia pendidikan 2026"`
	Category         string `json:"category" binding:"omitempty,max=100" example:"Teknologi"`
	Tone             string `json:"tone" binding:"omitempty,max=50" example:"informatif"`
	Keywords         string `json:"keywords" binding:"omitempty,max=300" example:"AI, pembelajaran adaptif"`
	IncludeWebSearch bool   `json:"include_web_search" example:"true"`
	MaxSources       int    `json:"max_sources" binding:"omitempty,min=1,max=10" example:"5"`
}

// SearchResult adalah satu hasil penelusuran mesin pencari.
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// GeneratedArticle adalah struktur JSON yang diharapkan dari model AI.
type GeneratedArticle struct {
	Title    string `json:"title"`
	Category string `json:"category"`
	Excerpt  string `json:"excerpt"`
	Content  string `json:"content"`
}

// GenerateResponse adalah draf artikel lengkap beserta sumber rujukan web.
type GenerateResponse struct {
	Title         string         `json:"title"`
	Category      string         `json:"category"`
	Excerpt       string         `json:"excerpt"`
	Content       string         `json:"content"`
	Sources       []SearchResult `json:"sources"`
	WebSearchUsed bool           `json:"web_search_used"`
	SearchWarning string         `json:"search_warning,omitempty"`
}
