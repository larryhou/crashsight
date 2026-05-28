package crashsight

import (
	"context"
	"strconv"
)

// ─────────────────────────────────────────────────────────────────────────────
//  用户与设备 API
// ─────────────────────────────────────────────────────────────────────────────

// QueryUserAccessList 查询用户/设备近 3 日异常数据上报记录。
//
// 对应接口: POST /uniform/openapi/queryAccessList
//
// p.UserIDList 与 p.DeviceIDList 二选一，都不传则返回全量（受 PageSize 限制）。
func (c *Client) QueryUserAccessList(ctx context.Context, appID string, platform Platform, p QueryUserAccessListParams) (map[string]any, error) {
	pageNumber := intDefault(p.PageNumber, 1)
	pageSize := intDefault(p.PageSize, 3000)

	body := map[string]any{
		"appId":                 appID,
		"platformId":            int(platform),
		"uploadTimeBeginMillis": p.UploadTimeBeginMillis,
		"skipDistinctQuery":     p.SkipDistinctQuery,
		"pageNumber":            pageNumber,
		"pageSize":              pageSize,
	}
	if len(p.UserIDList) > 0 {
		body["userIdList"] = p.UserIDList
	}
	if len(p.DeviceIDList) > 0 {
		body["deviceIdList"] = p.DeviceIDList
	}

	var out map[string]any
	if err := c.post(ctx, apiPathPrefix+"/queryAccessList", body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetCrashUserInfo 根据 openId（用户 ID）获取用户崩溃详情。
//
// 对应接口: POST /uniform/openapi/getCrashUserInfo/platformId/{pid}
//
// p.StartTime/EndTime 格式: YYYY-MM-DD HH:MM:SS
func (c *Client) GetCrashUserInfo(ctx context.Context, appID string, platform Platform, p GetCrashUserInfoParams) (*CrashUserInfoResponse, error) {
	pid := int(platform)
	limit := intDefault(p.Limit, 1000)
	reqID := strDefault(p.RequestID, newFSN())
	path := apiPathPrefix + "/getCrashUserInfo/platformId/" + strconv.Itoa(pid)

	body := map[string]any{
		"requestid": reqID,
		"stime":     p.StartTime,
		"etime":     p.EndTime,
		"type":      "pretty",
		"appId":     appID,
		"filters":   map[string]any{"user": p.UserIDs},
		"limit":     limit,
	}

	var out CrashUserInfoResponse
	if err := c.post(ctx, path, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetCrashUserList 获取时间范围内发生崩溃的用户列表。
//
// 对应接口: POST /uniform/openapi/getCrashUserList/platformId/{pid}
//
// 时间范围最长 30 天。p.StartDate/EndDate 格式: YYYYMMDD
func (c *Client) GetCrashUserList(ctx context.Context, appID string, platform Platform, p GetCrashUserListParams) (map[string]any, error) {
	pid := int(platform)
	version := strDefault(p.Version, "-1")
	path := apiPathPrefix + "/getCrashUserList/platformId/" + strconv.Itoa(pid)

	body := map[string]any{
		"appId":      appID,
		"platformId": pid,
		"startDate":  p.StartDate,
		"endDate":    p.EndDate,
		"type":       string(crashTypeDefault(p.CrashType, CrashTypeCrash)),
		"version":    version,
	}

	var out map[string]any
	if err := c.post(ctx, path, body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetMostReportUsers 获取 Top 崩溃上报用户。
//
// 对应接口: POST /uniform/openapi/getMostReportUser
func (c *Client) GetMostReportUsers(ctx context.Context, appID string, p GetMostReportUsersParams) ([]map[string]any, error) {
	versions := stringsDefault(p.Versions, []string{"-1"})
	timeRange := int64Default(p.TimeRangeMillis, 604800000)
	category := strDefault(p.ExceptionCategory, "CRASH")
	limit := intDefault(p.Limit, 10)
	// NeedDistinctCount: nil → 默认 true（与 Python SDK need_distinct_count=True 一致）。
	needDistinct := true
	if p.NeedDistinctCount != nil {
		needDistinct = *p.NeedDistinctCount
	}

	body := map[string]any{
		"appId": appID,
		"customFields": []map[string]any{
			{
				"available":         true,
				"name":              "userId",
				"aggregateType":     0,
				"termsSizeLimit":    limit,
				"needDistinctCount": needDistinct,
			},
		},
		"searchConditionGroup": SearchConditionGroup{
			Conditions: []SearchCondition{
				{
					QueryType: QueryTypeTermsWildcard,
					Terms:     versions,
					Field:     "version",
				},
				{
					Field:     "crashUploadTime",
					QueryType: QueryTypeRangeRelativeDatetime,
					Gte:       timeRange,
				},
				{
					QueryType: QueryTypeTerms,
					Terms:     []string{category},
					Field:     "exceptionCategory",
				},
			},
		},
	}

	var out []map[string]any
	if err := c.post(ctx, apiPathPrefix+"/getMostReportUser", body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetNetworkDevices 获取联网设备数据（仅移动端 Android/iOS）。
//
// 对应接口: POST /uniform/openapi/getNetworkDevices/platformId/{pid}
//
// 注意: 该接口偶发响应缓慢，建议使用 WithTimeout(60*time.Second)。
// 仅支持 PlatformAndroid 和 PlatformIOS。
func (c *Client) GetNetworkDevices(ctx context.Context, appID string, platform Platform, p GetNetworkDevicesParams) (map[string]any, error) {
	pid := int(platform)
	reqID := strDefault(p.RequestID, newFSN())
	path := apiPathPrefix + "/getNetworkDevices/platformId/" + strconv.Itoa(pid)

	body := map[string]any{
		"requestid": reqID,
		"stime":     p.StartTime,
		"etime":     p.EndTime,
		"appId":     appID,
	}

	var out map[string]any
	if err := c.post(ctx, path, body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetCrashDeviceStat 根据 deviceId 获取崩溃记录列表。
//
// 对应接口: POST /uniform/openapi/getCrashDeviceStat/platformId/{pid}
func (c *Client) GetCrashDeviceStat(ctx context.Context, appID string, platform Platform, p GetCrashDeviceStatParams) (*CrashDeviceStatResponse, error) {
	pid := int(platform)
	path := apiPathPrefix + "/getCrashDeviceStat/platformId/" + strconv.Itoa(pid)

	body := map[string]any{
		"requestid": newFSN(),
		"stime":     p.StartTime,
		"etime":     p.EndTime,
		"appId":     appID,
		"filters":   map[string]any{"deviceId": p.DeviceIDs},
		"limit":     p.Limit,
		"type":      "pretty",
	}

	var out CrashDeviceStatResponse
	if err := c.post(ctx, path, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetCrashDeviceInfo 根据 issueId 获取崩溃设备信息列表（移动端）。
//
// 对应接口: POST /uniform/openapi/getCrashDeviceInfo/platformId/{pid}
//
// 仅支持 PlatformAndroid 和 PlatformIOS。
func (c *Client) GetCrashDeviceInfo(ctx context.Context, appID string, platform Platform, p GetCrashDeviceInfoParams) (*CrashDeviceInfoResponse, error) {
	pid := int(platform)
	limit := intDefault(p.Limit, 1000)
	reqID := strDefault(p.RequestID, newFSN())
	path := apiPathPrefix + "/getCrashDeviceInfo/platformId/" + strconv.Itoa(pid)

	body := map[string]any{
		"requestid": reqID,
		"stime":     p.StartTime,
		"etime":     p.EndTime,
		"type":      "pretty",
		"appId":     appID,
		"filters":   map[string]any{"issueId": p.IssueIDs},
		"limit":     limit,
	}

	var out CrashDeviceInfoResponse
	if err := c.post(ctx, path, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetDeviceUserInfo 根据设备 ID 获取关联用户 OpenId。
//
// 对应接口: POST /uniform/openapi/getDeviceUserInfo/platformId/{pid}
//
// 仅支持 PlatformAndroid 和 PlatformIOS。
func (c *Client) GetDeviceUserInfo(ctx context.Context, appID string, platform Platform, p GetDeviceUserInfoParams) (*DeviceUserInfoResponse, error) {
	pid := int(platform)
	limit := intDefault(p.Limit, 10)
	reqID := strDefault(p.RequestID, newFSN())
	path := apiPathPrefix + "/getDeviceUserInfo/platformId/" + strconv.Itoa(pid)

	body := map[string]any{
		"requestid": reqID,
		"stime":     p.StartTime,
		"etime":     p.EndTime,
		"source":    0,
		"filters":   map[string]any{},
		"deviceId":  p.DeviceID,
		"limit":     limit,
		"type":      "pretty",
		"appId":     appID,
	}

	var out DeviceUserInfoResponse
	if err := c.post(ctx, path, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetStackDeviceInfo 根据堆栈关键字获取受影响的机型列表。
//
// 对应接口: POST /uniform/openapi/getStackDeviceInfo/platformId/{pid}
//
// p.KeyName 支持 * 通配符。
func (c *Client) GetStackDeviceInfo(ctx context.Context, appID string, platform Platform, p GetStackDeviceInfoParams) (*StackDeviceInfoResponse, error) {
	pid := int(platform)
	path := apiPathPrefix + "/getStackDeviceInfo/platformId/" + strconv.Itoa(pid)

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

	var out StackDeviceInfoResponse
	if err := c.post(ctx, path, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetCrashDeviceInfoByExpUID 根据 expUid 获取崩溃设备信息（移动端）。
//
// 对应接口: POST /uniform/openapi/getCrashDeviceInfoByExpUid/platformId/{pid}
func (c *Client) GetCrashDeviceInfoByExpUID(ctx context.Context, appID string, platform Platform, p GetCrashDeviceInfoByExpUIDParams) (map[string]any, error) {
	pid := int(platform)
	path := apiPathPrefix + "/getCrashDeviceInfoByExpUid/platformId/" + strconv.Itoa(pid)

	body := map[string]any{
		"requestid": newFSN(),
		"stime":     p.StartTime,
		"etime":     p.EndTime,
		"appId":     appID,
		"filters":   map[string]any{"expUid": p.ExpUIDs},
		"limit":     p.Limit,
		"type":      "pretty",
	}

	var out map[string]any
	if err := c.post(ctx, path, body, &out); err != nil {
		return nil, err
	}
	return out, nil
}
