package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Searcher adalah kontrak client mesin pencari.
type Searcher interface {
	Search(ctx context.Context, query string, limit int) ([]SearchResult, error)
}

// SearchClient adalah HTTP client untuk SEARCH_ENGINE_ENDPOINT.
type SearchClient struct {
	Endpoint string
	HTTP     *http.Client
}

// NewSearchClient membuat client mesin pencari dengan timeout default.
func NewSearchClient(endpoint string) *SearchClient {
	return &SearchClient{Endpoint: endpoint, HTTP: &http.Client{Timeout: 30 * time.Second}}
}

// Search mengirim query ke mesin pencari dan mengembalikan snippet + URL.
func (c *SearchClient) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	payload, _ := json.Marshal(map[string]any{
		"query":       query,
		"q":           query,
		"limit":       limit,
		"max_results": limit,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("search engine mengembalikan status %d", resp.StatusCode)
	}
	var raw any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("respons search engine tidak valid: %w", err)
	}
	return ParseSearchResults(raw, limit), nil
}

var listKeys = []string{"results", "items", "organic_results", "organic", "web", "hits", "data"}

// findList mencari array hasil di dalam berbagai bentuk respons JSON
// (mis. {data:{results:[...]}}, {results:[...]}, {web:{results:[...]}} atau [...]).
func findList(v any) []any {
	switch t := v.(type) {
	case []any:
		return t
	case map[string]any:
		for _, k := range listKeys {
			if child, ok := t[k]; ok {
				if list := findList(child); list != nil {
					return list
				}
			}
		}
	}
	return nil
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s, ok := m[k].(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

// ParseSearchResults menormalkan respons mesin pencari ke []SearchResult.
func ParseSearchResults(raw any, limit int) []SearchResult {
	results := []SearchResult{}
	for _, item := range findList(raw) {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		r := SearchResult{
			Title:   firstString(m, "title", "name"),
			URL:     firstString(m, "url", "link", "href"),
			Snippet: firstString(m, "snippet", "content", "description", "body", "text"),
		}
		if r.URL == "" && r.Title == "" {
			continue
		}
		results = append(results, r)
		if limit > 0 && len(results) >= limit {
			break
		}
	}
	return results
}
