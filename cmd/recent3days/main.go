// recent3days 演示如何查询最近三天的崩溃数据，支持按版本过滤。
//
// 运行（自动过滤最近三天发布的版本）:
//
//	go run ./cmd/recent3days
//
// 运行（手动指定版本）:
//
//	go run ./cmd/recent3days -version "Physical.RealisticMP.2026-05-28.6238352"
//
// 运行（多个版本）:
//
//	go run ./cmd/recent3days -version "v1.0" -version "v1.1"
//
// 列出可用版本:
//
//	go run ./cmd/recent3days -list-versions
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	crashsight "github.com/larryhou/crashsight"
)

// multiFlag 支持 -version 重复传入多个值
type multiFlag []string

func (f *multiFlag) String() string        { return fmt.Sprint([]string(*f)) }
func (f *multiFlag) Set(v string) error    { *f = append(*f, v); return nil }

func main() {
	var versions multiFlag
	flag.Var(&versions, "version", "手动指定版本过滤（可多次指定）；不传则自动取最近 N 天内首次出现的版本")
	listVersions := flag.Bool("list-versions", false, "列出当前 app 所有版本后退出")
	days := flag.Int("days", 3, "查询最近 N 天（默认 3）")
	flag.Parse()

	userID := os.Getenv("CRASHSIGHT_USER_ID")
	apiKey := os.Getenv("CRASHSIGHT_API_KEY")
	appID := os.Getenv("CRASHSIGHT_APP_ID")
	region := os.Getenv("CRASHSIGHT_REGION")
	if userID == "" || apiKey == "" || appID == "" {
		log.Fatal("请设置环境变量 CRASHSIGHT_USER_ID / CRASHSIGHT_API_KEY / CRASHSIGHT_APP_ID")
	}

	r := crashsight.RegionCN
	if region == "sg" {
		r = crashsight.RegionSG
	}

	client := crashsight.NewClient(userID, apiKey,
		crashsight.WithRegion(r),
		crashsight.WithTimeout(60*time.Second),
	)
	ctx := context.Background()

	// ── 列出版本后退出 ────────────────────────────────────────────
	if *listVersions {
		sel, err := client.GetSelectorData(ctx, appID, crashsight.PlatformPC, crashsight.GetSelectorDataParams{Types: "version"})
		if err != nil {
			log.Fatalf("GetSelectorData: %v", err)
		}
		fmt.Printf("共 %d 个版本:\n", len(sel.VersionList))
		for _, v := range sel.VersionList {
			fmt.Printf("  %s\n", v.ProductVersion)
		}
		return
	}

	now := time.Now()
	start := now.AddDate(0, 0, -*days)
	minDate := start.Format("20060102")
	maxDate := now.Format("20060102")

	// ── 确定版本列表 ──────────────────────────────────────────────
	var versionList []string
	if len(versions) > 0 {
		// 手动指定
		versionList = []string(versions)
		fmt.Printf("查询时间范围: %s ~ %s  版本过滤（手动）: %v\n\n", minDate, maxDate, versionList)
	} else {
		// 自动：从 GetVersionDateList 中过滤出 first_date 在查询窗口内的版本
		fmt.Printf("正在获取最近 %d 天内发布的版本...\n", *days)
		versionList = recentVersions(ctx, client, appID, start, now)
		if len(versionList) == 0 {
			fmt.Println("该时间段内无新发布版本，改用全部版本")
			versionList = []string{"-1"}
			fmt.Printf("查询时间范围: %s ~ %s  版本: 全部\n\n", minDate, maxDate)
		} else {
			fmt.Printf("查询时间范围: %s ~ %s  版本过滤（自动，共 %d 个）:\n", minDate, maxDate, len(versionList))
			for _, v := range versionList {
				fmt.Printf("  %s\n", v)
			}
			fmt.Println()
		}
	}

	time.Sleep(2 * time.Second)

	// ── 1. 每日趋势 ──────────────────────────────────────────────
	fmt.Println("=== 崩溃趋势（按天）===")
	trends, err := client.GetTrend(ctx, appID, crashsight.PlatformPC, crashsight.GetTrendParams{
		StartDate:     minDate,
		EndDate:       maxDate,
		CrashType:     crashsight.CrashTypeCrash,
		VersionList:   versionList,
		MergeVersions: len(versionList) > 1,
	})
	if err != nil {
		printError("GetTrend", err)
	} else {
		for _, item := range trends {
			fmt.Printf("  %s  崩溃次数=%-6d 崩溃用户=%-6d 活跃用户=%d\n",
				item.Date, item.CrashNum, item.CrashUser, item.AccessUser)
		}
	}

	time.Sleep(2 * time.Second)

	// ── 2. TOP 崩溃问题 ──────────────────────────────────────────
	// GetTopIssues 多版本时需 MergeDates=true，但不支持多版本精确 TOP（服务端限制）
	// 超过 1 个版本时退化为全版本查询并提示
	topVersionList := versionList
	if len(versionList) > 1 {
		fmt.Println("\n注意: TOP 问题接口不支持多版本精确过滤，改用全部版本")
		topVersionList = []string{"-1"}
	}
	fmt.Println("\n=== TOP 10 崩溃问题 ===")
	topResp, err := client.GetTopIssues(ctx, appID, crashsight.PlatformPC, crashsight.GetTopIssuesParams{
		MinDate:          minDate,
		MaxDate:          maxDate,
		VersionList:      topVersionList,
		CrashType:        crashsight.CrashTypeCrash,
		Limit:            10,
		TopIssueDataType: crashsight.TopIssueDataTypeUnSystemExit,
		MergeDates:       true,
	})
	if err != nil {
		printError("GetTopIssues", err)
	} else {
		for i, issue := range topResp.TopIssueList {
			fmt.Printf("  #%-2d issueId=%-34s 崩溃次数=%-6d 用户数=%-6d %s\n",
				i+1, issue.IssueID, issue.CrashNum, issue.CrashUser, issue.ExceptionName)
		}
	}

	time.Sleep(2 * time.Second)

	// ── 3. 级联查询第一条问题的崩溃堆栈 ─────────────────────────
	if topResp != nil && len(topResp.TopIssueList) > 0 {
		issueID := topResp.TopIssueList[0].IssueID
		fmt.Printf("\n=== 级联查询 issueId=%s 的最新崩溃堆栈 ===\n", issueID)

		lastCrash, err := client.GetLastCrash(ctx, appID, crashsight.PlatformPC, issueID)
		if err != nil {
			printError("GetLastCrash", err)
		} else {
			fmt.Printf("最新 crashId: %s\n", lastCrash.CrashID)
			fmt.Printf("版本: %s  OS: %s\n", lastCrash.ProductVersion, lastCrash.OsVersion)

			time.Sleep(2 * time.Second)

			crashHash := crashsight.CrashIDToHash(lastCrash.CrashID)
			doc, err := client.GetCrashDoc(ctx, appID, crashsight.PlatformPC, crashHash, crashsight.GetCrashDocParams{})
			if err != nil {
				printError("GetCrashDoc", err)
			} else {
				fmt.Println("\nKeyStack:")
				fmt.Println(doc.CrashMap.KeyStack)
				if doc.CrashMap.CallStack != "" {
					fmt.Println("\nCallStack (前 800 字符):")
					cs := doc.CrashMap.CallStack
					if len(cs) > 800 {
						cs = cs[:800] + "\n..."
					}
					fmt.Println(cs)
				}
			}
		}
	}

	time.Sleep(2 * time.Second)

	// ── 4. 日级汇总 ──────────────────────────────────────────────
	fmt.Println("\n=== 日级汇总（DailySummary）===")
	// DailySummary 只支持单版本，多版本时取第一个
	summaryVersion := versionList[0]
	summary, err := client.GetDailySummary(ctx, appID, crashsight.PlatformPC, crashsight.GetDailySummaryParams{
		StartDate: minDate,
		EndDate:   maxDate,
		Version:   summaryVersion,
	})
	if err != nil {
		printError("GetDailySummary", err)
	} else {
		printJSON(summary)
	}
}

// recentVersions 从 GetVersionDateList 中筛选出首次出现日期在 [since, until] 范围内的版本号。
func recentVersions(ctx context.Context, client *crashsight.Client, appID string, since, until time.Time) []string {
	dates, err := client.GetVersionDateList(ctx, appID, crashsight.PlatformPC)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[GetVersionDateList] %v\n", err)
		return nil
	}

	// first_date 格式为 "2026-05-28"
	const layout = "2006-01-02"
	sinceDay := since.Format(layout)
	untilDay := until.Format(layout)

	seen := make(map[string]bool)
	var result []string
	for _, item := range dates {
		if item.Date >= sinceDay && item.Date <= untilDay {
			if !seen[item.Version] {
				seen[item.Version] = true
				result = append(result, item.Version)
			}
		}
	}
	return result
}

func printError(method string, err error) {
	var apiErr *crashsight.APIError
	var authErr *crashsight.AuthError
	var rateErr *crashsight.RateLimitError
	switch {
	case errors.As(err, &apiErr):
		fmt.Fprintf(os.Stderr, "[%s] 业务错误: %s (code=%d traceId=%s)\n",
			method, apiErr.Message, apiErr.StatusCode, apiErr.TraceID)
	case errors.As(err, &authErr):
		fmt.Fprintf(os.Stderr, "[%s] 鉴权失败: %s\n", method, authErr.Message)
	case errors.As(err, &rateErr):
		fmt.Fprintf(os.Stderr, "[%s] 触发限速，稍后重试\n", method)
	default:
		fmt.Fprintf(os.Stderr, "[%s] 错误: %v\n", method, err)
	}
}

func printJSON(v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(b))
}
