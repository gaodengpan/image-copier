package core

import (
	"bytes"
	"context"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// TestLogger captures log output for testing purposes
type TestLogger struct {
	*log.Logger
	Buffer *bytes.Buffer
}

// NewTestLogger creates a logger that captures output in a buffer for inspection
func NewTestLogger() *TestLogger {
	buffer := &bytes.Buffer{}
	logger := log.New(buffer, "", log.Lshortfile)
	return &TestLogger{
		Logger: logger,
		Buffer: buffer,
	}
}

// GetLogs returns the captured log output
func (tl *TestLogger) GetLogs() string {
	return tl.Buffer.String()
}

// ContainsSensitiveInfo checks if logs contain sensitive information
func (tl *TestLogger) ContainsSensitiveInfo(secrets ...string) bool {
	logs := tl.GetLogs()
	for _, secret := range secrets {
		if strings.Contains(logs, secret) {
			return true
		}
	}
	return false
}

// TestLogHook is a logrus hook for capturing logs during tests
type TestLogHook struct {
	Entries []*logrus.Entry
	mutex   sync.RWMutex
}

// Fire adds an entry to the hook
func (hook *TestLogHook) Fire(entry *logrus.Entry) error {
	hook.mutex.Lock()
	defer hook.mutex.Unlock()

	newEntry := *entry
	hook.Entries = append(hook.Entries, &newEntry)
	return nil
}

// Levels specifies which log levels to capture
func (hook *TestLogHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// GetEntries returns all captured log entries
func (hook *TestLogHook) GetEntries() []*logrus.Entry {
	hook.mutex.RLock()
	defer hook.mutex.RUnlock()

	// Return a copy to prevent race conditions
	entries := make([]*logrus.Entry, len(hook.Entries))
	copy(entries, hook.Entries)
	return entries
}

// HasSensitiveInfo checks if any log entry contains sensitive information
func (hook *TestLogHook) HasSensitiveInfo(secrets ...string) bool {
	entries := hook.GetEntries()
	for _, entry := range entries {
		for _, secret := range secrets {
			if strings.Contains(entry.Message, secret) {
				return true
			}
			// Check fields for sensitive info too
			for _, value := range entry.Data {
				if strVal, ok := value.(string); ok && strings.Contains(strVal, secret) {
					return true
				}
			}
		}
	}
	return false
}

// newTestPullerWithLogHook creates a puller with a log hook for testing
func newTestPullerWithLogHook(cfg *Config) (*Puller, *TestLogHook) {
	if cfg == nil {
		cfg = &Config{
			GithubOwner:       "owner",
			GithubRepo:        "repo",
			GithubToken:       "token",
			GithubWorkflowID:  "workflow.yaml",
			RegistryHost:      "registry.example.com",
			RegistryUsername:  "user",
			RegistryPassword:  "pass",
			RegistryNamespace: "ns",
			RegistryArch:      "amd64",
			RegistryOs:        "linux",
		}
	}

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel) // Capture all log levels

	hook := &TestLogHook{}
	logger.AddHook(hook)

	return NewPuller(cfg, logger), hook
}

// CaptureLogsWithBuffer runs a function and captures logs written with stdlib log
func CaptureLogsWithBuffer(fn func()) string {
	buffer := &bytes.Buffer{}
	log.SetOutput(io.MultiWriter(buffer, log.Writer())) // Also write to original output
	defer log.SetOutput(log.Writer())                   // Restore original output

	fn()

	return buffer.String()
}

// SanitizeErrorMessage checks if an error message contains sensitive information
func SanitizeErrorMessage(err error, secrets ...string) bool {
	if err == nil {
		return false
	}

	errMsg := err.Error()
	for _, secret := range secrets {
		if strings.Contains(errMsg, secret) {
			return true
		}
	}
	return false
}

// CreateExpiredContext creates a context that's already expired
func CreateExpiredContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	return ctx, cancel
}

// WaitForRaceCondition allows goroutines time to potentially exhibit race conditions
func WaitForRaceCondition() {
	time.Sleep(100 * time.Millisecond)
}
