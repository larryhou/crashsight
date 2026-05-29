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
	IssueID          string   `json:"issueId"`
	ExceptionName    string   `json:"exceptionName"`
	ExceptionMsg     string   `json:"exceptionMessage"`
	RawStack         string   `json:"rawStack"`
	Processors       []string `json:"processors"`
	Status           int      `json:"status"`

	// Trend Data (Volume)
	TrendCrashNum    int64 `json:"trendCrashNum"`
	TrendCrashUser   int64 `json:"trendCrashUser"`

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
	startDateObj := today.AddDate(0, 0, -*daysFlag)
	startDate := startDateObj.Format("20060102")
	endDate := today.Format("20060102")

	minDateDash := startDateObj.Format("2006-01-02")

	log.Printf("scan start: appId=%s startDate=%s endDate=%s prefixes=%v", appID, startDate, endDate, []string(prefixes))

	// ── Step 1: Paginate queryIssueList ───────────────────────────────────────
	var collectedIssues []crashsight.IssueItem
	offset := 0
	pageSize := 100
	stoppedEarly := false

	for {
		log.Printf("fetching issue list start=%d", offset)
		resp, err := client.GetIssueList(ctx, crashsight.GetIssueListParams{
			Rows:      pageSize,
			Start:     offset,
			SortField: "uploadTime",
			SortOrder: "desc",
		})
		if err != nil {
			logAPIError("GetIssueList", err)
			break
		}

		for _, issue := range resp.IssueList {
			// Check date boundary
			var lastDate string
			if len(issue.LastestUploadTime) >= 10 {
				lastDate = issue.LastestUploadTime[:10]
			}
			if lastDate != "" && lastDate < minDateDash {
				log.Printf("  hit time boundary (%s < %s), stopping pagination", lastDate, minDateDash)
				stoppedEarly = true
				break
			}

			// Version filter (check FirstCrashVersion or just the Issue's version)
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

		if stoppedEarly || len(resp.IssueList) < pageSize {
			break
		}
		if *maxIssues > 0 && len(collectedIssues) >= *maxIssues {
			collectedIssues = collectedIssues[:*maxIssues]
			break
		}
		offset += pageSize
		time.Sleep(2 * time.Second) // rate limit
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

		trends, err := client.GetIssueTrend(ctx, crashsight.GetIssueTrendParams{
			IssueIDs:        chunk,
			MinDate:         trendStartStr,
			MaxDate:         trendEndStr,
			GranularityUnit: crashsight.GranularityDay,
		})
		if err != nil {
			logAPIError("GetIssueTrend", err)
			continue
		}

		for _, t := range trends {
			var totalCrash, totalUser int64
			for _, pt := range t.TrendList {
				totalCrash += pt.UploadCount
				totalUser += pt.ImeiCount
			}
			trendMap[t.IssueID] = struct{ Crash, User int64 }{totalCrash, totalUser}
		}
		time.Sleep(2 * time.Second) // rate limit
	}

	// ── Step 3: Merge and Format ──────────────────────────────────────────────
	report := ScanReport{
		StartDate: startDate,
		EndDate:   endDate,
		AppID:     appID,
		Platform:  "PC",
		Prefixes:  []string(prefixes),
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

		ir := IssueReport{
			IssueID:        issue.IssueID,
			ExceptionName:  issue.ExceptionName,
			ExceptionMsg:   issue.ExceptionMessage,
			RawStack:       issue.KeyStack,
			Processors:     processors,
			Status:         issue.Status,
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
		"IssueID", "ExceptionName", "ExceptionMsg", "Status", "Processors",
		"TrendCrashNum", "TrendCrashUser", "SampleCrashID", "AppVersion",
		"GPU", "GPUDriverVersion", "CPU", "MemoryMB", "FreeMemoryMB",
		"VRAMTotalMB", "VRAMUsedMB", "ElapsedTimeSec", "Country", "OsVer",
	}
	if err := w.Write(header); err != nil {
		return err
	}

	for _, e := range report.Issues {
		row := []string{
			e.IssueID, e.ExceptionName, e.ExceptionMsg, fmt.Sprint(e.Status), strings.Join(e.Processors, "|"),
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
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; padding: 20px; background: #f5f7f9; }
        .container { max-width: 1400px; margin: 0 auto; }
        .card { background: white; border-radius: 8px; padding: 20px; margin-bottom: 20px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .chart { width: 100%%; height: 400px; }
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
            <div class="stat-box"><div>Active Issues</div><div class="stat-num">%d</div></div>
            <div class="stat-box"><div>Total Crashes (Window)</div><div class="stat-num">%d</div></div>
            <div class="stat-box"><div>Date Range</div><div style="margin-top: 10px; font-size: 20px;">%s to %s</div></div>
        </div>

        <div class="card">
            <h2>Top GPU Crash Distribution (Weighted by Volume)</h2>
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
        const vramBuckets = {'< 2GB': 0, '2-4GB': 0, '4-8GB': 0, '8-12GB': 0, '> 12GB': 0};
        const timeBuckets = {'< 1m': 0, '1-5m': 0, '5-15m': 0, '15-30m': 0, '> 30m': 0};
        
        rawData.forEach(item => {
            const vol = item.trendCrashNum || 0;
            if (vol === 0) return;

            // GPU
            let gpu = item.gpu || 'Unknown';
            if (gpu.length > 30) gpu = gpu.substring(0, 30) + '...';
            gpuCounts[gpu] = (gpuCounts[gpu] || 0) + vol;
            
            // VRAM
            const v = item.vramUsedMB;
            if (v > 0) {
                if (v < 2000) vramBuckets['< 2GB'] += vol;
                else if (v < 4000) vramBuckets['2-4GB'] += vol;
                else if (v < 8000) vramBuckets['4-8GB'] += vol;
                else if (v < 12000) vramBuckets['8-12GB'] += vol;
                else vramBuckets['> 12GB'] += vol;
            }
            
            // Elapsed
            const t = item.elapsedTimeSec;
            if (t > 0) {
                if (t < 60) timeBuckets['< 1m'] += vol;
                else if (t < 300) timeBuckets['1-5m'] += vol;
                else if (t < 900) timeBuckets['5-15m'] += vol;
                else if (t < 1800) timeBuckets['15-30m'] += vol;
                else timeBuckets['> 30m'] += vol;
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

        // 2. VRAM Chart
        const vramChart = echarts.init(document.getElementById('vramChart'));
        vramChart.setOption({
            tooltip: { trigger: 'item' },
            xAxis: { type: 'category', data: Object.keys(vramBuckets) },
            yAxis: { type: 'value', name: 'Crash Count' },
            series: [{
                data: Object.values(vramBuckets),
                type: 'bar',
                itemStyle: { color: '#91cc75' },
                label: { show: true, position: 'top' }
            }]
        });

        // 3. Elapsed Time Chart
        const elapsedChart = echarts.init(document.getElementById('elapsedChart'));
        elapsedChart.setOption({
            tooltip: { trigger: 'item' },
            xAxis: { type: 'category', data: Object.keys(timeBuckets) },
            yAxis: { type: 'value', name: 'Crash Count' },
            series: [{
                data: Object.values(timeBuckets),
                type: 'line',
                smooth: true,
                areaStyle: {},
                itemStyle: { color: '#fac858' },
                label: { show: true, position: 'top' }
            }]
        });

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

	jsonBytes, err := json.Marshal(report.Issues)
	if err != nil {
		return err
	}

	htmlContent := fmt.Sprintf(htmlTemplate, report.TotalIssue, report.TotalCrash, report.StartDate, report.EndDate, string(jsonBytes))
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
