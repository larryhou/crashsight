# CrashSight Go SDK

A Go SDK for the [CrashSight OpenAPI](https://crashsight.qq.com), covering crash analytics, issue management, trend statistics, and device diagnostics.

- **Zero external dependencies** — pure Go standard library
- **Concurrent-safe** — a single `Client` instance can be shared freely across goroutines
- **Strongly typed** — every request uses a dedicated `XxxParams` struct; every response is a named Go type
- **Unified error handling** — `APIError`, `AuthError`, `RateLimitError`, `TransportError`, `ParseError`

```
import "github.com/larryhou/crashsight"
```

---

## Quick Start

### 1. Prerequisites

Obtain the following from the CrashSight console  
(**Profile → Personal Settings → OpenAPI Configuration**):

| Variable | Description |
|---|---|
| `CRASHSIGHT_USER_ID` | Numeric `localUserId` (e.g. `10565`) |
| `CRASHSIGHT_API_KEY` | `userOpenapiKey` (UUID format) |
| `CRASHSIGHT_APP_ID` | Project `appId` |
| `CRASHSIGHT_REGION` | `cn` (default) or `sg` |

Export them in your shell — never hard-code credentials in source files:

```bash
export CRASHSIGHT_USER_ID=<localUserId>
export CRASHSIGHT_API_KEY=<userOpenapiKey>
export CRASHSIGHT_APP_ID=<appId>
export CRASHSIGHT_REGION=cn
```

### 2. Add the Dependency

```bash
go get github.com/larryhou/crashsight
```

Or, if you are working from a local checkout:

```go
// go.mod
require github.com/larryhou/crashsight v0.0.0
replace github.com/larryhou/crashsight => /path/to/local/crashsight
```

### 3. Create a Client

```go
package main

import (
    "os"
    "time"

    "github.com/larryhou/crashsight"
)

func main() {
    client := crashsight.NewClient(
        os.Getenv("CRASHSIGHT_USER_ID"),
        os.Getenv("CRASHSIGHT_API_KEY"),
        os.Getenv("CRASHSIGHT_APP_ID"),
        crashsight.PlatformPC,
        crashsight.WithRegion(crashsight.RegionCN), // default, can be omitted
        crashsight.WithTimeout(30*time.Second),
    )
    _ = client
}
```

`NewClient` returns immediately; the underlying `*http.Client` with its
connection pool is initialised once and reused for every subsequent call.

### 4. Fetch Daily Crash Trends

```go
import (
    "context"
    "fmt"
    "log"
    "os"
    "time"

    "github.com/larryhou/crashsight"
)

func main() {
    client := crashsight.NewClient(
        os.Getenv("CRASHSIGHT_USER_ID"),
        os.Getenv("CRASHSIGHT_API_KEY"),
        os.Getenv("CRASHSIGHT_APP_ID"),
        crashsight.PlatformPC,
    )
    ctx := context.Background()
    appID := os.Getenv("CRASHSIGHT_APP_ID")

    end := time.Now()
    start := end.AddDate(0, 0, -7)

    items, err := client.GetTrend(ctx, appID, crashsight.PlatformPC, crashsight.GetTrendParams{
        StartDate:     start.Format("20060102"),
        EndDate:       end.Format("20060102"),
        VersionList:   []string{"-1"}, // "-1" = all versions
        MergeVersions: true,
    })
    if err != nil {
        log.Fatal(err)
    }
    for _, item := range items {
        fmt.Printf("%-10s  crash=%d/%d  access=%d\n",
            item.Date, item.CrashNum, item.CrashUser, item.AccessUser)
    }
}
```

### 5. Drill Down: Issue → Last Crash → Full Stack

```go
// Step 1 — find the top issue
top, err := client.GetTopIssues(ctx, appID, crashsight.PlatformPC, crashsight.GetTopIssuesParams{
    MinDate:    start.Format("20060102"),
    MaxDate:    end.Format("20060102"),
    Limit:      1,
    MergeDates: true,
})
if err != nil { log.Fatal(err) }
issueID := top.TopIssueList[0].IssueID

// Step 2 — get the most recent crash
last, err := client.GetLastCrash(ctx, appID, crashsight.PlatformPC, issueID)
if err != nil { log.Fatal(err) }

// Step 3 — fetch the full crash document
crashHash := crashsight.CrashIDToHash(last.CrashID)
doc, err := client.GetCrashDoc(ctx, appID, crashsight.PlatformPC, crashHash, crashsight.GetCrashDocParams{})
if err != nil { log.Fatal(err) }

fmt.Println(doc.CrashMap.CallStack)
```

### 6. Handle Errors

```go
import "errors"

_, err := client.GetTopIssues(ctx, appID, crashsight.PlatformPC, params)
if err != nil {
    var apiErr  *crashsight.APIError
    var authErr *crashsight.AuthError
    var rateErr *crashsight.RateLimitError
    switch {
    case errors.As(err, &apiErr):
        // business-level error — inspect apiErr.StatusCode, apiErr.TraceID
        fmt.Printf("API error %d: %s (trace=%s)\n",
            apiErr.StatusCode, apiErr.Message, apiErr.TraceID)
    case errors.As(err, &authErr):
        // wrong userID or apiKey
        log.Fatalf("authentication failed: %s", authErr.Message)
    case errors.As(err, &rateErr):
        // 25 req/min limit — back off and retry
        time.Sleep(5 * time.Second)
    default:
        log.Fatal(err)
    }
}
```

### 7. Run the Integration Tests

```bash
cd crashsight
CRASHSIGHT_USER_ID=<userId>  \
CRASHSIGHT_API_KEY=<apiKey>  \
CRASHSIGHT_APP_ID=<appId>    \
CRASHSIGHT_REGION=cn         \
go test -v -run TestIntegration -timeout 180s
```

---

## Architecture

```
github.com/larryhou/crashsight
├── types.go       — Platform, CrashType, VmType, IssueStatus, Region …
├── errors.go      — APIError, AuthError, RateLimitError, TransportError, ParseError
├── auth.go        — HMAC-SHA256 signature (per-request, no shared state)
├── models.go      — ~60 request XxxParams + response structs
├── client.go      — NewClient, ClientOption, HTTP dispatch, handleResponse
├── trend.go       — 8 trend/statistics methods
├── issue.go       — 11 issue management methods
├── crash.go       — 8 crash analysis methods
├── device.go      — 10 user & device methods
├── oom.go         — QueryOOMList
├── attachment.go  — FetchCrashAttachments
├── selector.go    — GetSelectorData, GetVersionDateList
└── cmd/
    ├── demo/      — runnable usage example
    └── compare/   — Python vs Go response diff tool
```

### Authentication

Every request signs its URL query string with:

```
userSecret = Base64( HexString( HMAC-SHA256(key=apiKey, msg="{userID}_{timestamp}") ) )
```

The timestamp is generated fresh per request; there is no global mutable state.

### Client Options

| Option | Default | Description |
|---|---|---|
| `WithRegion(r)` | `RegionCN` | Switch between `RegionCN` / `RegionSG` |
| `WithBaseURL(url)` | — | Override base URL (useful for proxies / tests) |
| `WithTimeout(d)` | `30s` | Per-request timeout |
| `WithHTTPClient(hc)` | built-in | Replace the underlying `*http.Client` |

---

## API Reference

### Trend Statistics (`trend.go`)

| Method | Endpoint | Key params |
|---|---|---|
| `GetTrend` | `POST /getTrendEx` | `StartDate`, `EndDate`, `VersionList` |
| `GetDailySummary` | `POST /fetchDailySummary` | `StartDate`, `EndDate` |
| `GetRealtimeTrendAppend` | `POST /getAppRealTimeTrendAppendEx` | `Date` (today) |
| `GetHourlyTrend` | `POST /getRealTimeHourlyStatEx` | `StartHour`, `EndHour` (`YYYYMMDDHH`) |
| `GetHourlyTopIssues` | `POST /getTopIssueHourly` | `StartHour`, `Limit` |
| `GetDimensionTopStats` | `POST /fetchDimensionTopStats` | `Field`: `"model"/"osVersion"/"version"` |
| `GetMinuteCrashData` | `POST /getMinuteCrashData` | `StartTime`, `EndTime` (`YYYY-MM-DD HH:MM:SS`) |
| `GetRealTimeAppendStat` | `GET /getRealTimeAppendStat` | `startHour`, `endHour` |

### Issue Management (`issue.go`)

| Method | Endpoint |
|---|---|
| `GetIssueList` | `POST /queryIssueList` |
| `GetTopIssues` | `POST /getTopIssueEx` |
| `GetIssueInfo` | `POST /issueInfo` |
| `GetIssueNotes` | `GET /noteList/...` |
| `GetIssueTrend` | `POST /queryIssueTrend` |
| `UpdateIssueStatus` | `POST /updateIssueStatus` |
| `AddIssueNote` | `POST /addIssueNote` |
| `AddIssueTag` | `POST /addTag` |
| `UpsertBugs` | `POST /upsertBugs` |
| `QueryBugs` | `POST /queryBugs` |
| `BindBugs` | `POST /bindBugs` |

### Crash Analysis (`crash.go`)

| Method | Endpoint |
|---|---|
| `GetCrashList` | `POST /crashList` |
| `GetLastCrash` | `POST /lastCrashInfo` |
| `GetCrashDetail` | `POST /appDetailCrash` |
| `GetCrashDoc` | `POST /crashDoc` |
| `GetANRMessage` | `POST /appDetailCrash` (filters ANR files) |
| `QueryCrashList` | `POST /queryCrashList` |
| `AdvancedSearch` | `POST /advancedSearchEx` |
| `GetStackCrashStat` | `POST /getStackCrashStat/platformId/{pid}` |

### User & Device (`device.go`)

| Method | Endpoint | Platform |
|---|---|---|
| `QueryUserAccessList` | `POST /queryAccessList` | All |
| `GetCrashUserInfo` | `POST /getCrashUserInfo/platformId/{pid}` | All |
| `GetCrashUserList` | `POST /getCrashUserList/platformId/{pid}` | All |
| `GetMostReportUsers` | `POST /getMostReportUser` | All |
| `GetNetworkDevices` | `POST /getNetworkDevices/platformId/{pid}` | Mobile only |
| `GetCrashDeviceStat` | `POST /getCrashDeviceStat/platformId/{pid}` | All |
| `GetCrashDeviceInfo` | `POST /getCrashDeviceInfo/platformId/{pid}` | Mobile only |
| `GetDeviceUserInfo` | `POST /getDeviceUserInfo/platformId/{pid}` | Mobile only |
| `GetStackDeviceInfo` | `POST /getStackDeviceInfo/platformId/{pid}` | All |
| `GetCrashDeviceInfoByExpUID` | `POST /getCrashDeviceInfoByExpUid/platformId/{pid}` | Mobile only |

### Other

| Method | File | Description |
|---|---|---|
| `QueryOOMList` | `oom.go` | OOM / non-OOM crash query with DSL conditions |
| `FetchCrashAttachments` | `attachment.go` | Batch download SDK logs, ANR traces, etc. |
| `GetSelectorData` | `selector.go` | Version list, tag list, processor list |
| `GetVersionDateList` | `selector.go` | First-seen date per version |

---

## Important Notes

| Topic | Detail |
|---|---|
| **Rate limit** | 25 requests / minute per user across all endpoints |
| **`platformId` type** | `issueInfo`, `lastCrashInfo`, `crashDoc`, `appDetailCrash` require a **string** `platformId` in the request body — the SDK handles this automatically |
| **`crashHash`** | `GetCrashDoc` / `GetCrashDetail` accept a hash, not a raw ID. Use `CrashIDToHash(crashID)` to convert |
| **Mobile-only APIs** | `GetNetworkDevices`, `GetCrashDeviceInfo`, `GetDeviceUserInfo` support Android / iOS only |
| **Slow endpoint** | `GetMinuteCrashData` can exceed 30 s — use `WithTimeout(120*time.Second)` |
| **Date format** | `GetIssueTrend` `MinDate`/`MaxDate` must be `YYYY-MM-DD HH:MM:SS`; do **not** pass Go layout strings directly |
| **Concurrency** | `Client` is safe to share across goroutines without any locking |

---

## Deployment Regions

| Region | Base URL |
|---|---|
| China (`RegionCN`) | `https://crashsight.qq.com` |
| Singapore (`RegionSG`) | `https://crashsight.wetest.net` |

---

## License

MIT
