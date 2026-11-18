package docs

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/swaggo/swag"
)

func TestSwaggerDocRegistered(t *testing.T) {
	doc, err := swag.ReadDoc()
	assert.NoError(t, err)

	var spec map[string]any
	assert.NoError(t, json.Unmarshal([]byte(doc), &spec))
	assert.Equal(t, "Delcom Blog API", spec["info"].(map[string]any)["title"])

	paths := spec["paths"].(map[string]any)
	for _, p := range []string{"/api/auth/login", "/api/posts", "/api/posts/ai-generate", "/api/admin/users", "/api/public/posts/{slug}"} {
		assert.Contains(t, paths, p)
	}
}
