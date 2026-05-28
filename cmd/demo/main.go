// demo 展示 CrashSight Go SDK 的常见使用方式。
//
// 运行: go run ./cmd/demo
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	crashsight "github.com/larryhou/crashsight"
)

func main() {
	// ── 初始化客户端 ──────────────────────────────────────────────
	userID := os.Getenv("CRASHSIGHT_USER_ID")
	apiKey := os.Getenv("CRASHSIGHT_API_KEY")
	if userID == "" || apiKey == "" {
		log.Fatal("请设置环境变量 CRASHSIGHT_USER_ID 和 CRASHSIGHT_API_KEY")
	}

	client := crashsight.NewClient(userID, apiKey,
		crashsight.WithRegion(crashsight.RegionCN),
		crashsight.WithTimeout(30*time.Second),
	)

	ctx := context.Background()
	appID := "your_app_id" // 替换为实际项目 ID

	// ── 示例 1: 获取每日崩溃趋势 ─────────────────────────────────
	fmt.Println("=== 每日崩溃趋势 ===")
	trends, err := client.GetTrend(ctx, appID, crashsight.PlatformPC, crashsight.GetTrendParams{
		StartDate:     "20260520",
		EndDate:       "20260527",
		CrashType:     crashsight.CrashTypeCrash,
		VersionList:   []string{"-1"},
		MergeVersions: false,
	})
	if err != nil {
		handleError("GetTrend", err)
	} else {
		printJSON(trends)
	}

	// ── 示例 2: 获取 TOP 问题列表 ─────────────────────────────────
	fmt.Println("\n=== TOP 问题列表 ===")
	topIssues, err := client.GetTopIssues(ctx, appID, crashsight.PlatformPC, crashsight.GetTopIssuesParams{
		MinDate:          "20260520",
		MaxDate:          "20260527",
		VersionList:      []string{"-1"},
		CrashType:        crashsight.CrashTypeCrash,
		Limit:            10,
		TopIssueDataType: crashsight.TopIssueDataTypeUnSystemExit,
		MergeDates:       true,
	})
	if err != nil {
		handleError("GetTopIssues", err)
	} else {
		printJSON(topIssues)
	}

	// ── 示例 3: 级联查询 issue → lastCrash → crashDoc ────────────
	fmt.Println("\n=== 级联查询崩溃详情 ===")
	issueID := "your_issue_id" // 替换为实际 issueId

	issueInfo, err := client.GetIssueInfo(ctx, appID, crashsight.PlatformPC, issueID)
	if err != nil {
		handleError("GetIssueInfo", err)
		return
	}
	fmt.Printf("Issue: %s | %s\n", issueInfo.IssueID, issueInfo.ExceptionName)

	time.Sleep(2 * time.Second) // 遵守 25 次/分钟限速

	lastCrash, err := client.GetLastCrash(ctx, appID, crashsight.PlatformPC, issueID)
	if err != nil {
		handleError("GetLastCrash", err)
		return
	}
	fmt.Printf("Last CrashID: %s\n", lastCrash.CrashID)

	time.Sleep(2 * time.Second)

	crashHash := crashsight.CrashIDToHash(lastCrash.CrashID)
	doc, err := client.GetCrashDoc(ctx, appID, crashsight.PlatformPC, crashHash, crashsight.GetCrashDocParams{})
	if err != nil {
		handleError("GetCrashDoc", err)
		return
	}
	fmt.Printf("KeyStack:\n%s\n", doc.CrashMap.KeyStack)

	// ── 示例 4: 获取选择器元数据（版本列表等）───────────────────────
	fmt.Println("\n=== 选择器数据 ===")
	selector, err := client.GetSelectorData(ctx, appID, crashsight.PlatformPC, crashsight.GetSelectorDataParams{
		Types: "version,tag",
	})
	if err != nil {
		handleError("GetSelectorData", err)
	} else {
		fmt.Printf("版本数量: %d\n", len(selector.VersionList))
		for i, v := range selector.VersionList {
			if i >= 5 {
				fmt.Println("  ...")
				break
			}
			fmt.Printf("  %s\n", v.ProductVersion)
		}
	}

	// ── 示例 5: 并发调用（展示并发安全）──────────────────────────
	fmt.Println("\n=== 并发查询（3 个平台）===")
	platforms := []crashsight.Platform{
		crashsight.PlatformAndroid,
		crashsight.PlatformIOS,
		crashsight.PlatformPC,
	}
	type result struct {
		platform crashsight.Platform
		count    int
		err      error
	}
	ch := make(chan result, len(platforms))

	for _, p := range platforms {
		p := p // capture loop variable
		go func() {
			issues, err := client.GetTopIssues(ctx, appID, p, crashsight.GetTopIssuesParams{
				MinDate:    "20260527",
				MaxDate:    "20260527",
				Limit:      5,
				MergeDates: true,
			})
			if err != nil {
				ch <- result{platform: p, err: err}
				return
			}
			ch <- result{platform: p, count: len(issues.TopIssueList)}
		}()
	}

	for range platforms {
		r := <-ch
		if r.err != nil {
			fmt.Printf("  %s: 错误 - %v\n", r.platform, r.err)
		} else {
			fmt.Printf("  %s: %d 个问题\n", r.platform, r.count)
		}
	}

	// ── 示例 6: 错误处理 ──────────────────────────────────────────
	fmt.Println("\n=== 错误处理示例 ===")
	_, err = client.GetIssueInfo(ctx, appID, crashsight.PlatformPC, "invalid_issue_id")
	if err != nil {
		var apiErr *crashsight.APIError
		var authErr *crashsight.AuthError
		var rateErr *crashsight.RateLimitError

		switch {
		case errors.As(err, &apiErr):
			fmt.Printf("业务错误: %s (code=%d, traceId=%s)\n",
				apiErr.Message, apiErr.StatusCode, apiErr.TraceID)
		case errors.As(err, &authErr):
			fmt.Printf("鉴权失败: %s\n", authErr.Message)
		case errors.As(err, &rateErr):
			fmt.Println("触发限速，稍后重试")
		default:
			fmt.Printf("其他错误: %v\n", err)
		}
	}
}

// handleError 统一打印错误，不终止程序。
func handleError(method string, err error) {
	fmt.Fprintf(os.Stderr, "[%s] 错误: %v\n", method, err)
}

// printJSON 将任意对象序列化为缩进 JSON 并打印。
func printJSON(v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(b))
}
