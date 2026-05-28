package crashsight

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	defaultTimeout    = 30 * time.Second
	apiPathPrefix     = "/uniform/openapi"
	headerContentType = "Content-Type"
	headerAcceptEnc   = "Accept-Encoding"
	mimeJSON          = "application/json"
)

// Client 是 CrashSight OpenAPI 的并发安全客户端。
//
// 所有字段在 NewClient 返回后均为只读，无需加锁。
// 底层 *http.Client 自带连接池，可安全地跨 goroutine 共享。
type Client struct {
	userID  string
	apiKey  string
	baseURL string       // 不带尾部斜线
	http    *http.Client // 并发安全，复用连接池
}

// ClientOption 函数式选项，用于配置 Client。
type ClientOption func(*Client)

// WithRegion 设置部署区域（默认 RegionCN）。
func WithRegion(r Region) ClientOption {
	return func(c *Client) {
		c.baseURL = r.BaseURL()
	}
}

// WithBaseURL 设置自定义基础 URL（覆盖 WithRegion）。
func WithBaseURL(rawURL string) ClientOption {
	return func(c *Client) {
		c.baseURL = strings.TrimRight(rawURL, "/")
	}
}

// WithTimeout 设置每次请求的超时时间（默认 30s）。
func WithTimeout(d time.Duration) ClientOption {
	return func(c *Client) {
		c.http.Timeout = d
	}
}

// WithHTTPClient 替换底层 *http.Client（用于测试或自定义 Transport）。
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) {
		c.http = hc
	}
}

// NewClient 创建一个新的 CrashSight 客户端。
//
//	client := crashsight.NewClient("your_user_id", "your_api_key",
//	    crashsight.WithRegion(crashsight.RegionCN),
//	)
func NewClient(userID, apiKey string, opts ...ClientOption) *Client {
	c := &Client{
		userID:  userID,
		apiKey:  apiKey,
		baseURL: RegionCN.BaseURL(),
		http: &http.Client{
			Timeout: defaultTimeout,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// CrashIDToHash 将 crashId 转换为 crashHash（每 2 位插入 ":"）。
//
// 例如 "a683c7" → "a6:83:c7"
func CrashIDToHash(id string) string {
	if len(id) == 0 {
		return id
	}
	var sb strings.Builder
	sb.Grow(len(id) + len(id)/2)
	for i := 0; i < len(id); i += 2 {
		if i > 0 {
			sb.WriteByte(':')
		}
		end := i + 2
		if end > len(id) {
			end = len(id)
		}
		sb.WriteString(id[i:end])
	}
	return sb.String()
}

// newFSN 生成随机请求追踪 ID（UUID v4 格式）。
func newFSN() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant bits
	var dst [36]byte
	hex.Encode(dst[0:8], b[0:4])
	dst[8] = '-'
	hex.Encode(dst[9:13], b[4:6])
	dst[13] = '-'
	hex.Encode(dst[14:18], b[6:8])
	dst[18] = '-'
	hex.Encode(dst[19:23], b[8:10])
	dst[23] = '-'
	hex.Encode(dst[24:36], b[10:16])
	return string(dst[:])
}

// buildURL 构造完整请求 URL，将鉴权参数和额外参数合并到 query string。
func (c *Client) buildURL(path string, extra url.Values) string {
	auth := buildAuthParams(c.userID, c.apiKey)
	// 将 extra 合并进 auth（auth 参数优先）
	for k, vs := range extra {
		if _, exists := auth[k]; !exists {
			auth[k] = vs
		}
	}
	return c.baseURL + path + "?" + auth.Encode()
}

// post 发送 JSON POST 请求，将响应解包后反序列化到 out。
func (c *Client) post(ctx context.Context, path string, body any, out any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("crashsight: marshal request: %w", err)
	}

	rawURL := c.buildURL(path, nil)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("crashsight: build request: %w", err)
	}
	req.Header.Set(headerContentType, mimeJSON)
	req.Header.Set(headerAcceptEnc, "*")

	resp, err := c.http.Do(req)
	if err != nil {
		return &TransportError{Cause: err}
	}
	defer resp.Body.Close()

	return c.handleResponse(req, resp, out)
}

// get 发送 GET 请求，将鉴权参数与 query 合并后发送，响应解包后反序列化到 out。
func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	rawURL := c.buildURL(path, query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fmt.Errorf("crashsight: build request: %w", err)
	}
	req.Header.Set(headerContentType, mimeJSON)
	req.Header.Set(headerAcceptEnc, "*")

	resp, err := c.http.Do(req)
	if err != nil {
		return &TransportError{Cause: err}
	}
	defer resp.Body.Close()

	return c.handleResponse(req, resp, out)
}

// handleResponse 统一处理 HTTP 响应：
//  1. HTTP 401 → AuthError
//  2. HTTP 429 → RateLimitError
//  3. Spring error body → APIError
//  4. ret.code != 200 → APIError，并解包 ret.data
//  5. 顶层 code != 0/200 → APIError，并解包 data
//  6. 否则直接反序列化到 out
//
// 调试：设置环境变量 CRASHSIGHT_DEBUG_JSON=1 打印所有接口原始响应；
// 或设置为接口路径关键词（如 getTopIssueEx）只打印匹配的接口。
func (c *Client) handleResponse(req *http.Request, resp *http.Response, out any) error {
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		body, _ := io.ReadAll(resp.Body)
		return &AuthError{Message: string(body)}
	case http.StatusTooManyRequests:
		return &RateLimitError{}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &TransportError{Cause: err}
	}

	// CRASHSIGHT_DEBUG_JSON=1 打印所有接口；设为路径关键词只打印匹配的接口。
	if dbg := os.Getenv("CRASHSIGHT_DEBUG_JSON"); dbg != "" && (dbg == "1" || strings.Contains(req.URL.Path, dbg)) {
		var pretty bytes.Buffer
		if json.Indent(&pretty, body, "", "  ") == nil {
			fmt.Fprintf(os.Stderr, "── CRASHSIGHT DEBUG [%s %s] ──\n%s\n──────────────────────────────\n", req.Method, req.URL.Path, pretty.String())
		} else {
			fmt.Fprintf(os.Stderr, "── CRASHSIGHT DEBUG [%s %s] ──\n%s\n──────────────────────────────\n", req.Method, req.URL.Path, body)
		}
	}

	// 尝试解析为通用 map 以便检查 envelope 格式
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return &ParseError{Body: string(body), Cause: err}
	}

	// Pattern: Spring error envelope {"error":..., "path":..., "timestamp":...}
	if _, hasError := raw["error"]; hasError {
		if _, hasPath := raw["path"]; hasPath {
			var msg string
			if v, ok := raw["message"]; ok {
				_ = json.Unmarshal(v, &msg)
			}
			if msg == "" {
				_ = json.Unmarshal(raw["error"], &msg)
			}
			var statusCode int
			if v, ok := raw["status"]; ok {
				_ = json.Unmarshal(v, &statusCode)
			}
			return &APIError{
				Message:    msg,
				StatusCode: statusCode,
				Raw:        json.RawMessage(body),
			}
		}
	}

	// Pattern 1: {"status":..., "ret": {"code":..., "data":..., "message":...}}
	if retRaw, ok := raw["ret"]; ok {
		var ret map[string]json.RawMessage
		if err := json.Unmarshal(retRaw, &ret); err == nil {
			// ret 是对象（非数组）
			if codeRaw, hascode := ret["code"]; hascode {
				var code int
				_ = json.Unmarshal(codeRaw, &code)
				if code != 0 && code != 200 {
					var msg string
					if v, ok := ret["message"]; ok {
						_ = json.Unmarshal(v, &msg)
					}
					var errorCode, traceID string
					if v, ok := ret["errorCode"]; ok {
						_ = json.Unmarshal(v, &errorCode)
					}
					if v, ok := ret["traceId"]; ok {
						_ = json.Unmarshal(v, &traceID)
					}
					return &APIError{
						Message:    msg,
						StatusCode: code,
						ErrorCode:  errorCode,
						TraceID:    traceID,
						Raw:        json.RawMessage(body),
					}
				}
			}
			// 解包 ret.data（若存在）或直接用 ret
			if dataRaw, ok := ret["data"]; ok {
				if out != nil {
					if err := json.Unmarshal(dataRaw, out); err != nil {
						return &ParseError{Body: string(dataRaw), Cause: err}
					}
				}
				return nil
			}
			// ret 本身无 data，直接序列化 ret
			if out != nil {
				if err := json.Unmarshal(retRaw, out); err != nil {
					return &ParseError{Body: string(retRaw), Cause: err}
				}
			}
			return nil
		}

		// ret 是数组（noteList 等）
		if retRaw[0] == '[' {
			if out != nil {
				if err := json.Unmarshal(retRaw, out); err != nil {
					return &ParseError{Body: string(retRaw), Cause: err}
				}
			}
			return nil
		}

		// Pattern 1b: ret 是数字/字符串（如 getTopIssueEx 返回 {"status":200,"ret":200,"data":{...}}）
		// 此时 data 在顶层，直接取顶层 data 字段。
		if dataRaw, ok := raw["data"]; ok {
			if out != nil {
				if err := json.Unmarshal(dataRaw, out); err != nil {
					return &ParseError{Body: string(dataRaw), Cause: err}
				}
			}
			return nil
		}
	}

	// Pattern 2: 平坦 envelope {"code":..., "data":..., "errmsg":...}
	if codeRaw, ok := raw["code"]; ok {
		var code int
		_ = json.Unmarshal(codeRaw, &code)
		if code != 0 && code != 200 && code != 100000 {
			var msg string
			for _, key := range []string{"errmsg", "message", "msg"} {
				if v, ok := raw[key]; ok {
					_ = json.Unmarshal(v, &msg)
					if msg != "" {
						break
					}
				}
			}
			return &APIError{
				Message:    msg,
				StatusCode: code,
				Raw:        json.RawMessage(body),
			}
		}
		if dataRaw, ok := raw["data"]; ok {
			if out != nil {
				if err := json.Unmarshal(dataRaw, out); err != nil {
					return &ParseError{Body: string(dataRaw), Cause: err}
				}
			}
			return nil
		}
	}

	// Pattern 3: 直接是目标结构（crashDoc, crashList 等平坦响应）
	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return &ParseError{Body: string(body), Cause: err}
		}
	}
	return nil
}

// strDefault 若 s 为空则返回 def。
func strDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// intDefault 若 v 为 0 则返回 def。
func intDefault(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}

// int64Default 若 v 为 0 则返回 def。
func int64Default(v, def int64) int64 {
	if v == 0 {
		return def
	}
	return v
}

// crashTypeDefault 若 v 为空则返回 def。
func crashTypeDefault(v CrashType, def CrashType) CrashType {
	if v == "" {
		return def
	}
	return v
}

// stringsDefault 若 v 为空则返回 def。
func stringsDefault(v []string, def []string) []string {
	if len(v) == 0 {
		return def
	}
	return v
}
