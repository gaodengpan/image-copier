package logformat

import (
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestCLIFormatter_Levels(t *testing.T) {
	f := &CLIFormatter{}

	tests := []struct {
		level      logrus.Level
		wantSymbol string
		wantColor  string
	}{
		{logrus.DebugLevel, "·", colorGray},
		{logrus.TraceLevel, "·", colorGray},
		{logrus.InfoLevel, "→", colorBlue},
		{logrus.WarnLevel, "⚠", colorYellow},
		{logrus.ErrorLevel, "✗", colorRed},
		{logrus.FatalLevel, "✗", colorRed},
		{logrus.PanicLevel, "✗", colorRed},
	}

	for _, tt := range tests {
		t.Run(tt.level.String(), func(t *testing.T) {
			entry := &logrus.Entry{
				Level:   tt.level,
				Message: "test message",
			}

			out, err := f.Format(entry)
			if err != nil {
				t.Fatalf("Format returned error: %v", err)
			}

			line := string(out)

			if !strings.Contains(line, tt.wantSymbol) {
				t.Errorf("expected symbol %q in output %q", tt.wantSymbol, line)
			}

			if !strings.Contains(line, tt.wantColor) {
				t.Errorf("expected color code %q in output %q", tt.wantColor, line)
			}

			if !strings.Contains(line, "test message") {
				t.Errorf("expected message in output %q", line)
			}

			if !strings.HasSuffix(line, "\n") {
				t.Errorf("expected trailing newline in output %q", line)
			}
		})
	}
}

func TestCLIFormatter_NoTimestamp(t *testing.T) {
	f := &CLIFormatter{}

	entry := &logrus.Entry{
		Level:   logrus.InfoLevel,
		Message: "hello world",
	}

	out, err := f.Format(entry)
	if err != nil {
		t.Fatalf("Format returned error: %v", err)
	}

	line := string(out)

	// Should not contain log level text
	for _, levelText := range []string{"INFO", "info", "WARN", "warn", "ERROR", "error"} {
		if strings.Contains(line, levelText) {
			t.Errorf("output should not contain level text %q, got %q", levelText, line)
		}
	}
}

func TestCLIFormatter_ColorReset(t *testing.T) {
	f := &CLIFormatter{}

	entry := &logrus.Entry{
		Level:   logrus.WarnLevel,
		Message: "warning",
	}

	out, err := f.Format(entry)
	if err != nil {
		t.Fatalf("Format returned error: %v", err)
	}

	line := string(out)

	// Should contain color reset after symbol
	if !strings.Contains(line, colorReset) {
		t.Errorf("expected color reset in output %q", line)
	}
}
