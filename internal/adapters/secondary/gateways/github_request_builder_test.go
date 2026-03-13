package gateways

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGitHubRequestBuilder_SetCommonHeaders(t *testing.T) {
	builder := NewGitHubRequestBuilder("test-token")
	req := httptest.NewRequest("GET", "/test", nil)

	builder.SetCommonHeaders(req)

	assert.Equal(t, GitHubMediaType, req.Header.Get("Accept"))
	assert.Equal(t, "Bearer test-token", req.Header.Get("Authorization"))
	assert.Equal(t, GitHubAPIVersion, req.Header.Get("X-GitHub-Api-Version"))
}

func TestGitHubRequestBuilder_SetJSONHeaders(t *testing.T) {
	builder := NewGitHubRequestBuilder("secret-token")
	req := httptest.NewRequest("POST", "/test", nil)

	builder.SetJSONHeaders(req)

	// 验证通用 headers
	assert.Equal(t, GitHubMediaType, req.Header.Get("Accept"))
	assert.Equal(t, "Bearer secret-token", req.Header.Get("Authorization"))
	assert.Equal(t, GitHubAPIVersion, req.Header.Get("X-GitHub-Api-Version"))
	// 验证 JSON header
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
}

func TestGitHubRequestBuilder_BuildGetRequest(t *testing.T) {
	builder := NewGitHubRequestBuilder("my-token")

	req, err := builder.BuildGetRequest("https://api.github.com/test")
	assert.NoError(t, err)
	assert.Equal(t, "GET", req.Method)
	assert.Equal(t, "https://api.github.com/test", req.URL.String())
	assert.Equal(t, "Bearer my-token", req.Header.Get("Authorization"))
}

func TestGitHubRequestBuilder_EmptyToken(t *testing.T) {
	builder := NewGitHubRequestBuilder("")
	req := httptest.NewRequest("GET", "/test", nil)

	builder.SetCommonHeaders(req)

	// 即使 token 为空，也应该设置 header
	assert.Equal(t, "Bearer ", req.Header.Get("Authorization"))
}
