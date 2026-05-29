package main

import (
	"context"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	crashsight "github.com/larryhou/crashsight"
)

// versionPrefixes supports multiple -version-prefix flags.
type versionPrefixes []string

func (v *versionPrefixes) String() string     { return strings.Join(*v, ",") }
func (v *versionPrefixes) Set(s string) error { *v = append(*v, s); return nil }

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

// IssueReport holds a single issue's aggregated data and hardware sample.
type IssueReport struct {
	IssueID       string   `json:"issueId"`
	ExceptionName string   `json:"exceptionName"`
	ExceptionMsg  string   `json:"exceptionMessage"`
	RawStack      string   `json:"rawStack"`
	Processors    []string `json:"processors"`
	Status        int      `json:"status"`
	Tags          []string `json:"tags"`
	UploadTime    string   `json:"uploadTime"`

	// Accumulate totals (all-time, from IssueItem)
	TotalCrashNum  int64 `json:"totalCrashNum"`
	TotalDeviceNum int64 `json:"totalDeviceNum"`

	// Trend Data (Volume)
	TrendCrashNum  int64 `json:"trendCrashNum"`
	TrendCrashUser int64 `json:"trendCrashUser"`

	// Hardware Sample (from LastMatchedReport)
	SampleCrashID    string `json:"sampleCrashId"`
	GPU              string `json:"gpu"`
	GPUDriverVersion string `json:"gpuDriverVersion"`
	CPU              string `json:"cpu"`
	MemoryMB         int64  `json:"memoryMB"`
	FreeMemoryMB     int64  `json:"freeMemoryMB"`
	VRAMTotalMB      int64  `json:"vramTotalMB"`
	VRAMUsedMB       int64  `json:"vramUsedMB"`
	ElapsedTimeSec   int64  `json:"elapsedTimeSec"`
	OsVer            string `json:"osVer"`
	Country          string `json:"country"`
	AppVersion       string `json:"appVersion"`
}

type ScanReport struct {
	StartDate  string        `json:"startDate"`
	EndDate    string        `json:"endDate"`
	AppID      string        `json:"appId"`
	Platform   string        `json:"platform"`
	Region     string        `json:"region"`
	Prefixes   []string      `json:"versionPrefixes"`
	TotalIssue int           `json:"totalIssue"`
	TotalCrash int64         `json:"totalCrash"`
	Issues     []IssueReport `json:"issues"`
}

func main() {
	daysFlag := flag.Int("days", 0, "time window in days: 0=today only, N=N days ago to today")
	maxIssues := flag.Int("max-issues", 0, "max issues to scan, 0=all")
	outFile := flag.String("out", "", "output file (JSON); defaults to stdout")
	htmlFile := flag.String("html", "", "generate a standalone HTML visualization report")
	csvFile := flag.String("csv", "", "output flat CSV for data analysis tools")

	var prefixes versionPrefixes
	flag.Var(&prefixes, "version-prefix", "version prefix filter, repeatable")
	flag.Parse()

	if len(prefixes) == 0 {
		prefixes = versionPrefixes{"Physical.RealisticMP", "Cloud.RealisticMP"}
	}

	userID := os.Getenv("CRASHSIGHT_USER_ID")
	apiKey := os.Getenv("CRASHSIGHT_API_KEY")
	appID := os.Getenv("CRASHSIGHT_APP_ID")
	region := os.Getenv("CRASHSIGHT_REGION")
	if userID == "" || apiKey == "" || appID == "" {
		log.Fatal("env CRASHSIGHT_USER_ID / CRASHSIGHT_API_KEY / CRASHSIGHT_APP_ID must be set")
	}

	r := crashsight.RegionCN
	if region == "sg" {
		r = crashsight.RegionSG
	}

	client := crashsight.NewClient(crashsight.Config{
		UserID:   userID,
		APIKey:   apiKey,
		AppID:    appID,
		Platform: crashsight.PlatformPC,
		Region:   r,
	}, crashsight.WithTimeout(60*time.Second))

	ctx := context.Background()

	today := time.Now()
	startDateObj := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location()).
		AddDate(0, 0, -*daysFlag)
	startDate := startDateObj.Format("20060102")
	endDate := today.Format("20060102")

	// IssueUploadTimeRelativeMillis: 从 startDate 零点到当前时刻的毫秒数，让服务端直接过滤
	relativeMs := today.Sub(startDateObj).Milliseconds()

	log.Printf("scan start: appId=%s startDate=%s endDate=%s relativeMs=%d prefixes=%v",
		appID, startDate, endDate, relativeMs, []string(prefixes))

	// ── Step 1: Paginate queryIssueList ───────────────────────────────────────
	// 使用 IssueUploadTimeRelativeMillis 让服务端过滤时间窗口，避免客户端时间边界误判导致提前退出
	var collectedIssues []crashsight.IssueItem
	offset := 0
	pageSize := 1000

	for {
		log.Printf("fetching issue list start=%d", offset)
		var resp *crashsight.IssueListResponse
		var err error

		for retries := 0; retries < 3; retries++ {
			resp, err = client.GetIssueList(ctx, crashsight.GetIssueListParams{
				ExceptionTypeList:             crashsight.ExceptionTypeCrash,
				Rows:                          pageSize,
				Start:                         offset,
				SortField:                     "uploadTime",
				SortOrder:                     "desc",
				IssueUploadTimeRelativeMillis: relativeMs,
			})
			if err != nil {
				var rateErr *crashsight.RateLimitError
				if errors.As(err, &rateErr) {
					log.Printf("[GetIssueList] rate limited, waiting 15s... (retry %d/3)", retries+1)
					time.Sleep(15 * time.Second)
					continue
				}
				break
			}
			break
		}

		if err != nil {
			logAPIError("GetIssueList", err)
			break
		}

		if len(resp.IssueList) == 0 {
			break
		}
		log.Printf("  got %d issues (numFound=%d)", len(resp.IssueList), resp.NumFound)

		for _, issue := range resp.IssueList {
			// Version filter
			ver := issue.Version
			if issue.LastMatchedReport != nil && issue.LastMatchedReport.CrashMap.ProductVersion != "" {
				ver = issue.LastMatchedReport.CrashMap.ProductVersion
			} else if issue.FirstCrashVersion != "" {
				ver = issue.FirstCrashVersion
			}
			if matchesPrefix(ver, prefixes) {
				collectedIssues = append(collectedIssues, issue)
			}
		}

		if *maxIssues > 0 && len(collectedIssues) >= *maxIssues {
			collectedIssues = collectedIssues[:*maxIssues]
			break
		}

		offset += len(resp.IssueList)
		if int64(offset) >= resp.NumFound {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	log.Printf("collected %d matching issues", len(collectedIssues))

	// ── Step 2: Batch GetIssueTrend ───────────────────────────────────────────
	trendMap := make(map[string]struct{ Crash, User int64 })
	issueIDs := make([]string, 0, len(collectedIssues))
	for _, issue := range collectedIssues {
		issueIDs = append(issueIDs, issue.IssueID)
	}

	chunkSize := 1000 // 官方接口硬上限为 1000
	trendStartStr := startDateObj.Format("2006-01-02") + " 00:00:00"
	trendEndStr := today.Format("2006-01-02") + " 23:59:59"

	for i := 0; i < len(issueIDs); i += chunkSize {
		end := i + chunkSize
		if end > len(issueIDs) {
			end = len(issueIDs)
		}
		chunk := issueIDs[i:end]
		log.Printf("fetching trends for %d issues (%d/%d)", len(chunk), end, len(issueIDs))

		var trends []crashsight.IssueTrendItem
		var err error

		for retries := 0; retries < 3; retries++ {
			trends, err = client.GetIssueTrend(ctx, crashsight.GetIssueTrendParams{
				IssueIDs:        chunk,
				MinDate:         trendStartStr,
				MaxDate:         trendEndStr,
				GranularityUnit: crashsight.GranularityDay,
			})
			if err != nil {
				var rateErr *crashsight.RateLimitError
				if errors.As(err, &rateErr) {
					log.Printf("[GetIssueTrend] rate limited, waiting 15s... (retry %d/3)", retries+1)
					time.Sleep(15 * time.Second)
					continue
				}
				break
			}
			break
		}

		if err != nil {
			logAPIError("GetIssueTrend", err)
			continue
		}

		for _, t := range trends {
			var totalCrash, totalUser int64
			for _, pt := range t.TrendList {
				totalCrash += pt.CrashNum
				totalUser += pt.CrashUser
			}
			trendMap[t.IssueID] = struct{ Crash, User int64 }{totalCrash, totalUser}
		}
		time.Sleep(500 * time.Millisecond) // rate limit, reduce sleep because we have retries
	}

	// ── Step 3: Merge and Format ──────────────────────────────────────────────
	report := ScanReport{
		StartDate: startDate,
		EndDate:   endDate,
		AppID:     appID,
		Platform:  "PC",
		Region: func() string {
			if region == "sg" {
				return "sg"
			}
			return "cn"
		}(),
		Prefixes: []string(prefixes),
	}

	for _, issue := range collectedIssues {
		trend := trendMap[issue.IssueID]
		if trend.Crash == 0 {
			continue // Skip issues with 0 crashes in this exact window
		}

		var processors []string
		for _, p := range issue.AssigneeList {
			if p.Name != "" {
				processors = append(processors, p.Name)
			} else if p.WetestUin != "" {
				processors = append(processors, p.WetestUin)
			}
		}
		if len(processors) == 0 && issue.Processor != "" {
			processors = append(processors, issue.Processor)
		}

		var tags []string
		for _, t := range issue.Tags {
			tags = append(tags, t.TagName)
		}
		for _, t := range issue.TagInfoList {
			tags = append(tags, t.TagName)
		}
		for _, t := range issue.Tag {
			tags = append(tags, t)
		}

		// Prefer full callStack from LastMatchedReport, fall back to KeyStack summary
		rawStack := issue.KeyStack
		if issue.LastMatchedReport != nil && issue.LastMatchedReport.CrashMap.CallStack != "" {
			rawStack = issue.LastMatchedReport.CrashMap.CallStack
		}

		ir := IssueReport{
			IssueID:        issue.IssueID,
			ExceptionName:  issue.ExceptionName,
			ExceptionMsg:   issue.ExceptionMessage,
			RawStack:       rawStack,
			Processors:     processors,
			Status:         issue.Status,
			Tags:           tags,
			UploadTime:     issue.LastestUploadTime,
			TotalCrashNum:  issue.CrashNum,
			TotalDeviceNum: issue.ImeiCount,
			TrendCrashNum:  trend.Crash,
			TrendCrashUser: trend.User,
		}

		// Extract hardware sample
		if issue.LastMatchedReport != nil {
			d := issue.LastMatchedReport.CrashMap
			ir.SampleCrashID = d.CrashID
			ir.GPU = d.GPU
			ir.GPUDriverVersion = d.GpuDriverVersion
			ir.CPU = d.CpuName
			ir.OsVer = d.OsVer
			ir.Country = d.Country
			ir.AppVersion = d.ProductVersion
			ir.ElapsedTimeSec = d.ElapsedTime / 1000

			if d.MemSize != "" {
				if b, err := strconv.ParseInt(d.MemSize, 10, 64); err == nil {
					ir.MemoryMB = b / 1024 / 1024
				}
			}
			if d.FreeMem != "" {
				if b, err := strconv.ParseInt(d.FreeMem, 10, 64); err == nil {
					ir.FreeMemoryMB = b / 1024 / 1024
				}
			}
			if d.ReservedMap != nil {
				if v, ok := d.ReservedMap["GPU_DEDICATED_VIDEO_0"]; ok {
					if b, err := strconv.ParseInt(v, 10, 64); err == nil {
						ir.VRAMTotalMB = b / 1024 / 1024
					}
				}
				if v, ok := d.ReservedMap["PROCESS_DEDICATED_ON_GPU_0"]; ok {
					if b, err := strconv.ParseInt(v, 10, 64); err == nil {
						ir.VRAMUsedMB = b / 1024 / 1024
					}
				}
			}
		}

		report.Issues = append(report.Issues, ir)
		report.TotalCrash += ir.TrendCrashNum
	}

	// Sort by crash volume descending
	sort.Slice(report.Issues, func(i, j int) bool {
		return report.Issues[i].TrendCrashNum > report.Issues[j].TrendCrashNum
	})
	report.TotalIssue = len(report.Issues)

	log.Printf("scan complete: %d issues, %d crashes (after filtering)", report.TotalIssue, report.TotalCrash)

	// ── Step 4: Output ────────────────────────────────────────────────────────
	if *csvFile != "" {
		if err := writeCSV(*csvFile, report); err != nil {
			log.Printf("failed to write CSV: %v", err)
		} else {
			log.Printf("CSV written to: %s", *csvFile)
		}
	}

	if *htmlFile != "" {
		if err := writeHTML(*htmlFile, report); err != nil {
			log.Printf("failed to write HTML: %v", err)
		} else {
			log.Printf("HTML report written to: %s", *htmlFile)
		}
	}

	enc := json.NewEncoder(os.Stdout)
	if *outFile != "" {
		f, err := os.Create(*outFile)
		if err != nil {
			log.Fatalf("failed to create output file: %v", err)
		}
		defer f.Close()
		enc = json.NewEncoder(f)
		log.Printf("report written to: %s", *outFile)
	}
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		log.Fatalf("failed to encode JSON: %v", err)
	}
}

func writeCSV(path string, report ScanReport) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	header := []string{
		"IssueID", "UploadTime", "Tags", "ExceptionName", "ExceptionMsg", "Status", "Processors",
		"TrendCrashNum", "TrendCrashUser", "SampleCrashID", "AppVersion",
		"GPU", "GPUDriverVersion", "CPU", "MemoryMB", "FreeMemoryMB",
		"VRAMTotalMB", "VRAMUsedMB", "ElapsedTimeSec", "Country", "OsVer",
	}
	if err := w.Write(header); err != nil {
		return err
	}

	for _, e := range report.Issues {
		row := []string{
			e.IssueID, e.UploadTime, strings.Join(e.Tags, "|"), e.ExceptionName, e.ExceptionMsg, fmt.Sprint(e.Status), strings.Join(e.Processors, "|"),
			fmt.Sprint(e.TrendCrashNum), fmt.Sprint(e.TrendCrashUser), e.SampleCrashID, e.AppVersion,
			e.GPU, e.GPUDriverVersion, e.CPU, fmt.Sprint(e.MemoryMB), fmt.Sprint(e.FreeMemoryMB),
			fmt.Sprint(e.VRAMTotalMB), fmt.Sprint(e.VRAMUsedMB), fmt.Sprint(e.ElapsedTimeSec), e.Country, e.OsVer,
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	return nil
}

func writeHTML(path string, report ScanReport) error {
	const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>CrashSight Daily Scan Report</title>
    <script src="https://cdn.jsdelivr.net/npm/echarts@5.5.0/dist/echarts.min.js"></script>
    <script src="https://unpkg.com/vue@3/dist/vue.global.prod.js"></script>
    <style>
        * { box-sizing: border-box; }
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; padding: 16px; background: #f5f7f9; margin: 0; }
        .container { width: 100%%; }
        .card { background: white; border-radius: 8px; padding: 20px; margin-bottom: 16px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .chart { width: 100%%; height: 400px; }
        h1, h2 { color: #333; margin-top: 0; }
        .stats { display: flex; gap: 16px; margin-bottom: 16px; }
        .stat-box { background: white; padding: 16px 20px; border-radius: 8px; flex: 1; text-align: center; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .stat-num { font-size: 28px; font-weight: bold; color: #1890ff; margin-top: 8px; }

        .table-container { overflow-x: auto; margin-top: 12px; }
        table { width: 100%%; border-collapse: collapse; background: white; table-layout: fixed; }
        col.col-id       { width: 96px; }
        col.col-status   { width: 88px; }
        col.col-assignee { width: 110px; }
        col.col-crashes  { width: 72px; }
        col.col-total    { width: 110px; }
        col.col-version  { width: 160px; }
        col.col-exception{ width: 260px; }
        col.col-gpu      { width: 200px; }
        col.col-driver   { width: 120px; }
        col.col-ram      { width: 72px; }
        col.col-survival { width: 80px; }
        col.col-tags     { width: 120px; }
        th, td { padding: 9px 10px; text-align: left; border-bottom: 1px solid #eee; font-size: 13px; overflow: hidden; }
        th { background: #fafafa; font-weight: 600; cursor: pointer; user-select: none; white-space: nowrap; }
        th:hover { background: #f0f0f0; }
        tr:hover > td { background: #f0f7ff; }
        .cell-clip { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; display: block; }
        .badge { padding: 2px 7px; border-radius: 10px; font-size: 11px; font-weight: 500; display: inline-block; margin: 1px; white-space: nowrap; }
        .badge-danger { background: #ffe5e5; color: #d93025; }
        .badge-success { background: #e6f4ea; color: #1e8e3e; }
        .badge-warning { background: #fef7e0; color: #f29900; }
        .badge-secondary { background: #f1f3f4; color: #5f6368; }
        .stack-trace { background: #1e1e1e; color: #d4d4d4; padding: 14px 16px; border-radius: 6px; font-family: "Cascadia Code", "Fira Code", "Consolas", monospace; white-space: pre; font-size: 12px; line-height: 1.6; max-height: 500px; overflow: auto; margin-top: 8px; word-break: break-all; }
        .search-box { padding: 7px 11px; width: 300px; border: 1px solid #ccc; border-radius: 4px; font-size: 13px; }
        .filter-bar { display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; flex-wrap: wrap; gap: 8px; }
        .unassigned-warning { color: #d93025; font-weight: 600; font-size: 12px; }
        a.issue-link { color: #1890ff; text-decoration: none; font-family: monospace; font-size: 12px; }
        a.issue-link:hover { text-decoration: underline; }
        button.btn { padding: 4px 10px; border: 1px solid #d9d9d9; border-radius: 4px; background: #fff; cursor: pointer; font-size: 12px; white-space: nowrap; }
        button.btn:hover { background: #e8f4ff; border-color: #1890ff; color: #1890ff; }
        .exc-name { font-weight: 600; color: #222; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; display: block; }
        .exc-msg  { font-size: 11px; color: #666; margin-top: 2px; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
        .exc-cell { cursor: pointer; }
        .exc-cell:hover .exc-name { color: #1890ff; }
        /* Stack Modal */
        #stack-modal-overlay {
            display: none;
            position: fixed; inset: 0;
            background: rgba(0,0,0,0.45);
            z-index: 9998;
        }
        #stack-modal {
            display: none;
            position: fixed;
            z-index: 9999;
            top: 50%%;
            left: 50%%;
            transform: translate(-50%%, -50%%);
            width: 90vw;
            max-height: 80vh;
            background: #fff;
            border-radius: 8px;
            box-shadow: 0 8px 40px rgba(0,0,0,0.25);
            display: none;
            flex-direction: column;
        }
        #stack-modal-header {
            display: flex;
            align-items: center;
            justify-content: space-between;
            padding: 12px 18px;
            border-bottom: 1px solid #eee;
            flex-shrink: 0;
        }
        #stack-modal-title { font-size: 13px; font-weight: 600; color: #333; font-family: "Cascadia Code", "Fira Code", "Consolas", monospace; }
        #stack-modal-close {
            cursor: pointer; font-size: 20px; color: #999; line-height: 1;
            border: none; background: none; padding: 0 4px;
        }
        #stack-modal-close:hover { color: #333; }
        #stack-modal-body {
            overflow-y: auto;
            flex: 1;
            overscroll-behavior: contain;
        }
        #stack-modal-body table {
            width: 100%%;
            border-collapse: collapse;
            font-family: "Cascadia Code", "Fira Code", "Consolas", monospace;
            font-size: 13px;
            line-height: 1.7;
            table-layout: auto;
        }
        #stack-modal-body tr:nth-child(even) { background: #f9f9f9; }
        #stack-modal-body tr:hover { background: #eef5ff; }
        #stack-modal-body td { padding: 3px 12px; vertical-align: top; white-space: nowrap; border-bottom: 1px solid #f0f0f0; font-family: "Cascadia Code", "Fira Code", "Consolas", monospace; }
        #stack-modal-body td.st-idx  { color: #bbb; width: 1%%; text-align: right; user-select: none; }
        #stack-modal-body td.st-dll  { color: #1d6fa4; font-weight: 600; width: 1%%; }
        #stack-modal-body td.st-pc   { color: #888; width: 1%%; }
        #stack-modal-body td.st-src  { color: #2d7a2d; white-space: pre-wrap; word-break: break-all; }
    </style>
</head>
<body>
    <div id="stack-modal-overlay" onclick="closeStackModal()"></div>
    <div id="stack-modal">
        <div id="stack-modal-header">
            <span id="stack-modal-title"></span>
            <button id="stack-modal-close" onclick="closeStackModal()">&#x2715;</button>
        </div>
        <div id="stack-modal-body"></div>
    </div>
    <div id="app" class="container">
        <h1>CrashSight Analysis Report</h1>
        <div class="stats">
            <div class="stat-box"><div>Active Issues</div><div class="stat-num">%d</div></div>
            <div class="stat-box"><div>Total Crashes (Window)</div><div class="stat-num">%d</div></div>
            <div class="stat-box"><div>Date Range</div><div style="margin-top: 10px; font-size: 20px;">%s to %s</div></div>
        </div>

        <div class="card">
            <h2>Crash Over Time (Scatter Plot)</h2>
            <div id="scatterChart" class="chart"></div>
        </div>

        <div style="display:grid; grid-template-columns:1fr 1fr; gap:16px; margin-bottom:16px;">
            <div class="card" style="margin-bottom:0;">
                <h2>GPU Distribution</h2>
                <div id="gpuChart" class="chart"></div>
            </div>
            <div class="card" style="margin-bottom:0;">
                <h2>Memory (RAM) Distribution</h2>
                <div id="memChart" class="chart"></div>
            </div>
            <div class="card" style="margin-bottom:0;">
                <h2>GPU Driver Version Distribution</h2>
                <div id="driverChart" class="chart"></div>
            </div>
            <div class="card" style="margin-bottom:0;">
                <h2>App Version Distribution</h2>
                <div id="verChart" class="chart"></div>
            </div>
        </div>

        <div class="card">
            <div class="filter-bar">
                <input type="text" v-model="searchQuery" class="search-box" placeholder="Search Exception, Stack, GPU, AppVersion...">
                <label style="cursor: pointer; display: flex; align-items: center; gap: 8px;">
                    <input type="checkbox" v-model="showUnassignedOnly"> 
                    <span style="font-weight: 500;">Show Unassigned Only</span>
                </label>
            </div>
            
            <div class="table-container">
                <table>
                    <colgroup>
                        <col class="col-id">
                        <col class="col-status">
                        <col class="col-assignee">
                        <col class="col-crashes">
                        <col class="col-total">
                        <col class="col-version">
                        <col class="col-exception">
                        <col class="col-gpu">
                        <col class="col-driver">
                        <col class="col-ram">
                        <col class="col-survival">
                        <col class="col-tags">
                    </colgroup>
                    <thead>
                        <tr>
                            <th>Issue ID</th>
                            <th>Status</th>
                            <th>Assignee</th>
                            <th @click="sortBy('trendCrashNum')" title="Window crashes (trend)">+Crashes{{ sortKey==='trendCrashNum' ? (sortDesc?' ↓':' ↑') : ' ↕' }}</th>
                            <th @click="sortBy('totalCrashNum')" title="All-time crashes / affected devices">Total C/D{{ sortKey==='totalCrashNum' ? (sortDesc?' ↓':' ↑') : ' ↕' }}</th>
                            <th @click="sortBy('appVersion')" title="Sort by version">Version{{ sortKey==='appVersion' ? (sortDesc?' ↓':' ↑') : ' ↕' }}</th>
                            <th>Exception &amp; Message</th>
                            <th @click="sortBy('gpu')" title="Sort by GPU">GPU{{ sortKey==='gpu' ? (sortDesc?' ↓':' ↑') : ' ↕' }}</th>
                            <th>Driver</th>
                            <th @click="sortBy('memoryMB')" title="Sort by RAM">RAM(G){{ sortKey==='memoryMB' ? (sortDesc?' ↓':' ↑') : ' ↕' }}</th>
                            <th @click="sortBy('elapsedTimeSec')" title="Sort by survival time">Survival{{ sortKey==='elapsedTimeSec' ? (sortDesc?' ↓':' ↑') : ' ↕' }}</th>
                            <th>Tags</th>
                        </tr>
                    </thead>
                    <tbody>
                        <template v-for="item in filteredAndSortedData" :key="item.issueId">
                            <tr>
                                <td><a :href="getIssueUrl(item)" target="_blank" class="issue-link" :title="item.issueId">{{ item.issueId.substring(0, 8) }}…</a></td>
                                <td><span :class="getStatusClass(item.status)">{{ getStatusText(item.status) }}</span></td>
                                <td>
                                    <span v-if="item.processors && item.processors.length" class="cell-clip" :title="item.processors.join(', ')">{{ item.processors.join(', ') }}</span>
                                    <span v-else class="unassigned-warning">⚠ None</span>
                                </td>
                                <td style="font-weight:700; color:#ff4d4f; text-align:right;">{{ item.trendCrashNum }}</td>
                                <td style="text-align:right; font-size:12px; color:#555;">
                                    <span style="color:#d4380d;">{{ item.totalCrashNum }}</span>
                                    <span style="color:#999;"> / </span>
                                    <span style="color:#096dd9;">{{ item.totalDeviceNum }}</span>
                                </td>
                                <td><span class="cell-clip" :title="item.appVersion">{{ item.appVersion }}</span></td>
                                <td class="exc-cell" @click="openStack(item)">
                                    <span class="exc-name">{{ item.exceptionName }}</span>
                                    <span class="exc-msg">{{ item.exceptionMessage }}</span>
                                </td>
                                <td><span class="cell-clip" :title="item.gpu">{{ item.gpu }}</span></td>
                                <td><span class="cell-clip" :title="item.gpuDriverVersion">{{ item.gpuDriverVersion }}</span></td>
                                <td style="text-align:right;">{{ fmtGB(item.memoryMB) }}</td>
                                <td style="text-align:right;">{{ fmtDuration(item.elapsedTimeSec) }}</td>
                                <td><span v-for="tag in item.tags" class="badge badge-secondary">{{ tag }}</span></td>
                            </tr>
                        </template>
                    </tbody>
                </table>
                <div v-if="filteredAndSortedData.length === 0" style="text-align:center; padding:30px; color:#999;">
                    No matching issues found.
                </div>
            </div>
        </div>
    </div>

    <script>
        const rawBase64 = "%s";
        // Safely decode UTF-8 Base64 string
        const binaryString = atob(rawBase64);
        const bytes = new Uint8Array(binaryString.length);
        for (let i = 0; i < binaryString.length; i++) {
            bytes[i] = binaryString.charCodeAt(i);
        }
        const jsonString = new TextDecoder("utf-8").decode(bytes);
        const rawData = JSON.parse(jsonString);
        const region = "%s";
        const appId = "%s";
        
        function makeBarOption(title, keys, vals, color) {
            return {
                tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
                grid: { left: '2%%', right: '6%%', top: '4%%', bottom: '4%%', containLabel: true },
                xAxis: { type: 'value', minInterval: 1 },
                yAxis: { type: 'category', data: keys, axisLabel: { width: 160, overflow: 'truncate', fontSize: 11 } },
                series: [{ type: 'bar', data: vals, itemStyle: { color: color },
                    label: { show: true, position: 'right', fontSize: 11 } }]
            };
        }

        function sortedTopN(counts, n) {
            return Object.entries(counts)
                .sort((a, b) => b[1] - a[1])
                .slice(0, n);
        }

        function initCharts(data) {
            const gpuCounts = {}, memCounts = {}, driverCounts = {}, verCounts = {};
            const scatterData = [];

            data.forEach(item => {
                const vol = item.trendCrashNum || 0;
                if (vol === 0) return;

                // Scatter
                if (item.uploadTime) {
                    scatterData.push([item.uploadTime, vol, item.issueId, item.elapsedTimeSec, item.appVersion]);
                }

                // GPU
                let gpu = (item.gpu || 'Unknown').trim() || 'Unknown';
                gpuCounts[gpu] = (gpuCounts[gpu] || 0) + vol;

                // RAM — exact value rounded to nearest GB
                const gb = (item.memoryMB || 0) / 1024;
                if (gb > 0) {
                    const label = Math.round(gb) + 'G';
                    memCounts[label] = (memCounts[label] || 0) + vol;
                }

                // GPU Driver
                let drv = (item.gpuDriverVersion || 'Unknown').trim() || 'Unknown';
                driverCounts[drv] = (driverCounts[drv] || 0) + vol;

                // App Version — keep last 2 dot segments (date + build) for brevity
                let ver = item.appVersion || 'Unknown';
                const parts = ver.split('.');
                if (parts.length >= 4) ver = parts.slice(-2).join('.');
                verCounts[ver] = (verCounts[ver] || 0) + vol;
            });

            // 1. Scatter
            const scatterChart = echarts.init(document.getElementById('scatterChart'));
            scatterChart.setOption({
                tooltip: {
                    trigger: 'item',
                    formatter: function(p) {
                        const d = p.data;
                        return '<b>' + d[2].substring(0,8) + '…</b><br/>' +
                            'Time: ' + d[0] + '<br/>Crashes: ' + d[1] +
                            '<br/>Version: ' + d[4] + '<br/>Survival: ' + d[3] + 's';
                    }
                },
                grid: { left: '4%%', right: '4%%', bottom: '10%%', top: '8%%', containLabel: true },
                xAxis: { type: 'time', name: 'Upload Time' },
                yAxis: { type: 'value', name: 'Crashes' },
                series: [{
                    type: 'scatter',
                    symbolSize: function(d) { return Math.min(Math.max(d[1] * 2, 8), 40); },
                    itemStyle: { color: 'rgba(24,144,255,0.6)', borderColor: '#1890ff', borderWidth: 1 },
                    cursor: 'pointer',
                    data: scatterData
                }]
            });
            scatterChart.on('click', function(params) {
                const issueId = params.data[2];
                const domain = region === 'sg' ? 'crashsight.wetest.net' : 'crashsight.qq.com';
                window.open('https://' + domain + '/crash-reporting/crashes/' + appId + '/' + issueId + '?pid=10', '_blank');
            });

            // 2. GPU
            const gpuTop = sortedTopN(gpuCounts, 12);
            const gpuChart = echarts.init(document.getElementById('gpuChart'));
            gpuChart.setOption(makeBarOption('GPU', gpuTop.map(e=>e[0]).reverse(), gpuTop.map(e=>e[1]).reverse(), '#5470c6'));

            // 3. Memory
            // Sort memory labels numerically (8G, 16G, 32G, ...)
            const memSorted = sortedTopN(memCounts, 999)
                .sort((a, b) => parseInt(a[0]) - parseInt(b[0]));
            const memChart = echarts.init(document.getElementById('memChart'));
            memChart.setOption(makeBarOption('RAM', memSorted.map(e=>e[0]), memSorted.map(e=>e[1]), '#91cc75'));

            // 4. Driver
            const drvTop = sortedTopN(driverCounts, 12);
            const driverChart = echarts.init(document.getElementById('driverChart'));
            driverChart.setOption(makeBarOption('Driver', drvTop.map(e=>e[0]).reverse(), drvTop.map(e=>e[1]).reverse(), '#fac858'));

            // 5. App Version — primary sort: crash count desc; tie-break: version string desc
            const verTop = Object.entries(verCounts)
                .sort((a, b) => b[1] - a[1] || b[0].localeCompare(a[0]))
                .slice(0, 12);
            const verChart = echarts.init(document.getElementById('verChart'));
            verChart.setOption(makeBarOption('Version', verTop.map(e=>e[0]).reverse(), verTop.map(e=>e[1]).reverse(), '#ee6666'));

            window.addEventListener('resize', () => {
                scatterChart.resize();
                gpuChart.resize();
                memChart.resize();
                driverChart.resize();
                verChart.resize();
            });
        }

        const { createApp } = Vue;
        createApp({
            data() {
                return {
                    issues: rawData,
                    searchQuery: '',
                    showUnassignedOnly: false,
                    sortKey: 'trendCrashNum',
                    sortDesc: true,
                    expandedRows: []
                };
            },
            computed: {
                filteredAndSortedData() {
                    let result = this.issues;
                    if (this.showUnassignedOnly) {
                        result = result.filter(item => !item.processors || item.processors.length === 0);
                    }
                    if (this.searchQuery) {
                        const q = this.searchQuery.toLowerCase();
                        result = result.filter(item => {
                            return (item.exceptionName && item.exceptionName.toLowerCase().includes(q)) ||
                                   (item.exceptionMessage && item.exceptionMessage.toLowerCase().includes(q)) ||
                                   (item.gpu && item.gpu.toLowerCase().includes(q)) ||
                                   (item.rawStack && item.rawStack.toLowerCase().includes(q)) ||
                                   (item.appVersion && item.appVersion.toLowerCase().includes(q));
                        });
                    }
                    result.sort((a, b) => {
                        let valA = a[this.sortKey];
                        let valB = b[this.sortKey];
                        if (typeof valA === 'string') valA = valA.toLowerCase();
                        if (typeof valB === 'string') valB = valB.toLowerCase();
                        if (valA < valB) return this.sortDesc ? 1 : -1;
                        if (valA > valB) return this.sortDesc ? -1 : 1;
                        return 0;
                    });
                    return result;
                }
            },
            methods: {
                sortBy(key) {
                    if (this.sortKey === key) {
                        this.sortDesc = !this.sortDesc;
                    } else {
                        this.sortKey = key;
                        this.sortDesc = true;
                    }
                },
                toggleExpand(id) {
                    const idx = this.expandedRows.indexOf(id);
                    if (idx > -1) {
                        this.expandedRows.splice(idx, 1);
                    } else {
                        this.expandedRows.push(id);
                    }
                },
                getStatusClass(status) {
                    switch(status) {
                        case 0: return 'badge badge-danger';
                        case 1: return 'badge badge-success';
                        case 2: return 'badge badge-warning';
                        default: return 'badge badge-secondary';
                    }
                },
                getStatusText(status) {
                    switch(status) {
                        case 0: return 'Unresolved';
                        case 1: return 'Resolved';
                        case 2: return 'Resolving';
                        default: return 'Unknown';
                    }
                },
                fmtGB(mb) {
                    if (!mb) return '-';
                    return (mb / 1024).toFixed(1) + ' G';
                },
                fmtDuration(sec) {
                    if (!sec) return '-';
                    const m = Math.floor(sec / 60);
                    const s = sec %% 60;
                    return m + ':' + String(s).padStart(2, '0');
                },
                getIssueUrl(item) {
                    const domain = region === 'sg' ? 'crashsight.wetest.net' : 'crashsight.qq.com';
                    return 'https://' + domain + '/crash-reporting/crashes/' + appId + '/' + item.issueId + '?pid=10';
                },
                openStack(item) {
                    const title = item.exceptionName + (item.exceptionMessage ? '  —  ' + item.exceptionMessage : '');
                    document.getElementById('stack-modal-title').textContent = title;
                    document.getElementById('stack-modal-body').innerHTML = this._renderStack(item.rawStack || '(no stack)');
                    document.getElementById('stack-modal-overlay').style.display = 'block';
                    document.getElementById('stack-modal').style.display = 'flex';
                    document.getElementById('stack-modal-body').scrollTop = 0;
                    document.body.style.overflow = 'hidden';
                },
                _renderStack(stack) {
                    const esc = s => s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
                    let idx = 0;
                    const rows = stack.split('\n').map(line => {
                        const parts = line.split('\t');
                        const dll = esc((parts[1] || '').trim());
                        const pc  = esc((parts[2] || '').trim());
                        const src = esc(parts.slice(3).join('\t').trim());
                        if (!dll && !pc && !src) return '';
                        return '<tr>' +
                            '<td class="st-idx">' + (idx++) + '</td>' +
                            '<td class="st-dll">' + dll + '</td>' +
                            '<td class="st-pc">'  + pc  + '</td>' +
                            '<td class="st-src">' + src + '</td>' +
                        '</tr>';
                    }).filter(r => r).join('');
                    return '<table>' + rows + '</table>';
                }
            },
            mounted() {
                this.$nextTick(() => {
                    initCharts(this.issues);
                });
            }
        }).mount('#app');

        function closeStackModal() {
            document.getElementById('stack-modal-overlay').style.display = 'none';
            document.getElementById('stack-modal').style.display = 'none';
            document.body.style.overflow = '';
        }
        document.addEventListener('keydown', e => { if (e.key === 'Escape') closeStackModal(); });
    </script>
</body>
</html>`

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	jsonBytes, err := json.Marshal(report.Issues)
	if err != nil {
		return err
	}

	b64Json := base64.StdEncoding.EncodeToString(jsonBytes)

	htmlContent := fmt.Sprintf(htmlTemplate, report.TotalIssue, report.TotalCrash, report.StartDate, report.EndDate, b64Json, report.Region, report.AppID)
	_, err = f.WriteString(htmlContent)
	return err
}

func logAPIError(method string, err error) {
	var apiErr *crashsight.APIError
	var authErr *crashsight.AuthError
	var rateErr *crashsight.RateLimitError
	switch {
	case errors.As(err, &apiErr):
		log.Printf("[%s] API error: %s (code=%d)", method, apiErr.Message, apiErr.StatusCode)
	case errors.As(err, &authErr):
		log.Fatalf("[%s] auth failed: %s", method, authErr.Message)
	case errors.As(err, &rateErr):
		log.Printf("[%s] rate limited, waiting 15s...", method)
		time.Sleep(15 * time.Second)
	default:
		log.Printf("[%s] error: %v", method, err)
	}
}
