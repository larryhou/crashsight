package crashsight

import (
	"context"
	"net/url"
	"strconv"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
//  问题管理 API
// ─────────────────────────────────────────────────────────────────────────────

// GetIssueList 获取崩溃/ANR/错误分析列表。
//
// 对应接口: POST /uniform/openapi/queryIssueList
func (c *Client) GetIssueList(ctx context.Context, p GetIssueListParams) (*IssueListResponse, error) {
	exType := strDefault(p.ExceptionTypeList, ExceptionTypeCrash)
	rows := intDefault(p.Rows, 20)
	sortField := strDefault(p.SortField, "uploadTime")
	sortOrder := strDefault(p.SortOrder, "desc")
	pid := int(c.platform)

	body := map[string]any{
		"appId":             c.appID,
		"platformId":        pid,
		"pid":               strconv.Itoa(pid),
		"exceptionTypeList": exType,
		"rows":              rows,
		"sortField":         sortField,
		"sortOrder":         sortOrder,
		"skipQueryHbase":    true,
	}
	if p.Start > 0 {
		body["start"] = p.Start
	}
	if p.Status != "" {
		body["status"] = p.Status
	}
	if p.Version != "" {
		body["version"] = p.Version
	}
	if p.TapdBugStatus != "" {
		body["tapdBugStatus"] = p.TapdBugStatus
	}
	if p.IssueUploadTimeRelativeMillis > 0 {
		body["issueUploadTimeRelativeMillis"] = p.IssueUploadTimeRelativeMillis
	}

	var out IssueListResponse
	if err := c.post(ctx, apiPathPrefix+"/queryIssueList", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetTopIssues 获取 TOP 问题列表。
//
// 对应接口: POST /uniform/openapi/getTopIssueEx
func (c *Client) GetTopIssues(ctx context.Context, p GetTopIssuesParams) (*TopIssuesResponse, error) {
	versionList := stringsDefault(p.VersionList, []string{"-1"})
	limit := intDefault(p.Limit, 20)
	dataType := p.TopIssueDataType
	if dataType == "" {
		dataType = TopIssueDataTypeUnSystemExit
	}
	version := "-1"
	if len(versionList) > 0 {
		version = versionList[0]
	}

	body := map[string]any{
		"appId":      c.appID,
		"platformId": int(c.platform),
		"version":    version,
		"date":       p.MinDate,
		"minDate":    p.MinDate,
		"maxDate":    p.MaxDate,
		"type":       string(crashTypeDefault(p.CrashType, CrashTypeCrash)),
		"limit":      limit,
		"topIssueDataType":                           string(dataType),
		"mergeMultipleVersionsWithInaccurateResult":   p.MergeVersions,
		"mergeMultipleDatesWithInaccurateResult":      p.MergeDates,
		"versionList":                                versionList,
		"countryList":                                p.CountryList,
	}
	if len(p.CountryList) == 0 {
		body["countryList"] = []string{}
	}

	var out TopIssuesResponse
	if err := c.post(ctx, apiPathPrefix+"/getTopIssueEx", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetIssueInfo 获取单个 issue 的详细信息。
//
// 对应接口: POST /uniform/openapi/issueInfo
//
// 注意: platformId 需传字符串类型，SDK 内部已处理。
func (c *Client) GetIssueInfo(ctx context.Context, issueID string) (*IssueInfo, error) {
	body := map[string]any{
		"appId":      c.appID,
		"platformId": strconv.Itoa(int(c.platform)), // 该接口要求字符串类型
		"issueId":    issueID,
	}

	var out IssueInfo
	if err := c.post(ctx, apiPathPrefix+"/issueInfo", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetIssueNotes 获取 issue 的备注列表。
//
// 对应接口: GET /uniform/openapi/noteList/appId/{appId}/platformId/{pid}/issueId/{issueId}
//
// crashDataType 通常传 "undefined"，留空时使用该默认值。
func (c *Client) GetIssueNotes(ctx context.Context, issueID string, crashDataType string) ([]IssueNote, error) {
	path := apiPathPrefix + "/noteList/appId/" + c.appID +
		"/platformId/" + strconv.Itoa(int(c.platform)) +
		"/issueId/" + issueID
	query := url.Values{
		"crashDataType": {strDefault(crashDataType, "undefined")},
	}

	var out []IssueNote
	if err := c.get(ctx, path, query, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetIssueTrend 获取问题趋势数据。
//
// 对应接口: POST /uniform/openapi/queryIssueTrend
//
// p.MinDate/MaxDate 格式: YYYY-MM-DD HH:MM:SS
func (c *Client) GetIssueTrend(ctx context.Context, p GetIssueTrendParams) ([]IssueTrendItem, error) {
	granularity := p.GranularityUnit
	if granularity == "" {
		granularity = GranularityDay
	}
	version := strDefault(p.Version, "-1")

	body := map[string]any{
		"appId":           c.appID,
		"platformId":      int(c.platform),
		"issueIds":        p.IssueIDs,
		"granularityUnit": string(granularity),
		"minDate":         p.MinDate,
		"maxDate":         p.MaxDate,
		"version":         version,
	}

	var out []IssueTrendItem
	if err := c.post(ctx, apiPathPrefix+"/queryIssueTrend", body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateIssueStatus 更新问题状态（处理/未处理/处理中）。
//
// 对应接口: POST /uniform/openapi/updateIssueStatus
func (c *Client) UpdateIssueStatus(ctx context.Context, p UpdateIssueStatusParams) error {
	operatorID := strDefault(p.OperatorUserID, c.userID)
	body := map[string]any{
		"appId":      c.appID,
		"platformId": int(c.platform),
		"issueIds":   p.IssueIDs,
		"status":     int(p.Status),
		"processors": p.Processors,
		"note":       p.Note,
		"newUserId":  operatorID,
	}
	return c.post(ctx, apiPathPrefix+"/updateIssueStatus", body, nil)
}

// AddIssueNote 为问题添加备注。
//
// 对应接口: POST /uniform/openapi/addIssueNote
func (c *Client) AddIssueNote(ctx context.Context, p AddIssueNoteParams) error {
	uid := strDefault(p.OperatorUserID, c.userID)
	now := time.Now().Format("2006-01-02 15:04:05")
	body := map[string]any{
		"appId":      c.appID,
		"platformId": int(c.platform),
		"issueStatus": 3,
		"issueIds":   p.IssueID,
		"note":       p.Note,
		"createTime": now,
		"userId":     uid,
		"newUserId":  uid,
	}
	return c.post(ctx, apiPathPrefix+"/addIssueNote", body, nil)
}

// AddIssueTag 为问题设置标签。
//
// 对应接口: POST /uniform/openapi/addTag
func (c *Client) AddIssueTag(ctx context.Context, issueID, tagName string) error {
	body := map[string]any{
		"appId":      c.appID,
		"platformId": int(c.platform),
		"issueId":    issueID,
		"tagName":    tagName,
	}
	return c.post(ctx, apiPathPrefix+"/addTag", body, nil)
}

// UpsertBugs 创建或更新关联的缺陷单（TAPD 等）。
//
// 对应接口: POST /uniform/openapi/upsertBugs
func (c *Client) UpsertBugs(ctx context.Context, p UpsertBugsParams) (map[string]any, error) {
	body := map[string]any{
		"appId":      c.appID,
		"platformId": int(c.platform),
		"issueId":    p.IssueID,
	}
	for k, v := range p.Extra {
		body[k] = v
	}

	var out map[string]any
	if err := c.post(ctx, apiPathPrefix+"/upsertBugs", body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// QueryBugs 查询缺陷单详情。
//
// 对应接口: POST /uniform/openapi/queryBugs
func (c *Client) QueryBugs(ctx context.Context, bugInfos []BugInfoParam) (map[string]any, error) {
	body := map[string]any{
		"appId":      c.appID,
		"platformId": int(c.platform),
		"bugInfos":   bugInfos,
	}

	var out map[string]any
	if err := c.post(ctx, apiPathPrefix+"/queryBugs", body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// BindBugs 绑定已有缺陷单到问题。
//
// 对应接口: POST /uniform/openapi/bindBugs
func (c *Client) BindBugs(ctx context.Context, p BindBugsParams) error {
	body := map[string]any{
		"appId":      c.appID,
		"platformId": int(c.platform),
		"issueId":    p.IssueID,
		"bugId":      p.BugID,
		"bugUrl":     p.BugURL,
	}
	return c.post(ctx, apiPathPrefix+"/bindBugs", body, nil)
}
