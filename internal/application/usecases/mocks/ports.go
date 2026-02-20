package mocks

import (
	"context"
	"net/http"

	"github.com/gaodengpan/image-copier/internal/application/ports"
	"github.com/stretchr/testify/mock"
)

type MockSystemClient struct {
	mock.Mock
}

func (m *MockSystemClient) CommandExists(ctx context.Context, cmd string) (bool, error) {
	args := m.Called(ctx, cmd)
	return args.Bool(0), args.Error(1)
}

func (m *MockSystemClient) DockerRunning(ctx context.Context) (bool, error) {
	args := m.Called(ctx)
	return args.Bool(0), args.Error(1)
}

var _ ports.SystemClient = (*MockSystemClient)(nil)

type MockHTTPClient struct {
	mock.Mock
}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*http.Response), args.Error(1)
}

var _ ports.HTTPClient = (*MockHTTPClient)(nil)
