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

type MockCommandRunner struct {
	mock.Mock
}

func (m *MockCommandRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	called := m.Called(ctx, name, args)
	if called.Error(1) != nil {
		return nil, called.Error(1)
	}
	return called.Get(0).([]byte), nil
}

func (m *MockCommandRunner) RunWithOutput(ctx context.Context, name string, args ...string) (string, string, error) {
	called := m.Called(ctx, name, args)
	return called.String(0), called.String(1), called.Error(2)
}

var _ ports.CommandRunner = (*MockCommandRunner)(nil)
