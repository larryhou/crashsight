package crashsight

import (
	"context"
	"errors"
)

// ─────────────────────────────────────────────────────────────────────────────
//  附件管理 API
// ─────────────────────────────────────────────────────────────────────────────

// FetchCrashAttachments 批量获取崩溃附件下载链接。
//
// 对应接口: POST /uniform/openapi/fetchCrashAttachments
//
// 支持的附件类型:
//   - SDK_LOG
//   - CustomizedAttachFile.zip
//   - CustomizedLogFile.log
//   - extraMessage.txt
//   - anrMessage.txt
//   - trace.zip
//
// 当指定的附件不存在时，服务端返回 "attachmentFilenameList is empty" 错误，
// SDK 将其静默处理并返回空列表（不返回 error）。
func (c *Client) FetchCrashAttachments(ctx context.Context, appID string, p FetchCrashAttachmentsParams) (*FetchCrashAttachmentsResponse, error) {
	filenames := p.AttachmentFilenameList
	if len(filenames) == 0 {
		filenames = []string{"SDK_LOG"}
	}

	body := map[string]any{
		"appId":                  appID,
		"crashIdList":            p.CrashIDList,
		"attachmentFilenameList": filenames,
	}

	var out FetchCrashAttachmentsResponse
	if err := c.post(ctx, apiPathPrefix+"/fetchCrashAttachments", body, &out); err != nil {
		// 附件列表为空时服务端返回业务错误，静默处理返回空结构
		var apiErr *APIError
		if errors.As(err, &apiErr) {
			if containsStr(apiErr.Message, "attachmentFilenameList is empty") {
				return &FetchCrashAttachmentsResponse{}, nil
			}
		}
		return nil, err
	}
	return &out, nil
}

// containsStr 检查 s 中是否包含 sub 子串（避免引入 strings 包）。
func containsStr(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	if len(s) < len(sub) {
		return false
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
