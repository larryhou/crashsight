package crashsight

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// ─────────────────────────────────────────────────────────────────────────────
//  趋势统计 API
// ─────────────────────────────────────────────────────────────────────────────

// GetTrend 获取指定日期范围内的每日趋势数据。
//
// 对应接口: POST /uniform/openapi/getTrendEx
//
// 示例:
//
//	items, err := client.GetTrend(ctx, "appId", crashsight.PlatformPC,
//	    crashsight.GetTrendParams{
//	        StartDate: "20260301",
//	        EndDate:   "20260327",
//	    },
//	)
func (c *Client) GetTrend(ctx context.Context, appID string, platform Platform, p GetTrendParams) ([]TrendDataItem, error) {
	versionList := stringsDefault(p.VersionList, []string{"-1"})
	body := map[string]any{
		"appId":      appID,
		"platformId": int(platform),
		"startDate":  p.StartDate,
		"endDate":    p.EndDate,
		"type":       string(crashTypeDefault(p.CrashType, CrashTypeCrash)),
		"dataType":   "trendData",
		"vm":         int(p.VM),
		"versionList": versionList,
		"needCountryDimension":                       p.NeedCountry,
		"countryList":                                p.CountryList,
		"mergeMultipleVersionsWithInaccurateResult":   p.MergeVersions,
	}
	if len(p.CountryList) == 0 {
		body["countryList"] = []string{}
	}
	if len(p.SceneTags) > 0 {
		body["userSceneTagList"] = p.SceneTags
	}

	var out []TrendDataItem
	if err := c.post(ctx, apiPathPrefix+"/getTrendEx", body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetDailySummary 获取日级精简统计数据（含 crash/anr/error/oom/jank/hang 等全类型）。
//
// 对应接口: POST /uniform/openapi/fetchDailySummary
func (c *Client) GetDailySummary(ctx context.Context, appID string, platform Platform, p GetDailySummaryParams) ([]DailySummaryItem, error) {
	version := strDefault(p.Version, "-1")
	body := map[string]any{
		"appId":           appID,
		"platformId":      int(platform),
		"version":         version,
		"subModuleId":     "-1",
		"startHour":       p.StartDate + "00",
		"endHour":         p.EndDate + "23",
		"queryAllVmTypes": p.QueryAllVmTypes,
	}

	var out []DailySummaryItem
	if err := c.post(ctx, apiPathPrefix+"/fetchDailySummary", body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetRealtimeTrendAppend 获取今日累计趋势（小时粒度，当天从 00 到 23 小时的累计值）。
//
// 对应接口: POST /uniform/openapi/getAppRealTimeTrendAppendEx
func (c *Client) GetRealtimeTrendAppend(ctx context.Context, appID string, platform Platform, p GetRealtimeTrendAppendParams) ([]TrendDataItem, error) {
	version := strDefault(p.Version, "-1")
	body := map[string]any{
		"appId":      appID,
		"platformId": int(platform),
		"startDate":  p.Date + "00",
		"endDate":    p.Date + "23",
		"type":       string(crashTypeDefault(p.CrashType, CrashTypeCrash)),
		"dataType":   "realTimeTrendData",
		"vm":         int(p.VM),
		"version":    version,
		"mergeMultipleVersionsWithInaccurateResult": p.MergeVersions,
		"needCountryDimension":                      p.NeedCountry,
		"countryList":                               p.CountryList,
	}
	if len(p.CountryList) == 0 {
		body["countryList"] = []string{}
	}

	var out []TrendDataItem
	if err := c.post(ctx, apiPathPrefix+"/getAppRealTimeTrendAppendEx", body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetHourlyTrend 获取小时粒度趋势数据。
//
// 对应接口: POST /uniform/openapi/getRealTimeHourlyStatEx
//
// startHour/endHour 格式: YYYYMMDDHH，最多跨越 360 小时。
func (c *Client) GetHourlyTrend(ctx context.Context, appID string, platform Platform, p GetHourlyTrendParams) ([]TrendDataItem, error) {
	version := strDefault(p.Version, "-1")
	body := map[string]any{
		"appId":      appID,
		"platformId": int(platform),
		"startHour":  p.StartHour,
		"endHour":    p.EndHour,
		"version":    version,
		"type":       string(crashTypeDefault(p.CrashType, CrashTypeCrash)),
		"dataType":   "realTimeCompareData",
		"vm":         int(p.VM),
		"mergeMultipleVersionsWithInaccurateResult": p.MergeVersions,
		"needCountryDimension":                      p.NeedCountry,
		"countryList":                               p.CountryList,
	}
	if len(p.CountryList) == 0 {
		body["countryList"] = []string{}
	}

	var out []TrendDataItem
	if err := c.post(ctx, apiPathPrefix+"/getRealTimeHourlyStatEx", body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetHourlyTopIssues 获取小时级 TOP 问题列表。
//
// 对应接口: POST /uniform/openapi/getTopIssueHourly
func (c *Client) GetHourlyTopIssues(ctx context.Context, appID string, platform Platform, p GetHourlyTopIssuesParams) (*HourlyTopIssuesResponse, error) {
	version := strDefault(p.Version, "-1")
	limit := intDefault(p.Limit, 5)
	body := map[string]any{
		"appId":       appID,
		"platformId":  int(platform),
		"startHour":   p.StartHour,
		"version":     version,
		"type":        string(crashTypeDefault(p.CrashType, CrashTypeCrash)),
		"vm":          int(p.VM),
		"limit":       limit,
		"countryList": p.CountryList,
	}
	if len(p.CountryList) == 0 {
		body["countryList"] = []string{}
	}

	var out HourlyTopIssuesResponse
	if err := c.post(ctx, apiPathPrefix+"/getTopIssueHourly", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetDimensionTopStats 获取崩溃/ANR/错误在指定维度的排行榜。
//
// 对应接口: POST /uniform/openapi/fetchDimensionTopStats
//
// p.Field 取值: "model"（设备型号）、"osVersion"（系统版本）、"version"（应用版本）。
func (c *Client) GetDimensionTopStats(ctx context.Context, appID string, platform Platform, p GetDimensionTopStatsParams) ([]map[string]any, error) {
	field := strDefault(p.Field, "model")
	limit := intDefault(p.Limit, 20)
	needCountry := "false"
	if p.NeedCountry {
		needCountry = "true"
	}
	body := map[string]any{
		"appId":      appID,
		"platformId": int(platform),
		"version":    p.Version,
		"minDate":    p.MinDate,
		"maxDate":    p.MaxDate,
		"type":       string(crashTypeDefault(p.CrashType, CrashTypeCrash)),
		"field":      field,
		"limit":      limit,
		"mergeMultipleVersionsWithInaccurateResult": p.MergeVersions,
		"mergeMultipleDatesWithInaccurateResult":    p.MergeDates,
		"sortByException":                           p.SortByException,
		"needCountryDimension":                      needCountry,
		"countryList":                               p.CountryList,
	}
	if len(p.CountryList) == 0 {
		body["countryList"] = []string{}
	}

	var out []map[string]any
	if err := c.post(ctx, apiPathPrefix+"/fetchDimensionTopStats", body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetMinuteCrashData 获取分钟级崩溃数据。
//
// 对应接口: POST /uniform/openapi/getMinuteCrashData
//
// 注意: 该接口仅部分项目支持，且响应可能超过 30 秒，建议使用
// WithTimeout(120*time.Second) 创建客户端。
func (c *Client) GetMinuteCrashData(ctx context.Context, appID string, p GetMinuteCrashDataParams) ([]map[string]any, error) {
	version := strDefault(p.ProductVersion, "-1")
	limit := intDefault(p.Limit, 10)
	body := map[string]any{
		"appId":           appID,
		"stime":           p.StartTime,
		"etime":           p.EndTime,
		"product_version": version,
		"limit":           limit,
	}

	var out []map[string]any
	if err := c.post(ctx, apiPathPrefix+"/getMinuteCrashData", body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetRealTimeAppendStat 获取当日累计统计（GET 接口）。
//
// 对应接口: GET /uniform/openapi/getRealTimeAppendStat
//
// startHour/endHour 格式: YYYYMMDDHH（必须是同一天，小时部分必须为 00）。
func (c *Client) GetRealTimeAppendStat(ctx context.Context, appID string, platform Platform, startHour, endHour string, crashType CrashType) ([]DailySummaryItem, error) {
	ct := crashTypeDefault(crashType, CrashTypeCrash)
	q := url.Values{
		"appId":      {appID},
		"platformId": {strconv.Itoa(int(platform))},
		"startHour":  {startHour},
		"endHour":    {endHour},
		"type":       {string(ct)},
		"fsn":        {newFSN()},
	}

	var out []DailySummaryItem
	if err := c.get(ctx, apiPathPrefix+"/getRealTimeAppendStat", q, &out); err != nil {
		return nil, fmt.Errorf("GetRealTimeAppendStat: %w", err)
	}
	return out, nil
}
