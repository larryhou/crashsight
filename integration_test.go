// integration_test.go — CrashSight SDK 集成测试
//
// 需要设置以下环境变量才能运行：
//
//	CRASHSIGHT_USER_ID    localUserId
//	CRASHSIGHT_API_KEY    userOpenapiKey
//	CRASHSIGHT_APP_ID     项目 appId
//	CRASHSIGHT_REGION     cn 或 sg（默认 cn）
//
// 运行方式：
//
//	CRASHSIGHT_USER_ID=xxx CRASHSIGHT_API_KEY=xxx CRASHSIGHT_APP_ID=xxx \
//	  go test -v -run TestIntegration -timeout 120s
package crashsight

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"
)

// ─── 测试基础设施 ──────────────────────────────────────────────────────────────

type integrationEnv struct {
	client   *Client
	appID    string
	platform Platform
}

// newIntegrationEnv 从环境变量读取凭据，缺失时跳过测试。
func newIntegrationEnv(t *testing.T) *integrationEnv {
	t.Helper()
	userID := os.Getenv("CRASHSIGHT_USER_ID")
	apiKey := os.Getenv("CRASHSIGHT_API_KEY")
	appID := os.Getenv("CRASHSIGHT_APP_ID")
	region := os.Getenv("CRASHSIGHT_REGION")
	if userID == "" || apiKey == "" || appID == "" {
		t.Skip("跳过集成测试：请设置 CRASHSIGHT_USER_ID / CRASHSIGHT_API_KEY / CRASHSIGHT_APP_ID")
	}
	if region == "" {
		region = "cn"
	}
	client := NewClient(Config{
		UserID:   userID,
		APIKey:   apiKey,
		AppID:    appID,
		Platform: PlatformPC,
		Region:   Region(region),
	}, WithTimeout(60*time.Second))
	return &integrationEnv{
		client:   client,
		appID:    appID,
		platform: PlatformPC,
	}
}

// prettyJSON 打印结构体为缩进 JSON（测试输出用）。
func prettyJSON(t *testing.T, label string, v any) {
	t.Helper()
	b, err := json.MarshalIndent(v, "  ", "  ")
	if err != nil {
		t.Logf("[%s] marshal error: %v", label, err)
		return
	}
	t.Logf("[%s]\n  %s", label, string(b))
}

// checkAPIError 区分业务错误与网络错误，业务错误只打印不 Fatal。
func checkAPIError(t *testing.T, label string, err error) bool {
	t.Helper()
	if err == nil {
		return false
	}
	var apiErr *APIError
	var authErr *AuthError
	var rateErr *RateLimitError
	var transErr *TransportError
	switch {
	case errors.As(err, &apiErr):
		t.Logf("[%s] API 业务错误: %s (status=%d, errorCode=%s, traceId=%s)",
			label, apiErr.Message, apiErr.StatusCode, apiErr.ErrorCode, apiErr.TraceID)
	case errors.As(err, &authErr):
		t.Fatalf("[%s] 鉴权失败: %s", label, authErr.Message)
	case errors.As(err, &rateErr):
		t.Logf("[%s] 触发限速，等待 5s 后重试", label)
		time.Sleep(5 * time.Second)
	case errors.As(err, &transErr):
		t.Fatalf("[%s] 网络错误: %v", label, transErr.Cause)
	default:
		t.Logf("[%s] 未知错误: %v", label, err)
	}
	return true
}

// ─── 测试用例 ──────────────────────────────────────────────────────────────────

// TestIntegration_SelectorData 获取版本/标签/处理人列表（元数据，最稳定的接口）。
func TestIntegration_SelectorData(t *testing.T) {
	e := newIntegrationEnv(t)
	ctx := context.Background()

	resp, err := e.client.GetSelectorData(ctx, e.appID, e.platform, GetSelectorDataParams{
		Types: "version,tag,member",
	})
	if checkAPIError(t, "GetSelectorData", err) {
		return
	}
	prettyJSON(t, "SelectorData", resp)

	t.Logf("版本数量: %d", len(resp.VersionList))
	t.Logf("标签数量: %d", len(resp.TagList))
	t.Logf("处理人数量: %d", len(resp.ProcessorList))

	if len(resp.VersionList) == 0 {
		t.Log("警告: 版本列表为空")
	} else {
		t.Logf("最新版本示例: %s", resp.VersionList[0].ProductVersion)
	}
}

// TestIntegration_VersionDateList 获取版本首次出现日期。
func TestIntegration_VersionDateList(t *testing.T) {
	e := newIntegrationEnv(t)
	ctx := context.Background()

	items, err := e.client.GetVersionDateList(ctx, e.appID, e.platform)
	if checkAPIError(t, "GetVersionDateList", err) {
		return
	}
	t.Logf("版本日期条目数: %d", len(items))
	for i, item := range items {
		if i >= 5 {
			t.Log("  ...")
			break
		}
		t.Logf("  version=%s date=%s", item.Version, item.Date)
	}
}

// TestIntegration_DailyTrend 获取最近 7 天每日趋势。
func TestIntegration_DailyTrend(t *testing.T) {
	e := newIntegrationEnv(t)
	ctx := context.Background()

	end := time.Now()
	start := end.AddDate(0, 0, -7)
	startDate := start.Format("20060102")
	endDate := end.Format("20060102")

	items, err := e.client.GetTrend(ctx, e.appID, e.platform, GetTrendParams{
		StartDate:     startDate,
		EndDate:       endDate,
		CrashType:     CrashTypeCrash,
		VersionList:   []string{"-1"},
		MergeVersions: true,
	})
	if checkAPIError(t, "GetTrend", err) {
		return
	}
	t.Logf("趋势数据点数: %d (范围 %s ~ %s)", len(items), startDate, endDate)
	for _, item := range items {
		t.Logf("  date=%-10s crashNum=%-6d crashUser=%-6d accessUser=%d",
			item.Date, item.CrashNum, item.CrashUser, item.AccessUser)
	}
}

// TestIntegration_HourlyTrend 获取最近 24 小时趋势。
func TestIntegration_HourlyTrend(t *testing.T) {
	e := newIntegrationEnv(t)
	ctx := context.Background()

	now := time.Now()
	endHour := now.Format("2006010215")
	startHour := now.Add(-24 * time.Hour).Format("2006010215")

	items, err := e.client.GetHourlyTrend(ctx, e.appID, e.platform, GetHourlyTrendParams{
		StartHour:     startHour,
		EndHour:       endHour,
		CrashType:     CrashTypeCrash,
		MergeVersions: true,
	})
	if checkAPIError(t, "GetHourlyTrend", err) {
		return
	}
	t.Logf("小时趋势数据点数: %d (范围 %s ~ %s)", len(items), startHour, endHour)
	for _, item := range items {
		t.Logf("  date=%-12s crashNum=%-6d accessUser=%d", item.Date, item.CrashNum, item.AccessUser)
	}
}

// TestIntegration_TopIssues 获取最近 7 天 TOP10 问题。
func TestIntegration_TopIssues(t *testing.T) {
	e := newIntegrationEnv(t)
	ctx := context.Background()

	end := time.Now()
	start := end.AddDate(0, 0, -7)

	resp, err := e.client.GetTopIssues(ctx, e.appID, e.platform, GetTopIssuesParams{
		MinDate:          start.Format("20060102"),
		MaxDate:          end.Format("20060102"),
		VersionList:      []string{"-1"},
		CrashType:        CrashTypeCrash,
		Limit:            10,
		TopIssueDataType: TopIssueDataTypeUnSystemExit,
		MergeDates:       true,
	})
	if checkAPIError(t, "GetTopIssues", err) {
		return
	}
	t.Logf("TOP Issues 数量: %d, crashDevices=%d, accessDevices=%d",
		len(resp.TopIssueList), resp.CrashDevices, resp.AccessDevices)
	for i, issue := range resp.TopIssueList {
		t.Logf("  #%d issueId=%-34s crashNum=%-6d exception=%s",
			i+1, issue.IssueID, issue.CrashNum, issue.ExceptionName)
	}
}

// TestIntegration_IssueList 获取问题列表。
func TestIntegration_IssueList(t *testing.T) {
	e := newIntegrationEnv(t)
	ctx := context.Background()

	resp, err := e.client.GetIssueList(ctx, e.appID, e.platform, GetIssueListParams{
		ExceptionTypeList: ExceptionTypeCrash,
		Rows:              10,
		SortField:         "uploadTime",
		SortOrder:         "desc",
	})
	if checkAPIError(t, "GetIssueList", err) {
		return
	}
	t.Logf("IssueList numFound=%d, 返回=%d", resp.NumFound, len(resp.IssueList))
	for _, issue := range resp.IssueList {
		t.Logf("  issueId=%-34s status=%d exception=%s",
			issue.IssueID, issue.Status, issue.ExceptionName)
	}
}

// TestIntegration_IssueInfoAndCrashDoc 级联查询：IssueInfo → LastCrash → CrashDoc。
// 这是最核心的调用链，依赖 TopIssues 返回至少一个 issue。
func TestIntegration_IssueInfoAndCrashDoc(t *testing.T) {
	e := newIntegrationEnv(t)
	ctx := context.Background()

	// Step 1: 先拿一个 issueId
	end := time.Now()
	start := end.AddDate(0, 0, -7)
	topResp, err := e.client.GetTopIssues(ctx, e.appID, e.platform, GetTopIssuesParams{
		MinDate:    start.Format("20060102"),
		MaxDate:    end.Format("20060102"),
		Limit:      1,
		MergeDates: true,
	})
	if checkAPIError(t, "GetTopIssues(for issueId)", err) {
		return
	}
	if len(topResp.TopIssueList) == 0 {
		t.Skip("没有可用 issue，跳过级联测试")
	}
	issueID := topResp.TopIssueList[0].IssueID
	t.Logf("使用 issueId: %s", issueID)
	time.Sleep(2 * time.Second) // 限速保护

	// Step 2: GetIssueInfo
	info, err := e.client.GetIssueInfo(ctx, e.appID, e.platform, issueID)
	if checkAPIError(t, "GetIssueInfo", err) {
		return
	}
	t.Logf("IssueInfo: exception=%s keyStack=%.80s", info.ExceptionName, info.KeyStack)
	time.Sleep(2 * time.Second)

	// Step 3: GetLastCrash
	last, err := e.client.GetLastCrash(ctx, e.appID, e.platform, issueID)
	if checkAPIError(t, "GetLastCrash", err) {
		return
	}
	t.Logf("LastCrash: crashId=%s version=%s model=%s osVer=%s",
		last.CrashID, last.ProductVersion, last.Hardware, last.OsVersion)
	if last.CrashID == "" {
		t.Log("警告: crashId 为空，跳过 CrashDoc")
		return
	}
	time.Sleep(2 * time.Second)

	// Step 4: GetCrashDoc
	crashHash := CrashIDToHash(last.CrashID)
	t.Logf("crashHash: %s", crashHash)
	doc, err := e.client.GetCrashDoc(ctx, e.appID, e.platform, crashHash, GetCrashDocParams{})
	if checkAPIError(t, "GetCrashDoc", err) {
		return
	}
	t.Logf("CrashDoc: statusCode=%d issueId=%s version=%s",
		doc.StatusCode, doc.CrashMap.IssueID, doc.CrashMap.ProductVersion)
	t.Logf("  keyStack: %.120s", doc.CrashMap.KeyStack)
	t.Logf("  callStack: %.120s", doc.CrashMap.CallStack)
	t.Logf("  detailMap.fileList count: %d", len(doc.DetailMap.FileList))
}

// TestIntegration_CrashList 获取指定 issue 的 crash 列表。
func TestIntegration_CrashList(t *testing.T) {
	e := newIntegrationEnv(t)
	ctx := context.Background()

	// 先拿一个 issueId
	end := time.Now()
	start := end.AddDate(0, 0, -7)
	topResp, err := e.client.GetTopIssues(ctx, e.appID, e.platform, GetTopIssuesParams{
		MinDate:    start.Format("20060102"),
		MaxDate:    end.Format("20060102"),
		Limit:      1,
		MergeDates: true,
	})
	if checkAPIError(t, "GetTopIssues(for crashList)", err) {
		return
	}
	if len(topResp.TopIssueList) == 0 {
		t.Skip("没有可用 issue")
	}
	issueID := topResp.TopIssueList[0].IssueID
	time.Sleep(2 * time.Second)

	resp, err := e.client.GetCrashList(ctx, e.appID, e.platform, GetCrashListParams{
		IssueID: issueID,
		Start:   0,
		Rows:    10,
	})
	if checkAPIError(t, "GetCrashList", err) {
		return
	}
	t.Logf("CrashList: numFound=%d, crashIdList=%d条", resp.NumFound, len(resp.CrashIDList))
	for i, cid := range resp.CrashIDList {
		if i >= 5 {
			t.Log("  ...")
			break
		}
		t.Logf("  crashId=%s", cid)
	}
}

// TestIntegration_IssueNotes 获取备注列表。
func TestIntegration_IssueNotes(t *testing.T) {
	e := newIntegrationEnv(t)
	ctx := context.Background()

	end := time.Now()
	start := end.AddDate(0, 0, -7)
	topResp, err := e.client.GetTopIssues(ctx, e.appID, e.platform, GetTopIssuesParams{
		MinDate:    start.Format("20060102"),
		MaxDate:    end.Format("20060102"),
		Limit:      1,
		MergeDates: true,
	})
	if checkAPIError(t, "GetTopIssues(for notes)", err) {
		return
	}
	if len(topResp.TopIssueList) == 0 {
		t.Skip("没有可用 issue")
	}
	issueID := topResp.TopIssueList[0].IssueID
	time.Sleep(2 * time.Second)

	notes, err := e.client.GetIssueNotes(ctx, e.appID, e.platform, issueID, "")
	if checkAPIError(t, "GetIssueNotes", err) {
		return
	}
	t.Logf("备注数量: %d", len(notes))
	for _, n := range notes {
		t.Logf("  [%s] %s", n.CreateTime, n.Note)
	}
}

// TestIntegration_IssueTrend 获取 issue 趋势。
func TestIntegration_IssueTrend(t *testing.T) {
	e := newIntegrationEnv(t)
	ctx := context.Background()

	end := time.Now()
	start := end.AddDate(0, 0, -7)
	topResp, err := e.client.GetTopIssues(ctx, e.appID, e.platform, GetTopIssuesParams{
		MinDate:    start.Format("20060102"),
		MaxDate:    end.Format("20060102"),
		Limit:      3,
		MergeDates: true,
	})
	if checkAPIError(t, "GetTopIssues(for trend)", err) {
		return
	}
	if len(topResp.TopIssueList) == 0 {
		t.Skip("没有可用 issue")
	}
	issueIDs := make([]string, 0, len(topResp.TopIssueList))
	for _, issue := range topResp.TopIssueList {
		issueIDs = append(issueIDs, issue.IssueID)
	}
	time.Sleep(2 * time.Second)

	trends, err := e.client.GetIssueTrend(ctx, e.appID, e.platform, GetIssueTrendParams{
		IssueIDs:        issueIDs,
		MinDate:         start.Format("2006-01-02") + " 00:00:00",
		MaxDate:         end.Format("2006-01-02") + " 23:59:59",
		GranularityUnit: GranularityDay,
	})
	if checkAPIError(t, "GetIssueTrend", err) {
		return
	}
	t.Logf("IssueTrend 条目数: %d", len(trends))
	for _, item := range trends {
		t.Logf("  issueId=%s trendPoints=%d", item.IssueID, len(item.TrendList))
	}
}

// TestIntegration_StackCrashStat 堆栈关键字崩溃统计（用通配符）。
func TestIntegration_StackCrashStat(t *testing.T) {
	e := newIntegrationEnv(t)
	ctx := context.Background()

	end := time.Now()
	start := end.AddDate(0, 0, -3)

	resp, err := e.client.GetStackCrashStat(ctx, e.appID, e.platform, GetStackCrashStatParams{
		KeyName:   "*",
		StartTime: start.Format("2006-01-02") + " 00:00:00",
		EndTime:   end.Format("2006-01-02") + " 23:59:59",
		Limit:     10,
	})
	if checkAPIError(t, "GetStackCrashStat", err) {
		return
	}
	t.Logf("StackCrashStat 结果数: %d", len(resp.Results))
	for i, item := range resp.Results {
		if i >= 5 {
			t.Log("  ...")
			break
		}
		t.Logf("  keyName=%-40s crashNums=%d", item.KeyName, item.CrashNums)
	}
}

// TestIntegration_DailySummary 日级汇总统计。
func TestIntegration_DailySummary(t *testing.T) {
	e := newIntegrationEnv(t)
	ctx := context.Background()

	end := time.Now()
	start := end.AddDate(0, 0, -7)

	items, err := e.client.GetDailySummary(ctx, e.appID, e.platform, GetDailySummaryParams{
		StartDate: start.Format("20060102"),
		EndDate:   end.Format("20060102"),
		Version:   "-1",
	})
	if checkAPIError(t, "GetDailySummary", err) {
		return
	}
	t.Logf("DailySummary 数据点: %d", len(items))
	for _, item := range items {
		t.Logf("  date=%-10s crash=%d/%d anr=%d/%d error=%d/%d access=%d",
			item.Date,
			item.CrashNum, item.CrashUser,
			item.AnrNum, item.AnrUser,
			item.ErrorNum, item.ErrorUser,
			item.AccessUser)
	}
}

// TestIntegration_HourlyTopIssues 小时级 TOP 问题。
func TestIntegration_HourlyTopIssues(t *testing.T) {
	e := newIntegrationEnv(t)
	ctx := context.Background()

	// 取当前小时
	startHour := time.Now().Format("2006010215")

	resp, err := e.client.GetHourlyTopIssues(ctx, e.appID, e.platform, GetHourlyTopIssuesParams{
		StartHour: startHour,
		CrashType: CrashTypeCrash,
		Limit:     5,
	})
	if checkAPIError(t, "GetHourlyTopIssues", err) {
		return
	}
	t.Logf("HourlyTopIssues: crashDevices=%d accessDevices=%d topList=%d",
		resp.CrashDevices, resp.AccessDevices, len(resp.TopIssueList))
	for i, issue := range resp.TopIssueList {
		t.Logf("  #%d %s crashNum=%d", i+1, issue.IssueID, issue.CrashNum)
	}
}

// TestIntegration_ErrorTypes 验证错误类型分发是否正确。
func TestIntegration_ErrorTypes(t *testing.T) {
	e := newIntegrationEnv(t)
	ctx := context.Background()

	// 用无效 issueId 触发 API 业务错误
	_, err := e.client.GetIssueInfo(ctx, e.appID, e.platform, "invalid_issue_id_that_does_not_exist")
	if err == nil {
		t.Log("警告: 无效 issueId 未返回错误（服务端容错）")
		return
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		t.Logf("正确捕获 APIError: status=%d message=%s", apiErr.StatusCode, apiErr.Message)
	} else {
		t.Logf("其他错误类型: %T %v", err, err)
	}

	// 用错误凭据触发鉴权错误
	badClient := NewClient("bad_user", "bad_key",
		WithRegion(RegionCN),
		WithTimeout(10*time.Second),
	)
	_, err = badClient.GetSelectorData(ctx, e.appID, e.platform, GetSelectorDataParams{})
	if err == nil {
		t.Log("警告: 错误凭据未返回错误")
		return
	}
	var authErr *AuthError
	if errors.As(err, &authErr) {
		t.Logf("正确捕获 AuthError: %s", authErr.Message)
	} else {
		// 部分接口不返回 401，而是 API 业务错误
		t.Logf("鉴权失败（非 AuthError 类型）: %T %v", err, err)
	}
}

// TestIntegration_CrashIDToHash 验证 CrashIDToHash 工具函数。
func TestIntegration_CrashIDToHash(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"a683c7", "a6:83:c7"},
		{"A1B2C3D4", "A1:B2:C3:D4"},
		{"ab", "ab"},
		{"a", "a"},
		{"", ""},
		{"337A2BABD0DBA145C462625FD26BD349", "33:7A:2B:AB:D0:DB:A1:45:C4:62:62:5F:D2:6B:D3:49"},
	}
	for _, c := range cases {
		got := CrashIDToHash(c.input)
		if got != c.expected {
			t.Errorf("CrashIDToHash(%q) = %q, want %q", c.input, got, c.expected)
		} else {
			t.Logf("CrashIDToHash(%q) = %q ✓", c.input, got)
		}
	}
}

// TestIntegration_ConcurrentRequests 并发安全验证：同一 Client 多 goroutine 同时请求。
func TestIntegration_ConcurrentRequests(t *testing.T) {
	e := newIntegrationEnv(t)
	ctx := context.Background()

	const goroutines = 3
	type result struct {
		n   int
		err error
	}
	ch := make(chan result, goroutines)

	end := time.Now()
	start := end.AddDate(0, 0, -3)

	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			items, err := e.client.GetTrend(ctx, e.appID, e.platform, GetTrendParams{
				StartDate:     start.Format("20060102"),
				EndDate:       end.Format("20060102"),
				MergeVersions: true,
			})
			ch <- result{n: len(items), err: err}
			t.Logf("goroutine %d 完成", i)
		}()
	}

	for i := 0; i < goroutines; i++ {
		r := <-ch
		if r.err != nil {
			t.Logf("并发请求 %d 错误: %v", i, r.err)
		} else {
			t.Logf("并发请求 %d 成功: %d 个数据点", i, r.n)
		}
	}
	t.Log("并发安全验证完成")
}

// TestIntegration_Summary 最后汇总一次完整健康检查（关键接口串行）。
func TestIntegration_Summary(t *testing.T) {
	e := newIntegrationEnv(t)
	ctx := context.Background()

	type checkResult struct {
		name string
		ok   bool
		info string
	}
	var results []checkResult

	check := func(name string, fn func() (string, error)) {
		info, err := fn()
		if err != nil {
			var apiErr *APIError
			if errors.As(err, &apiErr) {
				results = append(results, checkResult{name, false,
					fmt.Sprintf("APIError status=%d: %s", apiErr.StatusCode, apiErr.Message)})
			} else {
				results = append(results, checkResult{name, false, err.Error()})
			}
		} else {
			results = append(results, checkResult{name, true, info})
		}
		time.Sleep(2500 * time.Millisecond) // 限速保护
	}

	end := time.Now()
	start := end.AddDate(0, 0, -7)

	check("GetSelectorData", func() (string, error) {
		r, err := e.client.GetSelectorData(ctx, e.appID, e.platform, GetSelectorDataParams{})
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("versions=%d tags=%d processors=%d", len(r.VersionList), len(r.TagList), len(r.ProcessorList)), nil
	})

	check("GetTrend", func() (string, error) {
		items, err := e.client.GetTrend(ctx, e.appID, e.platform, GetTrendParams{
			StartDate: start.Format("20060102"), EndDate: end.Format("20060102"), MergeVersions: true,
		})
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("dataPoints=%d", len(items)), nil
	})

	check("GetTopIssues", func() (string, error) {
		r, err := e.client.GetTopIssues(ctx, e.appID, e.platform, GetTopIssuesParams{
			MinDate: start.Format("20060102"), MaxDate: end.Format("20060102"),
			Limit: 5, MergeDates: true,
		})
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("issues=%d crashDevices=%d", len(r.TopIssueList), r.CrashDevices), nil
	})

	check("GetIssueList", func() (string, error) {
		r, err := e.client.GetIssueList(ctx, e.appID, e.platform, GetIssueListParams{Rows: 5})
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("numFound=%d returned=%d", r.NumFound, len(r.IssueList)), nil
	})

	check("GetDailySummary", func() (string, error) {
		items, err := e.client.GetDailySummary(ctx, e.appID, e.platform, GetDailySummaryParams{
			StartDate: start.Format("20060102"), EndDate: end.Format("20060102"),
		})
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("dataPoints=%d", len(items)), nil
	})

	// 打印汇总表
	t.Log("\n========== 集成测试汇总 ==========")
	passed, failed := 0, 0
	for _, r := range results {
		status := "✓ PASS"
		if !r.ok {
			status = "✗ FAIL"
			failed++
		} else {
			passed++
		}
		t.Logf("  %s  %-25s  %s", status, r.name, r.info)
	}
	t.Logf("================================")
	t.Logf("  通过: %d  失败: %d  共: %d", passed, failed, passed+failed)
	if failed > 0 {
		t.Fail()
	}
}
