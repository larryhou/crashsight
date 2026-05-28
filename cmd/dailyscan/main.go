// dailyscan 扫描指定时间窗口内所有崩溃问题，输出每条 crash 的完整设备信息。
//
// crashList GET 接口的 crashDatas 已包含 gpu/gpuDriverVersion/cpuName/memSize，
// 无需逐条调 GetCrashDoc，每个 issue 只需一次请求。
//
// 运行（默认只看今天，过滤 Physical.RealisticMP / Cloud.RealisticMP）:
//
//	go run ./cmd/dailyscan -out report.json
//
// -days N 表示累计最近 N+1 天（N 天前到今天）:
//
//	go run ./cmd/dailyscan -days 2 -out report.json   # 3天前到今天
//
// 自定义版本前缀:
//
//	go run ./cmd/dailyscan -version-prefix Physical.Ma3 -version-prefix Cloud.Ma3
//
// 调试（只看前 N 个 issue）:
//
//	go run ./cmd/dailyscan -max-issues 5
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	crashsight "github.com/larryhou/crashsight"
)

// versionPrefixes 支持 -version-prefix 多次指定
type versionPrefixes []string

func (v *versionPrefixes) String() string     { return strings.Join(*v, ",") }
func (v *versionPrefixes) Set(s string) error { *v = append(*v, s); return nil }

// matchesPrefix 判断版本号是否匹配任意前缀（空列表表示不过滤）
func matchesPrefix(version string, prefixes []string) bool {
	if len(prefixes) == 0 {
		return true
	}
	for _, p := range prefixes {
		if strings.HasPrefix(version, p) {
			return true
		}
	}
	return false
}

// CrashEntry 单条 crash 的设备详情。
type CrashEntry struct {
	CrashID          string `json:"crashId"`
	UploadTime       string `json:"uploadTime"`
	AppVersion       string `json:"appVersion"`
	GPU              string `json:"gpu"`
	GPUDriverVersion string `json:"gpuDriverVersion"`
	CPU              string `json:"cpu"`
	MemoryMB         int64  `json:"memoryMB"`
	DeviceID         string `json:"deviceId"`
	UserID           string `json:"userId"`
	OsVer            string `json:"osVer"`
}

// IssueReport 单个 issue 及其 crash 列表。
type IssueReport struct {
	IssueID       string       `json:"issueId"`
	ExceptionName string       `json:"exceptionName"`
	ExceptionMsg  string       `json:"exceptionMessage,omitempty"`
	RawStack      string       `json:"rawStack,omitempty"`
	CrashNum      int64        `json:"crashNum"`
	CrashUser     int64        `json:"crashUser"`
	Crashes       []CrashEntry `json:"crashes"`
}

// ScanReport 整体扫描报告。
type ScanReport struct {
	StartDate  string        `json:"startDate"`
	EndDate    string        `json:"endDate"`
	AppID      string        `json:"appId"`
	Platform   string        `json:"platform"`
	Prefixes   []string      `json:"versionPrefixes"`
	TotalIssue int           `json:"totalIssue"`
	TotalCrash int64         `json:"totalCrash"`
	Issues     []IssueReport `json:"issues"`
}

func main() {
	daysFlag := flag.Int("days", 0, "时间窗口天数，0=仅今天，N=N天前到今天（共N+1天）")
	maxIssues := flag.Int("max-issues", 50, "最多扫描 issue 数量，0 表示全部")
	outFile := flag.String("out", "", "输出到文件（JSON），默认 stdout")
	rowsPerIssue := flag.Int("rows", 0, "每个 issue 最多拉取的 crash 总数，0 表示全量分页拉取")
	var prefixes versionPrefixes
	flag.Var(&prefixes, "version-prefix", "版本号前缀过滤，可多次指定（默认: Physical.RealisticMP Cloud.RealisticMP）")
	flag.Parse()

	if len(prefixes) == 0 {
		prefixes = versionPrefixes{"Physical.RealisticMP", "Cloud.RealisticMP"}
	}

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

	today := time.Now()
	endDate := today.Format("20060102")
	startDate := today.AddDate(0, 0, -*daysFlag).Format("20060102")

	// 构建日期集合用于 crash 过滤（格式 YYYY-MM-DD）
	dateSet := make(map[string]bool)
	for i := 0; i <= *daysFlag; i++ {
		d := today.AddDate(0, 0, -i)
		dateSet[d.Format("2006-01-02")] = true
	}

	log.Printf("开始扫描 appId=%s startDate=%s endDate=%s 版本前缀=%v", appID, startDate, endDate, []string(prefixes))

	// ── Step 1: 拉取时间窗口内 TOP issue 列表（1次请求）─────────────────
	issues, err := fetchAllIssues(ctx, client, appID, startDate, endDate, *maxIssues)
	if err != nil {
		log.Fatalf("获取 issue 列表失败: %v", err)
	}
	log.Printf("共获取到 %d 个 issue", len(issues))

	report := ScanReport{
		StartDate: startDate,
		EndDate:   endDate,
		AppID:     appID,
		Platform:  "PC",
		Prefixes:  []string(prefixes),
	}

	// ── Step 2: 每个 issue 一次 GetCrashList（rows=500）──────────
	// crashDatas 已包含 gpu/gpuDriverVersion/cpuName/memSize，无需 GetCrashDoc
	for i, issue := range issues {
		log.Printf("[%d/%d] issueId=%s crashNum=%d %s",
			i+1, len(issues), issue.IssueID, issue.CrashNum, issue.ExceptionName)

		crashes, err := fetchCrashesForIssue(ctx, client, appID, issue.IssueID, *rowsPerIssue, []string(prefixes), dateSet, startDate)
		if err != nil {
			logAPIError(fmt.Sprintf("GetCrashList issueId=%s", issue.IssueID), err)
		}

		// 过滤后为空则跳过
		if len(crashes) == 0 {
			log.Printf("  跳过（过滤后无 crash）")
			time.Sleep(2500 * time.Millisecond)
			continue
		}

		// 按过滤后结果修正 crashNum/crashUser（crashUser 按 deviceId 去重）
		uniqueDevices := make(map[string]struct{})
		for _, c := range crashes {
			if c.DeviceID != "" {
				uniqueDevices[c.DeviceID] = struct{}{}
			}
		}

		entry := IssueReport{
			IssueID:       issue.IssueID,
			ExceptionName: issue.ExceptionName,
			ExceptionMsg:  issue.ExceptionMessage,
			RawStack:      issue.KeyStack,
			CrashNum:      int64(len(crashes)),
			CrashUser:     int64(len(uniqueDevices)),
			Crashes:       crashes,
		}
		report.Issues = append(report.Issues, entry)
		report.TotalCrash += int64(len(crashes))

		// 限速：25次/分钟，每次请求后等待
		time.Sleep(2500 * time.Millisecond)
	}

	report.TotalIssue = len(report.Issues)
	log.Printf("扫描完成：%d issues，%d crash 条目（过滤后）", report.TotalIssue, report.TotalCrash)

	// ── Step 3: 输出 ──────────────────────────────────────────────
	enc := json.NewEncoder(os.Stdout)
	if *outFile != "" {
		f, err := os.Create(*outFile)
		if err != nil {
			log.Fatalf("创建输出文件失败: %v", err)
		}
		defer f.Close()
		enc = json.NewEncoder(f)
		log.Printf("报告写入: %s", *outFile)
	}
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		log.Fatalf("JSON 输出失败: %v", err)
	}
}

// fetchAllIssues 拉取时间窗口内 TOP issue 列表（1次请求）。
func fetchAllIssues(ctx context.Context, client *crashsight.Client, appID, minDate, maxDate string, maxIssues int) ([]crashsight.IssueItem, error) {
	resp, err := client.GetTopIssues(ctx, appID, crashsight.PlatformPC, crashsight.GetTopIssuesParams{
		MinDate:          minDate,
		MaxDate:          maxDate,
		VersionList:      []string{"-1"},
		CrashType:        crashsight.CrashTypeCrash,
		Limit:            100,
		TopIssueDataType: crashsight.TopIssueDataTypeUnSystemExit,
		MergeDates:       true,
	})
	if err != nil {
		return nil, err
	}
	all := resp.TopIssueList
	if maxIssues > 0 && len(all) > maxIssues {
		all = all[:maxIssues]
	}
	return all, nil
}

// fetchCrashesForIssue 分页拉取 issue 下所有 crash（每页 100 条，按 numFound 循环）。
// crashDatas 已包含 gpu/gpuDriverVersion/cpuName/memSize，无需调 GetCrashDoc。
// dateSet 为允许的日期集合（格式 YYYY-MM-DD），只保留集合内日期的数据。
// minDateDash 为最早日期（格式 YYYY-MM-DD），crashList 倒序返回，早于此日期可提前终止。
func fetchCrashesForIssue(ctx context.Context, client *crashsight.Client, appID, issueID string, maxRows int, prefixes []string, dateSet map[string]bool, minDate string) ([]CrashEntry, error) {
	// 将 minDate YYYYMMDD 转为 YYYY-MM-DD 便于与 uploadTime 比较
	minDateDash := minDate[:4] + "-" + minDate[4:6] + "-" + minDate[6:]

	const pageSize = 100
	entries := make([]CrashEntry, 0)
	start := 0
	numFound := -1

	for page := 1; ; page++ {
		resp, err := client.GetCrashList(ctx, appID, crashsight.PlatformPC, crashsight.GetCrashListParams{
			IssueID: issueID,
			Start:   start,
			Rows:    pageSize,
		})
		if err != nil {
			return entries, err
		}

		if numFound < 0 {
			numFound = int(resp.NumFound)
			log.Printf("  numFound=%d，预计 %d 页", numFound, (numFound+pageSize-1)/pageSize)
		}

		matched := 0
		outOfDate := 0
		for _, crashID := range resp.CrashIDList {
			d := resp.CrashDatas[crashID]

			// 日期过滤：uploadTime 格式 "2026-05-28 18:36:11"，取前10字符作为日期键
			var uploadDateKey string
			if len(d.UploadTime) >= 10 {
				uploadDateKey = d.UploadTime[:10]
			}
			if !dateSet[uploadDateKey] {
				outOfDate++
				continue
			}
			if !matchesPrefix(d.ProductVersion, prefixes) {
				continue
			}
			matched++

			var memMB int64
			if d.MemSize != "" {
				var b int64
				fmt.Sscanf(d.MemSize, "%d", &b)
				memMB = b / 1024 / 1024
			}

			entries = append(entries, CrashEntry{
				CrashID:          crashID,
				UploadTime:       d.UploadTime,
				AppVersion:       d.ProductVersion,
				GPU:              d.GPU,
				GPUDriverVersion: d.GpuDriverVersion,
				CPU:              d.CpuName,
				MemoryMB:         memMB,
				DeviceID:         d.DeviceID,
				UserID:           d.UserID,
				OsVer:            d.OsVer,
			})
		}

		log.Printf("  第%d页 start=%d returned=%d matched=%d outOfDate=%d total_collected=%d",
			page, start, len(resp.CrashIDList), matched, outOfDate, len(entries))

		start += pageSize

		// crashList 按 uploadTime 倒序，一旦当页所有条目的日期都早于 minDateDash 可提前终止
		if len(resp.CrashIDList) > 0 {
			last := resp.CrashDatas[resp.CrashIDList[len(resp.CrashIDList)-1]]
			var lastDate string
			if len(last.UploadTime) >= 10 {
				lastDate = last.UploadTime[:10]
			}
			if lastDate != "" && lastDate < minDateDash {
				log.Printf("  末条 uploadTime=%s 早于 %s，提前终止", last.UploadTime, minDateDash)
				break
			}
		}
		if len(resp.CrashIDList) < pageSize || start >= numFound {
			break
		}
		if maxRows > 0 && start >= maxRows {
			log.Printf("  已达 -rows=%d 上限，停止", maxRows)
			break
		}

		time.Sleep(2500 * time.Millisecond)
	}

	return entries, nil
}

func logAPIError(method string, err error) {
	var apiErr *crashsight.APIError
	var authErr *crashsight.AuthError
	var rateErr *crashsight.RateLimitError
	switch {
	case errors.As(err, &apiErr):
		log.Printf("[%s] 业务错误: %s (code=%d)", method, apiErr.Message, apiErr.StatusCode)
	case errors.As(err, &authErr):
		log.Fatalf("[%s] 鉴权失败: %s", method, authErr.Message)
	case errors.As(err, &rateErr):
		log.Printf("[%s] 触发限速，等待 15s...", method)
		time.Sleep(15 * time.Second)
	default:
		log.Printf("[%s] 错误: %v", method, err)
	}
}
