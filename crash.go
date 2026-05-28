package crashsight

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// ─────────────────────────────────────────────────────────────────────────────
//  异常分析 API
// ─────────────────────────────────────────────────────────────────────────────

// GetCrashList 根据 issueId 获取崩溃列表（支持 PC）。
//
// 对应接口: POST /uniform/openapi/crashList
func (c *Client) GetCrashList(ctx context.Context, appID string, platform Platform, p GetCrashListParams) (*CrashListResponse, error) {
	rows := intDefault(p.Rows, 50)
	exType := strDefault(p.ExceptionTypeList, ExceptionTypeCrash)
	pid := int(platform)

	body := map[string]any{
		"appId":             appID,
		"crashDataType":     "undefined",
		"start":             p.Start,
		"searchType":        "detail",
		"exceptionTypeList": exType,
		"pid":               pid,
		"platformId":        pid,
		"issueId":           p.IssueID,
		"rows":              rows,
		"version":           p.Version,
	}

	var out CrashListResponse
	if err := c.post(ctx, apiPathPrefix+"/crashList", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetLastCrash 获取问题最近一次崩溃的 crashId 和基础信息（支持 PC）。
//
// 对应接口: POST /uniform/openapi/lastCrashInfo
//
// 注意: platformId 该接口要求字符串类型，SDK 内部已处理。
func (c *Client) GetLastCrash(ctx context.Context, appID string, platform Platform, issueID string) (*LastCrashResponse, error) {
	body := map[string]any{
		"appId":         appID,
		"platformId":    strconv.Itoa(int(platform)), // 字符串
		"issues":        issueID,
		"crashDataType": "undefined",
	}

	var out LastCrashResponse
	if err := c.post(ctx, apiPathPrefix+"/lastCrashInfo", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetCrashDetail 获取崩溃追踪数据、系统日志、用户日志及自定义 KV。
//
// 对应接口: POST /uniform/openapi/appDetailCrash
//
// 注意: platformId 该接口要求字符串类型，SDK 内部已处理。
func (c *Client) GetCrashDetail(ctx context.Context, appID string, platform Platform, crashHash string) (*CrashDetailResponse, error) {
	body := map[string]any{
		"appId":      appID,
		"platformId": strconv.Itoa(int(platform)), // 字符串
		"crashHash":  crashHash,
	}

	var out CrashDetailResponse
	if err := c.post(ctx, apiPathPrefix+"/appDetailCrash", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetCrashDoc 获取完整崩溃详情（崩溃堆栈、错误、ANR，支持 PC）。
//
// 对应接口: POST /uniform/openapi/crashDoc
//
// p.LogType 仅 PC 有效：""（默认）/"interface"/"file"/"all"。
// crashHash 格式: 将 crashId 每 2 位插入 ":"，可使用 CrashIDToHash 辅助函数生成。
func (c *Client) GetCrashDoc(ctx context.Context, appID string, platform Platform, crashHash string, p GetCrashDocParams) (*CrashDocResponse, error) {
	body := map[string]any{
		"appId":             appID,
		"platformId":        strconv.Itoa(int(platform)), // 字符串
		"crashHash":         crashHash,
		"logtype":           p.LogType,
		"needQueryCustomKv": p.NeedCustomKV,
	}

	var out CrashDocResponse
	if err := c.post(ctx, apiPathPrefix+"/crashDoc", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetANRMessage 获取 ANR 消息及 ANR Trace 文件。
//
// 对应接口: POST /uniform/openapi/appDetailCrash
//
// 返回结果中筛选 FileName 为 "anrMessage.txt" 或 "trace.zip" 的条目。
func (c *Client) GetANRMessage(ctx context.Context, appID string, platform Platform, crashHash string) ([]AttachItem, error) {
	detail, err := c.GetCrashDetail(ctx, appID, platform, crashHash)
	if err != nil {
		return nil, fmt.Errorf("GetANRMessage: %w", err)
	}
	var result []AttachItem
	for _, a := range detail.AttachList {
		if a.FileName == "anrMessage.txt" || a.FileName == "trace.zip" {
			result = append(result, a)
		}
	}
	return result, nil
}

// QueryCrashList 根据筛选条件拉取崩溃列表详情。
//
// 对应接口: POST /uniform/openapi/queryCrashList
func (c *Client) QueryCrashList(ctx context.Context, appID string, platform Platform, p QueryCrashListParams) (map[string]any, error) {
	body := map[string]any{
		"appId":      appID,
		"platformId": int(platform),
		"type":       string(crashTypeDefault(p.CrashType, CrashTypeCrash)),
		"start":      p.Start,
		"rows":       intDefault(p.Rows, 50),
		"version":    p.Version,
		"startDate":  p.StartDate,
		"endDate":    p.EndDate,
	}
	if p.Model != "" {
		body["model"] = p.Model
	}
	if p.OsVersion != "" {
		body["osVersion"] = p.OsVersion
	}
	if p.Keyword != "" {
		body["keyword"] = p.Keyword
	}
	if p.UserID != "" {
		body["userId"] = p.UserID
	}
	if p.DeviceID != "" {
		body["deviceId"] = p.DeviceID
	}
	if p.CrashID != "" {
		body["crashId"] = p.CrashID
	}

	var out map[string]any
	if err := c.post(ctx, apiPathPrefix+"/queryCrashList", body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// AdvancedSearch 崩溃分析自定义检索。
//
// 对应接口: POST /uniform/openapi/advancedSearchEx
//
// p.StartHour/EndHour 格式: YYYYMMDDHH
func (c *Client) AdvancedSearch(ctx context.Context, appID string, platform Platform, p AdvancedSearchParams) (map[string]any, error) {
	body := map[string]any{
		"appId":      appID,
		"platformId": int(platform),
		"startHour":  p.StartHour,
		"endHour":    p.EndHour,
		"type":       string(crashTypeDefault(p.CrashType, CrashTypeCrash)),
		"version":    strDefault(p.Version, "-1"),
		"dataType":   "realTimeTrendData",
		"vm":         int(p.VM),
	}

	var out map[string]any
	if err := c.post(ctx, apiPathPrefix+"/advancedSearchEx", body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetStackCrashStat 根据堆栈关键字获取崩溃统计。
//
// 对应接口: POST /uniform/openapi/getStackCrashStat/platformId/{pid}
//
// p.KeyName 支持 * 通配符。
func (c *Client) GetStackCrashStat(ctx context.Context, appID string, platform Platform, p GetStackCrashStatParams) (*StackCrashStatResponse, error) {
	pid := int(platform)
	path := apiPathPrefix + "/getStackCrashStat/platformId/" + strconv.Itoa(pid)
	body := map[string]any{
		"requestid": newFSN(),
		"stime":     p.StartTime,
		"etime":     p.EndTime,
		"source":    0,
		"appId":     appID,
		"params": map[string]any{
			"keyName": p.KeyName,
			"appId":   appID,
		},
		"limit": p.Limit,
		"type":  "pretty",
	}

	var out StackCrashStatResponse
	if err := c.post(ctx, path, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetCrashInfo 通过 GET 接口获取 issue 备注（旧版路径参数风格，内部复用）。
// 对外暴露 GetCrashDoc / GetCrashDetail 即可，此为内部辅助方法示例。
func (c *Client) getCrashDocByGet(ctx context.Context, appID string, platform Platform, crashHash string) (*CrashDocResponse, error) {
	path := apiPathPrefix + "/crashDoc/appId/" + appID +
		"/platformId/" + strconv.Itoa(int(platform)) +
		"/crashHash/" + crashHash
	q := url.Values{"fsn": {newFSN()}}

	var out CrashDocResponse
	if err := c.get(ctx, path, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
