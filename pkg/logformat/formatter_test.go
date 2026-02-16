package logformat

import (
	"bytes"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestCLIFormatter_Format(t *testing.T) {
	formatter := &CLIFormatter{}

	tests := []struct {
		name     string
		level    logrus.Level
		message  string
		wantChar string
	}{
		{
			name:     "debug level",
			level:    logrus.DebugLevel,
			message:  "debug message",
			wantChar: "·",
		},
		{
			name:     "info level",
			level:    logrus.InfoLevel,
			message:  "info message",
			wantChar: "→",
		},
		{
			name:     "warn level",
			level:    logrus.WarnLevel,
			message:  "warn message",
			wantChar: "⚠",
		},
		{
			name:     "error level",
			level:    logrus.ErrorLevel,
			message:  "error message",
			wantChar: "✗",
		},
		{
			name:     "fatal level",
			level:    logrus.FatalLevel,
			message:  "fatal message",
			wantChar: "✗",
		},
		{
			name:     "panic level",
			level:    logrus.PanicLevel,
			message:  "panic message",
			wantChar: "✗",
		},
		{
			name:     "trace level",
			level:    logrus.TraceLevel,
			message:  "trace message",
			wantChar: "·",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			entry := &logrus.Entry{
				Level:   tt.level,
				Message: tt.message,
				Logger: &logrus.Logger{
					Out: buf,
				},
			}

			result, err := formatter.Format(entry)
			assert.NoError(t, err)
			assert.Contains(t, string(result), tt.wantChar)
			assert.Contains(t, string(result), tt.message)
		})
	}
}
