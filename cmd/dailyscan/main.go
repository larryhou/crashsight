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
	AppVersion       string `json:"appVersion"`
	GPU              string `json:"gpu"`
	GPUDriverVersion string `json:"gpuDriverVersion"`
	CPU              string `json:"cpu"`
	MemoryMB         int64  `json:"memoryMB"`
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
			CrashNum:      int64(len(crashes)),
			CrashUser:     int64(len(uniqueDevices)),
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
