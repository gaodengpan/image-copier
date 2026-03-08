package sanitizer

import (
	"regexp"
)

// sensitivePatterns 定义需要过滤的敏感信息模式
var sensitivePatterns = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	// 环境变量名模式
	{regexp.MustCompile(`SKOPEO_SRC_USER=[^\s]*`), "SKOPEO_SRC_USER=***"},
	{regexp.MustCompile(`SKOPEO_SRC_PASS=[^\s]*`), "SKOPEO_SRC_PASS=***"},
	{regexp.MustCompile(`SKOPEO_DEST_USER=[^\s]*`), "SKOPEO_DEST_USER=***"},
	{regexp.MustCompile(`SKOPEO_DEST_PASS=[^\s]*`), "SKOPEO_DEST_PASS=***"},
	{regexp.MustCompile(`SKOPEO_USER=[^\s]*`), "SKOPEO_USER=***"},
	{regexp.MustCompile(`SKOPEO_PASSWORD=[^\s]*`), "SKOPEO_PASSWORD=***"},
	// URL 中的凭据 (user:pass@host) - 更宽松的模式以处理特殊字符密码
	// 匹配格式：非空白字符:非空白字符@，排除常见协议前缀
	{regexp.MustCompile(`([^\s:/@]+):([^\s/@]+)@`), "***:***@"},
	// Bearer tokens
	{regexp.MustCompile(`Bearer [a-zA-Z0-9_.-]+`), "Bearer ***"},
	// Basic auth
	{regexp.MustCompile(`Basic [a-zA-Z0-9+/=]+`), "Basic ***"},
	// Direct credential patterns (user:pass as standalone)
	{regexp.MustCompile(`--creds\s+\S+:\S+`), "--creds ***:***"},
	{regexp.MustCompile(`--src-creds\s+\S+:\S+`), "--src-creds ***:***"},
	{regexp.MustCompile(`--dest-creds\s+\S+:\S+`), "--dest-creds ***:***"},
}

// ErrorOutput 过滤错误输出中的敏感信息
func ErrorOutput(output string) string {
	result := output
	for _, sp := range sensitivePatterns {
		result = sp.pattern.ReplaceAllString(result, sp.replacement)
	}
	return result
}

// TruncateOutput 截断过长的输出
func TruncateOutput(output string, maxLen int) string {
	if len(output) <= maxLen {
		return output
	}
	return output[:maxLen] + "... (truncated)"
}

// SanitizeError 综合处理错误输出：过滤敏感信息并截断
func SanitizeError(output string, maxLen int) string {
	sanitized := ErrorOutput(output)
	if maxLen > 0 {
		return TruncateOutput(sanitized, maxLen)
	}
	return sanitized
}