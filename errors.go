package crashsight

import (
	"encoding/json"
	"fmt"
	"strings"
)

// APIError 表示服务端返回的业务错误（HTTP 200 但 code != 200/0）。
//
// 使用 errors.As 检查:
//
//	var apiErr *APIError
//	if errors.As(err, &apiErr) {
//	    fmt.Println(apiErr.StatusCode, apiErr.TraceID)
//	}
type APIError struct {
	// Message 服务端错误描述。
	Message string
	// StatusCode 服务端业务状态码。
	StatusCode int
	// ErrorCode 机器可读的错误码（部分接口返回）。
	ErrorCode string
	// TraceID 服务端 traceId，用于排查（部分接口返回）。
	TraceID string
	// Raw 完整的原始响应体，用于调试。
	Raw json.RawMessage
}

func (e *APIError) Error() string {
	var sb strings.Builder
	sb.WriteString("crashsight: api error")
	if e.Message != "" {
		sb.WriteString(": ")
		sb.WriteString(e.Message)
	}
	if e.StatusCode != 0 {
		fmt.Fprintf(&sb, " (status=%d)", e.StatusCode)
	}
	if e.ErrorCode != "" {
		fmt.Fprintf(&sb, " (error_code=%s)", e.ErrorCode)
	}
	if e.TraceID != "" {
		fmt.Fprintf(&sb, " (trace_id=%s)", e.TraceID)
	}
	return sb.String()
}

// AuthError 鉴权失败（HTTP 401）。
type AuthError struct {
	// Message 服务端返回的原始错误文本。
	Message string
}

func (e *AuthError) Error() string {
	return "crashsight: authentication failed: " + e.Message
}

// RateLimitError 触发频率限制（HTTP 429）。
// CrashSight 限制单用户所有接口合计 25 次/分钟。
type RateLimitError struct{}

func (e *RateLimitError) Error() string {
	return "crashsight: rate limit exceeded (25 req/min per user)"
}

// TransportError 表示网络传输层错误（DNS、连接超时、读超时等）。
type TransportError struct {
	// Cause 底层 error。
	Cause error
}

func (e *TransportError) Error() string {
	return "crashsight: transport error: " + e.Cause.Error()
}

func (e *TransportError) Unwrap() error { return e.Cause }

// ParseError 表示响应体 JSON 解析失败。
type ParseError struct {
	// Body 原始响应体（截断至 512 字节以避免日志过长）。
	Body string
	// Cause 底层解析错误。
	Cause error
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("crashsight: failed to parse response: %v (body: %.512s)", e.Cause, e.Body)
}

func (e *ParseError) Unwrap() error { return e.Cause }
