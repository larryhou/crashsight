package crashsight

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/url"
	"strconv"
	"time"
)

// computeSignature 计算 CrashSight OpenAPI 用户级鉴权签名。
//
// 算法: Base64( HexString( HMAC-SHA256(key=apiKey, msg="{userID}_{timestamp}") ) )
//
// 该函数无任何共享状态，可安全地在多个 goroutine 中并发调用。
func computeSignature(userID, apiKey string, ts int64) string {
	msg := userID + "_" + strconv.FormatInt(ts, 10)
	mac := hmac.New(sha256.New, []byte(apiKey))
	mac.Write([]byte(msg))
	hexDigest := hex.EncodeToString(mac.Sum(nil))
	return base64.StdEncoding.EncodeToString([]byte(hexDigest))
}

// buildAuthParams 生成需要附加到每个请求 query string 的三个鉴权参数。
// 每次调用都会取当前 Unix 时间戳，保证签名时效性。
//
// 返回的 url.Values 包含:
//   - userSecret: HMAC-SHA256 签名（Base64 编码）
//   - localUserId: 用户 ID
//   - t: Unix 时间戳（秒）
func buildAuthParams(userID, apiKey string) url.Values {
	ts := time.Now().Unix()
	sig := computeSignature(userID, apiKey, ts)
	v := make(url.Values, 3)
	v.Set("userSecret", sig)
	v.Set("localUserId", userID)
	v.Set("t", strconv.FormatInt(ts, 10))
	return v
}
