// compare — 输出各接口的 Go SDK 响应（JSON 格式），供对比脚本使用。
//
// 用法:
//
//	go run ./cmd/compare <case> [args...]
//
// 接口 case:
//
//	selector              GetSelectorData
//	trend <start> <end>   GetTrend（YYYYMMDD）
//	top_issues <start> <end>  GetTopIssues
//	issue_info <issueId>  GetIssueInfo
//	last_crash <issueId>  GetLastCrash
//	crash_doc <crashId>   GetCrashDoc（crashId 自动转 hash）
//	issue_list            GetIssueList
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	sdk "github.com/larryhou/crashsight"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: compare <case> [args...]")
		os.Exit(1)
	}

	userID := mustEnv("CRASHSIGHT_USER_ID")
	apiKey := mustEnv("CRASHSIGHT_API_KEY")
	appID := mustEnv("CRASHSIGHT_APP_ID")
	region := os.Getenv("CRASHSIGHT_REGION")
	if region == "" {
		region = "cn"
	}

	platform := sdk.PlatformPC

	client := sdk.NewClient(sdk.Config{
		UserID:   userID,
		APIKey:   apiKey,
		AppID:    appID,
		Platform: platform,
		Region:   sdk.Region(region),
	}, sdk.WithTimeout(60*time.Second))
	ctx := context.Background()

	caseName := os.Args[1]
	args := os.Args[2:]

	var result any
	var err error

	switch caseName {

	case "selector":
		result, err = client.GetSelectorData(ctx, sdk.GetSelectorDataParams{
			Types: "version,tag,member,bundle,channel",
		})

	case "trend":
		start, end := argDate(args, 0, -7), argDate(args, 1, 0)
		result, err = client.GetTrend(ctx, sdk.GetTrendParams{
			StartDate:     start,
			EndDate:       end,
			CrashType:     sdk.CrashTypeCrash,
			VersionList:   []string{"-1"},
			MergeVersions: true,
		})

	case "top_issues":
		start, end := argDate(args, 0, -7), argDate(args, 1, 0)
		result, err = client.GetTopIssues(ctx, sdk.GetTopIssuesParams{
			MinDate:          start,
			MaxDate:          end,
			VersionList:      []string{"-1"},
			CrashType:        sdk.CrashTypeCrash,
			Limit:            10,
			TopIssueDataType: sdk.TopIssueDataTypeUnSystemExit,
			MergeDates:       true,
		})

	case "issue_list":
		result, err = client.GetIssueList(ctx, sdk.GetIssueListParams{
			ExceptionTypeList: sdk.ExceptionTypeCrash,
			Rows:              10,
			SortField:         "uploadTime",
			SortOrder:         "desc",
		})

	case "issue_info":
		issueID := mustArg(args, 0, "issueId")
		result, err = client.GetIssueInfo(ctx, issueID)

	case "last_crash":
		issueID := mustArg(args, 0, "issueId")
		result, err = client.GetLastCrash(ctx, issueID)

	case "crash_doc":
		crashID := mustArg(args, 0, "crashId")
		crashHash := sdk.CrashIDToHash(crashID)
		result, err = client.GetCrashDoc(ctx, crashHash, sdk.GetCrashDocParams{})

	case "crash_detail":
		crashID := mustArg(args, 0, "crashId")
		crashHash := sdk.CrashIDToHash(crashID)
		result, err = client.GetCrashDetail(ctx, crashHash)

	case "daily_summary":
		start, end := argDate(args, 0, -7), argDate(args, 1, 0)
		result, err = client.GetDailySummary(ctx, sdk.GetDailySummaryParams{
			StartDate: start,
			EndDate:   end,
		})

	case "hourly_trend":
		startHour := argHour(args, 0, -24)
		endHour := argHour(args, 1, 0)
		result, err = client.GetHourlyTrend(ctx, sdk.GetHourlyTrendParams{
			StartHour:     startHour,
			EndHour:       endHour,
			MergeVersions: true,
		})

	case "version_date_list":
		result, err = client.GetVersionDateList(ctx, )

	case "issue_notes":
		issueID := mustArg(args, 0, "issueId")
		result, err = client.GetIssueNotes(ctx, issueID, "")

	default:
		fmt.Fprintf(os.Stderr, "unknown case: %s\n", caseName)
		os.Exit(1)
	}

	if err != nil {
		// 输出结构化错误，方便 Python 侧解析
		out := map[string]any{
			"__error__": err.Error(),
			"__type__":  fmt.Sprintf("%T", err),
		}
		printJSON(out)
		os.Exit(2)
	}

	printJSON(result)
}

func printJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(os.Stderr, "json encode error: %v\n", err)
		os.Exit(3)
	}
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		fmt.Fprintf(os.Stderr, "missing env: %s\n", key)
		os.Exit(1)
	}
	return v
}

func mustArg(args []string, i int, name string) string {
	if i >= len(args) || args[i] == "" {
		fmt.Fprintf(os.Stderr, "missing arg: %s\n", name)
		os.Exit(1)
	}
	return args[i]
}

func argDate(args []string, i, offsetDays int) string {
	if i < len(args) && args[i] != "" {
		return args[i]
	}
	return time.Now().AddDate(0, 0, offsetDays).Format("20060102")
}

func argHour(args []string, i, offsetHours int) string {
	if i < len(args) && args[i] != "" {
		return args[i]
	}
	return time.Now().Add(time.Duration(offsetHours) * time.Hour).Format("2006010215")
}
