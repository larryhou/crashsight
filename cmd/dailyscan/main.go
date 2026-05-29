// dailyscan scans all crash issues within a time window and outputs full device
// info for each crash (GPU/CPU/Memory/Driver).
//
// crashDatas from GetCrashList already contains gpu/gpuDriverVersion/cpuName/memSize,
// so GetCrashDoc is not needed — one request per issue suffices.
//
// Run (default: today only, filter Physical.RealisticMP / Cloud.RealisticMP):
//
//	go run ./cmd/dailyscan -out report.json
//
// -days N covers a window from N days ago to today (N+1 days total):
//
//	go run ./cmd/dailyscan -days 2 -out report.json
//
// Custom version prefixes:
//
//	go run ./cmd/dailyscan -version-prefix Physical.Ma3 -version-prefix Cloud.Ma3
//
// Debug (scan only the first N issues):
//
//	go run ./cmd/dailyscan -max-issues 5
package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	crashsight "github.com/larryhou/crashsight"
)

// versionPrefixes supports multiple -version-prefix flags.
type versionPrefixes []string

func (v *versionPrefixes) String() string     { return strings.Join(*v, ",") }
func (v *versionPrefixes) Set(s string) error { *v = append(*v, s); return nil }

// matchesPrefix reports whether version matches any of the given prefixes.
// An empty prefix list means no filtering (all versions pass).
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

// CrashEntry holds device details for a single crash.
type CrashEntry struct {
	CrashID          string `json:"crashId"`
	UploadTime       string `json:"uploadTime"`
	CrashTime        string `json:"crashTime"`        // 新增：崩溃发生时间
	ElapsedTimeSec   int64  `json:"elapsedTimeSec"`   // 新增：运行存活时长（秒）
	AppVersion       string `json:"appVersion"`
	GPU              string `json:"gpu"`
	GPUDriverVersion string `json:"gpuDriverVersion"`
	CPU              string `json:"cpu"`
	MemoryMB         int64  `json:"memoryMB"`
	FreeMemoryMB     int64  `json:"freeMemoryMB"`     // 新增：剩余内存(MB)
	VRAMTotalMB      int64  `json:"vramTotalMB"`      // 新增：总显存(MB)
	VRAMUsedMB       int64  `json:"vramUsedMB"`       // 新增：占用显存(MB)
	Country          string `json:"country"`          // 新增：国家/地区
	DeviceID         string `json:"deviceId"`
	UserID           string `json:"userId"`
	OsVer            string `json:"osVer"`
}

// IssueReport holds a single issue and its filtered crash list.
type IssueReport struct {
	IssueID       string       `json:"issueId"`
	ExceptionName string       `json:"exceptionName"`
	ExceptionMsg  string       `json:"exceptionMessage,omitempty"`
	RawStack      string       `json:"rawStack,omitempty"`
	Processors    []string     `json:"processors,omitempty"`
	Status        int          `json:"status"`
	CrashNum      int64        `json:"crashNum"`
	CrashUser     int64        `json:"crashUser"`
	Crashes       []CrashEntry `json:"crashes"`
}

// ScanReport is the top-level scan result.
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
	daysFlag := flag.Int("days", 0, "time window in days: 0=today only, N=N days ago to today (N+1 days total)")
	maxIssues := flag.Int("max-issues", 50, "max issues to scan, 0=all")
	outFile := flag.String("out", "", "output file (JSON); defaults to stdout")
	htmlFile := flag.String("html", "", "generate a standalone HTML visualization report")
	flatJSONFile := flag.String("flat-json", "", "output flat JSON for data analysis tools")
	csvFile := flag.String("csv", "", "output flat CSV for data analysis tools")
	rowsPerIssue := flag.Int("rows", 0, "max crashes to fetch per issue, 0=all pages")
	var prefixes versionPrefixes
	flag.Var(&prefixes, "version-prefix", "version prefix filter, repeatable (default: Physical.RealisticMP Cloud.RealisticMP)")
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

	client := crashsight.NewClient(userID, apiKey,
		crashsight.WithRegion(r),
		crashsight.WithTimeout(60*time.Second),
	)
	ctx := context.Background()

	today := time.Now()
	endDate := today.Format("20060102")
	startDate := today.AddDate(0, 0, -*daysFlag).Format("20060102")

	// Build date set for crash filtering (format YYYY-MM-DD).
	dateSet := make(map[string]bool)
	for i := 0; i <= *daysFlag; i++ {
		d := today.AddDate(0, 0, -i)
		dateSet[d.Format("2006-01-02")] = true
	}

	log.Printf("scan start: appId=%s startDate=%s endDate=%s versionPrefixes=%v", appID, startDate, endDate, []string(prefixes))

	// ── Step 1: fetch latest issues for the time window (1 request) ──────────
	millis := int64((*daysFlag + 1) * 24 * 3600 * 1000)
	issues, err := fetchLatestIssues(ctx, client, appID, millis, *maxIssues)
	if err != nil {
		log.Fatalf("failed to fetch issue list: %v", err)
	}
	log.Printf("fetched %d issues", len(issues))

	report := ScanReport{
		StartDate: startDate,
		EndDate:   endDate,
		AppID:     appID,
		Platform:  "PC",
		Prefixes:  []string(prefixes),
	}

	// ── Step 2: GetCrashList per issue ────────────────────────────────────────
	// crashDatas already contains gpu/gpuDriverVersion/cpuName/memSize; no need for GetCrashDoc.
	for i, issue := range issues {
		log.Printf("[%d/%d] issueId=%s crashNum=%d %s",
			i+1, len(issues), issue.IssueID, issue.CrashNum, issue.ExceptionName)

		crashes, err := fetchCrashesForIssue(ctx, client, appID, issue.IssueID, *rowsPerIssue, []string(prefixes), dateSet, startDate)
		if err != nil {
			logAPIError(fmt.Sprintf("GetCrashList issueId=%s", issue.IssueID), err)
		}

		// Skip issues with no matching crashes after filtering.
		if len(crashes) == 0 {
			log.Printf("  skipped (no crashes after filtering)")
			time.Sleep(2500 * time.Millisecond)
			continue
		}

		// Recalculate crashNum/crashUser from filtered results (crashUser deduped by deviceId).
		uniqueDevices := make(map[string]struct{})
		for _, c := range crashes {
			if c.DeviceID != "" {
				uniqueDevices[c.DeviceID] = struct{}{}
			}
		}

		// Parse processors (semicolon-separated ID string, drop empty entries).
		var processors []string
		for _, p := range strings.Split(issue.Processors, ";") {
			if p = strings.TrimSpace(p); p != "" {
				processors = append(processors, p)
			}
		}

		entry := IssueReport{
			IssueID:       issue.IssueID,
			ExceptionName: issue.ExceptionName,
			ExceptionMsg:  issue.ExceptionMessage,
			RawStack:      issue.KeyStack,
			Processors:    processors,
			Status:        issue.Status,
			Crashes:       crashes,
		}
		report.Issues = append(report.Issues, entry)
		report.TotalCrash += int64(len(crashes))

		// Rate limit: 25 req/min — pause between requests.
		time.Sleep(2500 * time.Millisecond)
	}

	report.TotalIssue = len(report.Issues)
	log.Printf("scan complete: %d issues, %d crashes (after filtering)", report.TotalIssue, report.TotalCrash)

	// ── Step 3: output ────────────────────────────────────────────────────────
	if *flatJSONFile != "" {
		if err := writeFlatJSON(*flatJSONFile, report); err != nil {
			log.Printf("failed to write flat JSON: %v", err)
		} else {
			log.Printf("flat JSON written to: %s", *flatJSONFile)
		}
	}

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

	// Always output the structured JSON report to stdout or outFile
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

// fetchLatestIssues fetches the latest issues updated within the time window.
// It uses GetIssueList to get results sorted by uploadTime descending,
// matching the behavior of the CrashSight web console.
func fetchLatestIssues(ctx context.Context, client *crashsight.Client, appID string, millis int64, maxIssues int) ([]crashsight.IssueItem, error) {
	limit := maxIssues
	if limit <= 0 {
		limit = 100 // default reasonable limit if not specified
	}
	resp, err := client.GetIssueList(ctx, appID, crashsight.PlatformPC, crashsight.GetIssueListParams{
		ExceptionTypeList:             crashsight.ExceptionTypeCrash,
		Status:                        "0,2", // 0=Unprocessed, 2=Processing
		Rows:                          limit,
		SortField:                     "uploadTime",
		SortOrder:                     "desc",
		IssueUploadTimeRelativeMillis: millis,
	})
	if err != nil {
		return nil, err
	}
	return resp.IssueList, nil
}

// fetchCrashesForIssue pages through all crashes for an issue (100 per page).
// crashDatas already contains gpu/gpuDriverVersion/cpuName/memSize; no need for GetCrashDoc.
// dateSet is the set of allowed dates (format YYYY-MM-DD).
// minDate (YYYYMMDD) is the earliest allowed date; since crashList is returned in reverse
// chronological order, we can stop early when the last entry on a page predates minDate.
func fetchCrashesForIssue(ctx context.Context, client *crashsight.Client, appID, issueID string, maxRows int, prefixes []string, dateSet map[string]bool, minDate string) ([]CrashEntry, error) {
	// Convert minDate YYYYMMDD → YYYY-MM-DD for comparison with uploadTime.
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
			log.Printf("  numFound=%d, estimated %d page(s)", numFound, (numFound+pageSize-1)/pageSize)
		}

		matched := 0
		outOfDate := 0
		for _, crashID := range resp.CrashIDList {
			d := resp.CrashDatas[crashID]

			// Date filter: uploadTime format "2026-05-28 18:36:11", use first 10 chars as key.
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

			var memMB, freeMemMB, vramTotal, vramUsed int64
			if d.MemSize != "" {
				var b int64
				fmt.Sscanf(d.MemSize, "%d", &b)
				memMB = b / 1024 / 1024
			}
			if d.FreeMem != "" {
				var b int64
				fmt.Sscanf(d.FreeMem, "%d", &b)
				freeMemMB = b / 1024 / 1024
			}
			if d.ReservedMap != nil {
				if v, ok := d.ReservedMap["GPU_DEDICATED_VIDEO_0"]; ok {
					if b, err := strconv.ParseInt(v, 10, 64); err == nil {
						vramTotal = b / 1024 / 1024
					}
				}
				if v, ok := d.ReservedMap["PROCESS_DEDICATED_ON_GPU_0"]; ok {
					if b, err := strconv.ParseInt(v, 10, 64); err == nil {
						vramUsed = b / 1024 / 1024
					}
				}
			}

			entries = append(entries, CrashEntry{
				CrashID:          crashID,
				UploadTime:       d.UploadTime,
				CrashTime:        d.CrashTime,
				ElapsedTimeSec:   d.ElapsedTime / 1000,
				AppVersion:       d.ProductVersion,
				GPU:              d.GPU,
				GPUDriverVersion: d.GpuDriverVersion,
				CPU:              d.CpuName,
				MemoryMB:         memMB,
				FreeMemoryMB:     freeMemMB,
				VRAMTotalMB:      vramTotal,
				VRAMUsedMB:       vramUsed,
				Country:          d.Country,
				DeviceID:         d.DeviceID,
				UserID:           d.UserID,
				OsVer:            d.OsVer,
			})
		}

		log.Printf("  page=%d start=%d returned=%d matched=%d outOfDate=%d collected=%d",
			page, start, len(resp.CrashIDList), matched, outOfDate, len(entries))

		start += pageSize

		// crashList is sorted by uploadTime descending; stop early if the last entry
		// on this page predates minDateDash.
		if len(resp.CrashIDList) > 0 {
			last := resp.CrashDatas[resp.CrashIDList[len(resp.CrashIDList)-1]]
			var lastDate string
			if len(last.UploadTime) >= 10 {
				lastDate = last.UploadTime[:10]
			}
			if lastDate != "" && lastDate < minDateDash {
				log.Printf("  last entry uploadTime=%s is before %s, stopping early", last.UploadTime, minDateDash)
				break
			}
		}
		if len(resp.CrashIDList) < pageSize || start >= numFound {
			break
		}
		if maxRows > 0 && start >= maxRows {
			log.Printf("  reached -rows=%d limit, stopping", maxRows)
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

type FlatCrashEntry struct {
	IssueID          string `json:"issueId"`
	ExceptionName    string `json:"exceptionName"`
	ExceptionMsg     string `json:"exceptionMessage"` // 新增：异常信息
	RawStack         string `json:"rawStack"`         // 新增：完整堆栈
	Status           int    `json:"status"`
	CrashID          string `json:"crashId"`
	UploadTime       string `json:"uploadTime"`
	CrashTime        string `json:"crashTime"`
	ElapsedTimeSec   int64  `json:"elapsedTimeSec"`
	AppVersion       string `json:"appVersion"`
	GPU              string `json:"gpu"`
	GPUDriverVersion string `json:"gpuDriverVersion"`
	CPU              string `json:"cpu"`
	MemoryMB         int64  `json:"memoryMB"`
	FreeMemoryMB     int64  `json:"freeMemoryMB"`
	VRAMTotalMB      int64  `json:"vramTotalMB"`
	VRAMUsedMB       int64  `json:"vramUsedMB"`
	Country          string `json:"country"`
	DeviceID         string `json:"deviceId"`
	UserID           string `json:"userId"`
	OsVer            string `json:"osVer"`
}

func getFlatEntries(report ScanReport) []FlatCrashEntry {
	var flat []FlatCrashEntry
	for _, issue := range report.Issues {
		for _, crash := range issue.Crashes {
			flat = append(flat, FlatCrashEntry{
				IssueID:          issue.IssueID,
				ExceptionName:    issue.ExceptionName,
				ExceptionMsg:     issue.ExceptionMsg,
				RawStack:         issue.RawStack,
				Status:           issue.Status,
				CrashID:          crash.CrashID,
				UploadTime:       crash.UploadTime,
				CrashTime:        crash.CrashTime,
				ElapsedTimeSec:   crash.ElapsedTimeSec,
				AppVersion:       crash.AppVersion,
				GPU:              crash.GPU,
				GPUDriverVersion: crash.GPUDriverVersion,
				CPU:              crash.CPU,
				MemoryMB:         crash.MemoryMB,
				FreeMemoryMB:     crash.FreeMemoryMB,
				VRAMTotalMB:      crash.VRAMTotalMB,
				VRAMUsedMB:       crash.VRAMUsedMB,
				Country:          crash.Country,
				DeviceID:         crash.DeviceID,
				UserID:           crash.UserID,
				OsVer:            crash.OsVer,
			})
		}
	}
	return flat
}

func writeFlatJSON(path string, report ScanReport) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	flat := getFlatEntries(report)
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(flat)
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
		"IssueID", "ExceptionName", "ExceptionMsg", "RawStack", "Status", "CrashID", "UploadTime", "CrashTime", "ElapsedTimeSec",
		"AppVersion", "GPU", "GPUDriverVersion", "CPU", "MemoryMB", "FreeMemoryMB",
		"VRAMTotalMB", "VRAMUsedMB", "Country", "DeviceID", "UserID", "OsVer",
	}
	if err := w.Write(header); err != nil {
		return err
	}

	flat := getFlatEntries(report)
	for _, e := range flat {
		row := []string{
			e.IssueID, e.ExceptionName, e.ExceptionMsg, e.RawStack, fmt.Sprint(e.Status), e.CrashID, e.UploadTime, e.CrashTime, fmt.Sprint(e.ElapsedTimeSec),
			e.AppVersion, e.GPU, e.GPUDriverVersion, e.CPU, fmt.Sprint(e.MemoryMB), fmt.Sprint(e.FreeMemoryMB),
			fmt.Sprint(e.VRAMTotalMB), fmt.Sprint(e.VRAMUsedMB), e.Country, e.DeviceID, e.UserID, e.OsVer,
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	return nil
}

func writeHTML(path string, report ScanReport) error {
	// A simple HTML using ECharts from a CDN
	const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>CrashSight Daily Scan Report</title>
    <script src="https://cdn.jsdelivr.net/npm/echarts@5.5.0/dist/echarts.min.js"></script>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; padding: 20px; background: #f5f7f9; }
        .container { max-width: 1400px; margin: 0 auto; }
        .card { background: white; border-radius: 8px; padding: 20px; margin-bottom: 20px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .chart { width: 100%; height: 400px; }
        h1, h2 { color: #333; }
        .stats { display: flex; gap: 20px; margin-bottom: 20px; }
        .stat-box { background: white; padding: 20px; border-radius: 8px; flex: 1; text-align: center; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .stat-num { font-size: 32px; font-weight: bold; color: #1890ff; margin-top: 10px; }
    </style>
</head>
<body>
    <div class="container">
        <h1>CrashSight Analysis Report</h1>
        <div class="stats">
            <div class="stat-box"><div>Total Issues</div><div class="stat-num">%d</div></div>
            <div class="stat-box"><div>Total Crashes</div><div class="stat-num">%d</div></div>
            <div class="stat-box"><div>Date Range</div><div style="margin-top: 10px; font-size: 20px;">%s to %s</div></div>
        </div>

        <div class="card">
            <h2>Top GPU Crash Distribution</h2>
            <div id="gpuChart" class="chart"></div>
        </div>

        <div class="card">
            <h2>Crash VRAM Usage Distribution (MB)</h2>
            <div id="vramChart" class="chart"></div>
        </div>

        <div class="card">
            <h2>Crash Elapsed Time (Survival Time in Seconds)</h2>
            <div id="elapsedChart" class="chart"></div>
        </div>
    </div>

    <script>
        const rawData = %s;
        
        // Data processing
        const gpuCounts = {};
        const vramData = [];
        const elapsedData = [];
        
        rawData.forEach(item => {
            // GPU
            let gpu = item.gpu || 'Unknown';
            if (gpu.length > 30) gpu = gpu.substring(0, 30) + '...';
            gpuCounts[gpu] = (gpuCounts[gpu] || 0) + 1;
            
            // VRAM
            if (item.vramUsedMB > 0) {
                vramData.push(item.vramUsedMB);
            }
            
            // Elapsed
            if (item.elapsedTimeSec > 0) {
                elapsedData.push(item.elapsedTimeSec);
            }
        });

        // 1. GPU Chart
        const gpuChart = echarts.init(document.getElementById('gpuChart'));
        const gpuKeys = Object.keys(gpuCounts).sort((a, b) => gpuCounts[b] - gpuCounts[a]).slice(0, 15);
        gpuChart.setOption({
            tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
            grid: { left: '3%%', right: '4%%', bottom: '3%%', containLabel: true },
            xAxis: { type: 'value' },
            yAxis: { type: 'category', data: gpuKeys.reverse() },
            series: [{
                name: 'Crashes',
                type: 'bar',
                data: gpuKeys.map(k => gpuCounts[k]),
                itemStyle: { color: '#5470c6' }
            }]
        });

        // 2. VRAM Scatter/Boxplot-like Distribution
        const vramChart = echarts.init(document.getElementById('vramChart'));
        // Bucket VRAM into segments
        const vramBuckets = {'< 2GB': 0, '2-4GB': 0, '4-8GB': 0, '8-12GB': 0, '> 12GB': 0};
        vramData.forEach(v => {
            if (v < 2000) vramBuckets['< 2GB']++;
            else if (v < 4000) vramBuckets['2-4GB']++;
            else if (v < 8000) vramBuckets['4-8GB']++;
            else if (v < 12000) vramBuckets['8-12GB']++;
            else vramBuckets['> 12GB']++;
        });
        vramChart.setOption({
            tooltip: { trigger: 'item' },
            xAxis: { type: 'category', data: Object.keys(vramBuckets) },
            yAxis: { type: 'value', name: 'Crash Count' },
            series: [{
                data: Object.values(vramBuckets),
                type: 'bar',
                itemStyle: { color: '#91cc75' }
            }]
        });

        // 3. Elapsed Time Distribution
        const elapsedChart = echarts.init(document.getElementById('elapsedChart'));
        const timeBuckets = {'< 1m': 0, '1-5m': 0, '5-15m': 0, '15-30m': 0, '> 30m': 0};
        elapsedData.forEach(t => {
            if (t < 60) timeBuckets['< 1m']++;
            else if (t < 300) timeBuckets['1-5m']++;
            else if (t < 900) timeBuckets['5-15m']++;
            else if (t < 1800) timeBuckets['15-30m']++;
            else timeBuckets['> 30m']++;
        });
        elapsedChart.setOption({
            tooltip: { trigger: 'item' },
            xAxis: { type: 'category', data: Object.keys(timeBuckets) },
            yAxis: { type: 'value', name: 'Crash Count' },
            series: [{
                data: Object.values(timeBuckets),
                type: 'line',
                smooth: true,
                areaStyle: {},
                itemStyle: { color: '#fac858' }
            }]
        });

        // Resize charts on window resize
        window.addEventListener('resize', () => {
            gpuChart.resize();
            vramChart.resize();
            elapsedChart.resize();
        });
    </script>
</body>
</html>`

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	flat := getFlatEntries(report)
	jsonBytes, err := json.Marshal(flat)
	if err != nil {
		return err
	}

	htmlContent := fmt.Sprintf(htmlTemplate, report.TotalIssue, report.TotalCrash, report.StartDate, report.EndDate, string(jsonBytes))
	_, err = f.WriteString(htmlContent)
	return err
}
