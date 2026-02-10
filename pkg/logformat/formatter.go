package logformat

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorGray   = "\033[90m"
)

// CLIFormatter formats log entries as clean, user-friendly CLI output
// with colored symbols indicating the log level.
//
// Output examples:
//
//	→ Processing image: nginx:latest          (Info, blue)
//	✓ Workflow completed successfully          (Info "completed/success", green — future use)
//	⚠ Failed to check local image, continuing (Warn, yellow)
//	✗ workflow failed                          (Error, red)
//	· detailed debug info                      (Debug, gray)
type CLIFormatter struct{}

// Format renders a log entry as: "symbol message\n"
func (f *CLIFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	var symbol, color string

	switch entry.Level {
	case logrus.TraceLevel, logrus.DebugLevel:
		symbol = "·"
		color = colorGray
	case logrus.InfoLevel:
		symbol = "→"
		color = colorBlue
	case logrus.WarnLevel:
		symbol = "⚠"
		color = colorYellow
	case logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel:
		symbol = "✗"
		color = colorRed
	default:
		symbol = "→"
		color = colorBlue
	}

	line := fmt.Sprintf("%s%s%s %s\n", color, symbol, colorReset, entry.Message)
	return []byte(line), nil
}
