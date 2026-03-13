package gateways

import (
	"context"
	"net/http"
)

// GitHubRequestBuilder 封装 GitHub API 请求构建逻辑
type GitHubRequestBuilder struct {
	token string
}

// NewGitHubRequestBuilder 创建请求构建器
func NewGitHubRequestBuilder(token string) *GitHubRequestBuilder {
	return &GitHubRequestBuilder{token: token}
}

// SetCommonHeaders 设置通用 GitHub API headers
func (b *GitHubRequestBuilder) SetCommonHeaders(req *http.Request) {
	req.Header.Set("Accept", GitHubMediaType)
	req.Header.Set("Authorization", "Bearer "+b.token)
	req.Header.Set("X-GitHub-Api-Version", GitHubAPIVersion)
}

// SetJSONHeaders 设置 JSON 内容类型（用于 POST 请求）
func (b *GitHubRequestBuilder) SetJSONHeaders(req *http.Request) {
	b.SetCommonHeaders(req)
	req.Header.Set("Content-Type", "application/json")
}

// BuildGetRequest 构建带认证的 GET 请求
func (b *GitHubRequestBuilder) BuildGetRequest(url string) (*http.Request, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	b.SetCommonHeaders(req)
	return req, nil
}

// BuildGetRequestWithContext 构建带 context 的 GET 请求
func (b *GitHubRequestBuilder) BuildGetRequestWithContext(ctx context.Context, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	b.SetCommonHeaders(req)
	return req, nil
}
