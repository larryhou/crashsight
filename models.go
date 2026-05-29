package crashsight

import "encoding/json"

// ─────────────────────────────────────────────────────────────────────────────
//  公共子结构
// ─────────────────────────────────────────────────────────────────────────────

// SearchCondition 高级搜索/OOM 查询的单个条件。
type SearchCondition struct {
	// Field 搜索字段，如 "version"、"oomStatus"、"exceptionCategory"。
	Field string `json:"field"`
	// QueryType 查询类型，见 QueryType 常量。
	QueryType QueryType `json:"queryType"`
	// Term 单值匹配（QueryTypeTerm 使用）。
	Term string `json:"term,omitempty"`
	// Terms 多值匹配（QueryTypeTerms / QueryTypeTermsWildcard 使用）。
	Terms []string `json:"terms,omitempty"`
	// Gte 相对时间范围的起始偏移（毫秒，QueryTypeRangeRelativeDatetime 使用）。
	Gte int64 `json:"gte,omitempty"`
}

// SearchConditionGroup 一组搜索条件（AND 关系）。
type SearchConditionGroup struct {
	Conditions []SearchCondition `json:"conditions"`
}

// TagInfo 标签信息。
// 注意：服务端 tagId 以整数形式返回（如 6807），使用 int64。
type TagInfo struct {
	TagID    int64  `json:"tagId"`
	TagName  string `json:"tagName"`
	TagType  int    `json:"tagType,omitempty"`
	// TagCount 该标签下的崩溃数量。
	TagCount int    `json:"tagCount,omitempty"`
	// 以下字段仅在 getSelectorDatas 的 tagList 中返回。
	AppID      string `json:"appId,omitempty"`
	PlatformID int    `json:"platformId,omitempty"`
	IsShow     int    `json:"isShow,omitempty"`
}

// BugInfo 关联的缺陷单信息。
type BugInfo struct {
	ID          string `json:"id"`
	Title       string `json:"title,omitempty"`
	WorkspaceID string `json:"workspaceId,omitempty"`
}

// IssueVersionItem 问题在某个版本上的统计。
type IssueVersionItem struct {
	Version               string `json:"version"`
	FirstUploadTime       string `json:"firstUploadTime,omitempty"`
	FirstUploadTimestamp  int64  `json:"firstUploadTimestamp,omitempty"`
	LastUploadTime        string `json:"lastUploadTime,omitempty"`
	LastUploadTimestamp   int64  `json:"lastUploadTimestamp,omitempty"`
	Count                 int64  `json:"count,omitempty"`
	DeviceCount           int64  `json:"deviceCount,omitempty"`
	SystemExitCount       int64  `json:"systemExitCount,omitempty"`
	SystemExitDeviceCount int64  `json:"systemExitDeviceCount,omitempty"`
}

type AssigneeItem struct {
	LocalUserID string `json:"localUserId,omitempty"`
	Name        string `json:"name,omitempty"`
	WetestUin   string `json:"wetestUin,omitempty"`
}

type LastMatchedReport struct {
	Archived   bool      `json:"archived"`
	CrashMap   CrashData `json:"crashMap"`
	Exists     bool      `json:"exists"`
	WellFormed bool      `json:"wellFormed"`
}

// IssueItem 问题摘要（TOP 问题列表/问题查询中均使用）。
type IssueItem struct {
	AppID                 string             `json:"appId"`
	PlatformID            int                `json:"platformId"`
	Version               string             `json:"version,omitempty"`
	// Date 仅 getTopIssueEx 等响应中包含，格式 YYYYMMDD。
	Date                  string             `json:"date,omitempty"`
	// Type 异常类型字符串，仅 getTopIssueEx 等响应中包含。
	Type                  string             `json:"type,omitempty"`
	IssueID               string             `json:"issueId"`
	ExceptionName         string             `json:"exceptionName,omitempty"`
	ExceptionMessage      string             `json:"exceptionMessage,omitempty"`
	KeyStack              string             `json:"keyStack,omitempty"`
	FirstUploadTime       string             `json:"firstUploadTime,omitempty"`
	FirstUploadTimestamp  int64              `json:"firstUploadTimestamp,omitempty"`
	LastestUploadTime     string             `json:"lastestUploadTime,omitempty"`
	// LastUpdateTime 最近更新时间，getTopIssueHourly/getTopIssueEx 中返回。
	LastUpdateTime        string             `json:"lastUpdateTime,omitempty"`
	LastUpdateTimestamp   int64              `json:"lastUpdateTimestamp,omitempty"`
	CrashNum              int64              `json:"crashNum,omitempty"`
	CrashUser             int64              `json:"crashUser,omitempty"`
	AccumulateCrashNum    int64              `json:"accumulateCrashNum,omitempty"`
	AccumulateCrashUser   int64              `json:"accumulateCrashUser,omitempty"`
	PreDayCrashNum        int64              `json:"preDayCrashNum,omitempty"`
	PreDayCrashUser       int64              `json:"preDayCrashUser,omitempty"`
	// PrevHourCrashDevices 上一小时崩溃设备数，getTopIssueHourly 中返回。
	PrevHourCrashDevices  int64              `json:"prevHourCrashDevices,omitempty"`
	State                 int                `json:"state,omitempty"`
	Processors            string             `json:"processors,omitempty"`
	Processor             string             `json:"processor,omitempty"`
	Status                int                `json:"status,omitempty"`
	IsSystemExit          string             `json:"is_system_exit,omitempty"`
	FtName                string             `json:"ftName,omitempty"`
	Tags                  []TagInfo          `json:"tags,omitempty"`
	TagInfoList           []TagInfo          `json:"tagInfoList,omitempty"`
	Bugs                  []BugInfo          `json:"bugs,omitempty"`
	IssueVersions         []IssueVersionItem `json:"issueVersions,omitempty"`
	ImeiCount             int64              `json:"imeiCount,omitempty"`
	Count                 int64              `json:"count,omitempty"`
	// 以下为 queryIssueList 响应中可能存在的额外字段
	AssigneeList          []AssigneeItem     `json:"assigneeList,omitempty"`
	CrossVerStat          int                `json:"crossVerStat,omitempty"`
	DetailID              string             `json:"detailId,omitempty"`
	EsCount               int64              `json:"esCount,omitempty"`
	EsDeviceCount         int64              `json:"esDeviceCount,omitempty"`
	FirstCrashVersion     string             `json:"firstCrashVersion,omitempty"`
	IssueExceptionType    int                `json:"issueExceptionType,omitempty"`
	IssueHash             string             `json:"issueHash,omitempty"`
	ParentHash            string             `json:"parentHash,omitempty"`
	SysCount              int64              `json:"sysCount,omitempty"`
	SysImeiCount          int64              `json:"sysImeiCount,omitempty"`
	Tag                   []string           `json:"tag,omitempty"`
	LastMatchedReport     *LastMatchedReport `json:"lastMatchedReport,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────────
//  趋势统计
// ─────────────────────────────────────────────────────────────────────────────

// GetTrendParams GetTrend 的可选参数。
type GetTrendParams struct {
	// StartDate 开始日期，格式 YYYYMMDD。必填。
	StartDate string
	// EndDate 结束日期，格式 YYYYMMDD。必填。
	EndDate string
	// CrashType 异常类型，默认 CrashTypeCrash。
	CrashType CrashType
	// VersionList 版本列表，支持通配符 *。默认 ["-1"]（全版本）。
	VersionList []string
	// VM 设备类型过滤，默认 VmTypeAll。
	VM VmType
	// NeedCountry 是否按国家维度统计。
	NeedCountry bool
	// CountryList 国家列表，空表示全部。
	CountryList []string
	// MergeVersions 多版本结果是否合并（合并后数据不精确）。
	MergeVersions bool
	// SceneTags 场景标签筛选（可选）。
	SceneTags []string
}

// TrendDataItem 每日/每小时趋势数据点。
type TrendDataItem struct {
	AppID              string  `json:"appId"`
	PlatformID         int     `json:"platformId"`
	Version            string  `json:"version"`
	Date               string  `json:"date"`
	Country            string  `json:"country,omitempty"`
	CrashNum           int64   `json:"crashNum"`
	CrashUser          int64   `json:"crashUser"`
	AccessNum          int64   `json:"accessNum"`
	AccessUser         int64   `json:"accessUser"`
	CrashRate          float64 `json:"crashRate,omitempty"`
	ReportNumAllData   int64   `json:"reportNumAllData,omitempty"`
	ReportDeviceAllData int64  `json:"reportDeviceAllData,omitempty"`
}

// GetDailySummaryParams GetDailySummary 的参数。
type GetDailySummaryParams struct {
	// StartDate 开始日期，格式 YYYYMMDD。必填。
	StartDate string
	// EndDate 结束日期，格式 YYYYMMDD。必填。
	EndDate string
	// Version 版本号，默认 "-1"（全版本）。
	Version string
	// QueryAllVmTypes 是否包含所有设备类型。
	QueryAllVmTypes bool
}

// DailySummaryItem 日级精简统计数据。
type DailySummaryItem struct {
	AppID        string `json:"appId"`
	PlatformID   int    `json:"platformId"`
	Version      string `json:"version"`
	Date         string `json:"date"`
	// Type 异常类型字符串，getRealTimeAppendStat 响应中包含。
	Type         string `json:"type,omitempty"`
	CrashNum     int64  `json:"crashNum"`
	CrashUser    int64  `json:"crashUser"`
	AnrNum       int64  `json:"anrNum"`
	AnrUser      int64  `json:"anrUser"`
	ErrorNum     int64  `json:"errorNum"`
	ErrorUser    int64  `json:"errorUser"`
	OomNum       int64  `json:"oomNum,omitempty"`
	JankNum      int64  `json:"jankNum,omitempty"`
	HangNum      int64  `json:"hangNum,omitempty"`
	AccessNum    int64  `json:"accessNum"`
	AccessUser   int64  `json:"accessUser"`
	// 以下 VM 虚拟机相关字段仅 getRealTimeAppendStat 返回。
	VmCrashNum   int64  `json:"vmCrashNum,omitempty"`
	VmCrashUser  int64  `json:"vmCrashUser,omitempty"`
	VmAnrNum     int64  `json:"vmAnrNum,omitempty"`
	VmAnrUser    int64  `json:"vmAnrUser,omitempty"`
	VmErrorNum   int64  `json:"vmErrorNum,omitempty"`
	VmErrorUser  int64  `json:"vmErrorUser,omitempty"`
}

// GetRealtimeTrendAppendParams GetRealtimeTrendAppend 的参数。
type GetRealtimeTrendAppendParams struct {
	// Date 日期，格式 YYYYMMDD。必填。
	Date string
	// Version 版本号，默认 "-1"。
	Version string
	// CrashType 异常类型，默认 CrashTypeCrash。
	CrashType CrashType
	// VM 设备类型，默认 VmTypeAll。
	VM VmType
	// MergeVersions 是否合并多版本。
	MergeVersions bool
	// NeedCountry 是否按国家统计。
	NeedCountry bool
	// CountryList 国家列表。
	CountryList []string
}

// GetHourlyTrendParams GetHourlyTrend 的参数。
type GetHourlyTrendParams struct {
	// StartHour 开始时间，格式 YYYYMMDDHH。必填。
	StartHour string
	// EndHour 结束时间，格式 YYYYMMDDHH。必填。
	EndHour string
	// Version 版本号，默认 "-1"。
	Version string
	// CrashType 异常类型，默认 CrashTypeCrash。
	CrashType CrashType
	// VM 设备类型，默认 VmTypeAll。
	VM VmType
	// MergeVersions 是否合并多版本。
	MergeVersions bool
	// NeedCountry 是否按国家统计。
	NeedCountry bool
	// CountryList 国家列表。
	CountryList []string
}

// GetHourlyTopIssuesParams GetHourlyTopIssues 的参数。
type GetHourlyTopIssuesParams struct {
	// StartHour 起始小时，格式 YYYYMMDDHH。必填。
	StartHour string
	// Version 版本号，默认 "-1"。
	Version string
	// CrashType 异常类型，默认 CrashTypeCrash。
	CrashType CrashType
	// VM 设备类型，默认 VmTypeAll。
	VM VmType
	// Limit 返回 TOP N 条数，默认 5。
	Limit int
	// CountryList 国家列表。
	CountryList []string
}

// HourlyTopIssuesResponse GetHourlyTopIssues 的响应。
type HourlyTopIssuesResponse struct {
	VersionCrashUser              int64       `json:"versionCrashUser"`
	PreDayVersionCrashUser        int64       `json:"preDayVersionCrashUser"`
	CrashDevices                  int64       `json:"crashDevices"`
	PrevHourCrashDevices          int64       `json:"prevHourCrashDevices"`
	PrevDaySameHourCrashDevices   int64       `json:"prevDaySameHourCrashDevices"`
	AccessDevices                 int64       `json:"accessDevices"`
	PrevHourAccessDevices         int64       `json:"prevHourAccessDevices"`
	PrevDaySameHourAccessDevices  int64       `json:"prevDaySameHourAccessDevices"`
	TopIssueList                  []IssueItem `json:"topIssueList"`
}

// GetDimensionTopStatsParams GetDimensionTopStats 的参数。
type GetDimensionTopStatsParams struct {
	// Version 版本号。必填。
	Version string
	// MinDate 起始日期，格式 YYYYMMDD。必填。
	MinDate string
	// MaxDate 结束日期，格式 YYYYMMDD。必填。
	MaxDate string
	// CrashType 异常类型，默认 CrashTypeCrash。
	CrashType CrashType
	// Field 聚合维度："model"（设备型号）/ "osVersion"（系统版本）/ "version"（应用版本）。默认 "model"。
	Field string
	// Limit 返回条数，默认 20。
	Limit int
	// MergeVersions 是否合并多版本。
	MergeVersions bool
	// MergeDates 是否合并多天。
	MergeDates bool
	// SortByException 是否按异常数排序。
	SortByException bool
	// NeedCountry 是否按国家统计。
	NeedCountry bool
	// CountryList 国家列表。
	CountryList []string
}

// GetMinuteCrashDataParams GetMinuteCrashData 的参数。
type GetMinuteCrashDataParams struct {
	// StartTime 格式 YYYY-MM-DD HH:MM:SS。必填。
	StartTime string
	// EndTime 格式 YYYY-MM-DD HH:MM:SS。必填。
	EndTime string
	// ProductVersion 版本号，默认 "-1"。
	ProductVersion string
	// Limit 返回条数，默认 10。
	Limit int
}

// ─────────────────────────────────────────────────────────────────────────────
//  问题管理
// ─────────────────────────────────────────────────────────────────────────────

// GetIssueListParams GetIssueList 的参数。
type GetIssueListParams struct {
	// ExceptionTypeList 异常类型，逗号分隔，默认 ExceptionTypeCrash。
	ExceptionTypeList string
	// Rows 返回条数，默认 20。
	Rows int
	// SortField 排序字段，默认 "uploadTime"。
	SortField string
	// SortOrder 排序方向 "desc"/"asc"，默认 "desc"。
	SortOrder string
	// Status 按状态过滤，多选逗号分隔，如 "0,2"。空表示不过滤。
	Status string
	// Version 版本号，多版本分号分隔，支持通配符 *。
	Version string
	// TapdBugStatus "UNREPORTED"（未关联）/ "REPORTED"（已关联）。
	TapdBugStatus string
	// IssueUploadTimeRelativeMillis 过滤最近 N 毫秒内有上报的问题（0 表示不过滤）。
	IssueUploadTimeRelativeMillis int64
	// Start 用于分页，标识起始偏移量（如 0, 20, 40...）
	Start int
}

// IssueListResponse GetIssueList 的响应。
type IssueListResponse struct {
	AppID      string      `json:"appId"`
	// PlatformID 服务端以整数形式返回。
	PlatformID int         `json:"platformId"`
	IssueList  []IssueItem `json:"issueList"`
	NumFound   int64       `json:"numFound"`
}

// GetTopIssuesParams GetTopIssues 的参数。
type GetTopIssuesParams struct {
	// MinDate 起始日期，格式 YYYYMMDD。必填。
	MinDate string
	// MaxDate 结束日期，格式 YYYYMMDD。必填。
	MaxDate string
	// VersionList 版本列表，支持通配符 *。默认 ["-1"]（全版本）。
	VersionList []string
	// CrashType 异常类型，默认 CrashTypeCrash。
	CrashType CrashType
	// Limit 返回条数，默认 20。
	Limit int
	// TopIssueDataType 数据类型，默认 TopIssueDataTypeUnSystemExit。
	TopIssueDataType TopIssueDataType
	// MergeVersions 是否合并多版本。
	MergeVersions bool
	// MergeDates 是否查询多日数据。
	MergeDates bool
	// CountryList 国家列表。
	CountryList []string
}

// TopIssuesResponse GetTopIssues 的响应。
type TopIssuesResponse struct {
	TopIssueList        []IssueItem `json:"topIssueList"`
	CrashDevices        int64       `json:"crashDevices"`
	AccessDevices       int64       `json:"accessDevices"`
	PrevDayCrashDevices int64       `json:"prevDayCrashDevices"`
	PrevDayAccessDevices int64      `json:"prevDayAccessDevices"`
}

// IssueInfo GetIssueInfo 的响应。
type IssueInfo struct {
	IssueID              string             `json:"issueId"`
	ExceptionName        string             `json:"exceptionName"`
	ExceptionMessage     string             `json:"exceptionMessage"`
	KeyStack             string             `json:"keyStack"`
	LastestUploadTime    string             `json:"lastestUploadTime"`
	LatestUploadTimestamp int64             `json:"latestUploadTimestamp"`
	ImeiCount            int64              `json:"imeiCount"`
	SysImeiCount         int64              `json:"sysImeiCount"`
	Count                int64              `json:"count"`
	SysCount             int64              `json:"sysCount"`
	Version              string             `json:"version"`
	TagInfoList          []TagInfo          `json:"tagInfoList"`
	Processor            string             `json:"processor"`
	Status               int                `json:"status"`
	FirstUploadTime      string             `json:"firstUploadTime"`
	FirstUploadTimestamp int64              `json:"firstUploadTimestamp"`
	IssueHash            string             `json:"issueHash"`
	FtName               string             `json:"ftName"`
	IssueVersions        []IssueVersionItem `json:"issueVersions"`
	DetailID             string             `json:"detailId"`
	ParentHash           string             `json:"parentHash"`
	Bugs                 json.RawMessage    `json:"bugs"`
}

// IssueNote GetIssueNotes 返回的备注条目。
type IssueNote struct {
	AppID       string `json:"appId"`
	PlatformID  int    `json:"platformId"`
	IssueIDs    string `json:"issueIds"`
	Note        string `json:"note"`
	CreateTime  string `json:"createTime"`
	UserID      string `json:"userId"`
	NewUserID   string `json:"newUserId"`
	IssueStatus int    `json:"issueStatus"`
	Processors  string `json:"processors"`
	TapdID      string `json:"tapdId,omitempty"`
	BugURL      string `json:"bugUrl,omitempty"`
	WorkspaceID string `json:"workspaceId,omitempty"`
}

// GetIssueTrendParams GetIssueTrend 的参数。
type GetIssueTrendParams struct {
	// IssueIDs 问题 ID 列表。必填。
	IssueIDs []string
	// MinDate 格式 YYYY-MM-DD HH:MM:SS。必填。
	MinDate string
	// MaxDate 格式 YYYY-MM-DD HH:MM:SS。必填。
	MaxDate string
	// GranularityUnit 聚合粒度，默认 GranularityDay。
	GranularityUnit GranularityUnit
	// Version 版本号，默认 "-1"。
	Version string
}

// IssueTrendPoint 趋势数据点。
type IssueTrendPoint struct {
	Date      string `json:"date"`
	CrashNum  int64  `json:"crashNum"`
	CrashUser int64  `json:"crashUser"`
}

// IssueTrendItem 单个 issue 的趋势数据。
type IssueTrendItem struct {
	IssueID   string            `json:"issueId"`
	TrendList []IssueTrendPoint `json:"trendList"`
}

// UpdateIssueStatusParams UpdateIssueStatus 的参数。
type UpdateIssueStatusParams struct {
	// IssueIDs 问题 ID，多个逗号分隔。必填。
	IssueIDs string
	// Status 目标状态。必填。
	Status IssueStatus
	// Processors 处理人 ID。
	Processors string
	// Note 备注。
	Note string
	// OperatorUserID 操作人 userId（默认使用 Client 的 userID）。
	OperatorUserID string
}

// AddIssueNoteParams AddIssueNote 的参数。
type AddIssueNoteParams struct {
	// IssueID 问题 ID。必填。
	IssueID string
	// Note 备注内容。必填。
	Note string
	// OperatorUserID 操作人（默认使用 Client 的 userID）。
	OperatorUserID string
}

// UpsertBugsParams UpsertBugs 的参数。
type UpsertBugsParams struct {
	// IssueID 问题 ID。必填。
	IssueID string
	// Extra 额外的自定义字段（按需透传）。
	Extra map[string]any
}

// BugInfoParam QueryBugs 中的单个缺陷单参数。
type BugInfoParam struct {
	// BugPlatform 缺陷单平台，如 "TAPD"。
	BugPlatform string `json:"bugPlatform"`
	// ID 缺陷单 ID。
	ID string `json:"id"`
}

// BindBugsParams BindBugs 的参数。
type BindBugsParams struct {
	// IssueID 问题 ID。必填。
	IssueID string
	// BugID 缺陷单 ID。必填。
	BugID string
	// BugURL 缺陷单链接。必填。
	BugURL string
}

// ─────────────────────────────────────────────────────────────────────────────
//  异常分析
// ─────────────────────────────────────────────────────────────────────────────

// GetCrashListParams GetCrashList 的参数。
type GetCrashListParams struct {
	// IssueID 问题 ID。必填。
	IssueID string
	// Start 分页偏移，默认 0。配合 Rows 做翻页：start=0,100,200,...
	Start int
	// Rows 每页条数，默认 100。官方限制最大 100。
	Rows int
	// ExceptionTypeList 异常类型过滤，默认 ExceptionTypeCrash。
	ExceptionTypeList string
	// Version 版本过滤。
	Version string
}

// CrashData 单条崩溃记录摘要（来自 crashList GET 接口的 crashDatas 字段）。
// 已包含完整设备信息，无需再调 GetCrashDoc。
type CrashData struct {
	CrashID        string `json:"crashId"`
	UploadTime     string `json:"uploadTime"`
	ProductVersion string `json:"productVersion"`
	DeviceID       string `json:"deviceId"`
	UserID         string `json:"userId"`
	OsVer          string `json:"osVer"`
	Model          string `json:"model"`
	DumpID         string `json:"dumpId"`
	ID             string `json:"id"`
	// GPU 名称（如 "NVIDIA GeForce RTX 4060"）
	GPU              string `json:"gpu"`
	// GpuDriverVersion GPU 驱动版本（如 "32.0.15.7688"）
	GpuDriverVersion string `json:"gpuDriverVersion"`
	// CpuName CPU 型号
	CpuName          string `json:"cpuName"`
	// MemSize 物理内存总量（字节，字符串形式）
	MemSize          string `json:"memSize"`
	// 新增维度：可视化和多维度分析所需数据
	CrashTime        string            `json:"crashTime"`
	ElapsedTime      int64             `json:"elapsedTime"` // 运行时间（毫秒）
	FreeMem          string            `json:"freeMem"`     // 剩余内存（字节，字符串形式）
	Country          string            `json:"country"`     // 国家/地区
	ReservedMap      map[string]string `json:"reservedMap"` // 存放详尽的显存占用等硬件状态
}

// CrashListResponse GetCrashList 的响应。
type CrashListResponse struct {
	StatusCode  int                  `json:"statusCode"`
	Message     string               `json:"message"`
	NumFound    int64                `json:"numFound"`
	// CrashIDList 崩溃 ID 列表，是本接口的核心返回值。
	CrashIDList []string             `json:"crashIdList"`
	// IssueList 关联的 issue ID 列表。
	IssueList   []string             `json:"issueList"`
	CrashDatas  map[string]CrashData `json:"crashDatas"`
	CrashNums   int64                `json:"crashNums"`
	AnrNums     int64                `json:"anrNums"`
	ErrorNums   int64                `json:"errorNums"`
	ScrollID    string               `json:"scrollId"`
}

// LastCrashResponse GetLastCrash 的响应。
// 注意：AppInBack 等字段服务端以字符串 "true"/"false" 返回，而非 JSON boolean。
type LastCrashResponse struct {
	UserID             string `json:"userId"`
	ProcessName        string `json:"processName"`
	ThreadName         string `json:"threadName"`
	CrashID            string `json:"crashId"`
	CrashHash          string `json:"crashHash"`
	CrashTime          string `json:"crashTime"`
	UploadTime         string `json:"uploadTime"`
	BundleID           string `json:"bundleId"`
	ProductVersion     string `json:"productVersion"`
	StartTime          string `json:"startTime"`
	// AppInBack 服务端返回字符串 "true"/"false"，而非 JSON boolean。
	AppInBack          string `json:"appInBack"`
	Hardware           string `json:"hardware"`
	ModelOriginalName  string `json:"modelOriginalName"`
	OsVersion          string `json:"osVersion"`
	ROM                string `json:"rom"`
	CpuName            string `json:"cpuName"`
	CpuType            string `json:"cpuType"`
	Type               string `json:"type"`
	CallStack          string `json:"callStack"`
	RetraceCrashDetail string `json:"retraceCrashDetail"`
	GpuName            string `json:"gpuName"`
	DumpID             string `json:"dumpId"`
	// NewDumpID 注意 JSON key 使用 snake_case: "new_dumpid"。
	NewDumpID          string `json:"new_dumpid"`
	Mac                string `json:"mac"`
	// LaunchTime 服务端以字符串形式返回（Unix 秒时间戳字符串）。
	LaunchTime         string `json:"launchTime"`
}

// AttachItem 崩溃附件条目。
type AttachItem struct {
	// FileName 文件名。
	FileName string `json:"fileName"`
	// FileType 文件类型：1=日志 2=线程栈 3=其他值映射 6=自定义KV 7=服务端KV。
	FileType int `json:"fileType"`
	// Content 文件内容（文本）。
	Content string `json:"content"`
}

// CrashDetailResponse GetCrashDetail 的响应。
type CrashDetailResponse struct {
	AttachName string       `json:"attachName"`
	StackName  string       `json:"stackName"`
	AttachList []AttachItem `json:"attachList"`
	SysLogs    []string     `json:"sysLogs"`
	UserLogs   []string     `json:"userLogs"`
}

// FileItem 崩溃详情中的文件记录。
type FileItem struct {
	FileName    string `json:"fileName"`
	CodeType    string `json:"codeType"`
	FileType    int    `json:"fileType"`
	FileContent string `json:"fileContent"`
}

// CrashMap 崩溃基础信息（crashDoc 响应中的 crashMap）。
// 注意：服务端对 memSize/diskSize/freeMem/freeStorage/isRooted/appInBack 以字符串返回，
// isVirtualMachine 以整数返回，与直觉不符，已据实定义。
type CrashMap struct {
	ID                 string `json:"id"`
	IssueID            string `json:"issueId"`
	ProductVersion     string `json:"productVersion"`
	Model              string `json:"model"`
	UserID             string `json:"userId"`
	ExpMessage         string `json:"expMessage"`
	ExpName            string `json:"expName"`
	Type               string `json:"type"`
	ProcessName        string `json:"processName"`
	RetraceStatus      int    `json:"retraceStatus"`
	UploadTime         string `json:"uploadTime"`
	UploadTimestamp    int64  `json:"uploadTimestamp"`
	CrashTime          string `json:"crashTime"`
	CrashTimestamp     int64  `json:"crashTimestamp"`
	StartTime          string `json:"startTime"`
	StartTimestamp     int64  `json:"startTimestamp"`
	// AppInBack 服务端以字符串 "true"/"false" 返回。
	AppInBack          string `json:"appInBack"`
	CpuType            string `json:"cpuType"`
	CrashID            string `json:"crashId"`
	BundleID           string `json:"bundleId"`
	SdkVersion         string `json:"sdkVersion"`
	OsVer              string `json:"osVer"`
	ExpAddr            string `json:"expAddr"`
	ThreadName         string `json:"threadName"`
	// MemSize 服务端以字符串形式返回（如 "1587986432"）。
	MemSize            string `json:"memSize"`
	// DiskSize 服务端以字符串形式返回。
	DiskSize           string `json:"diskSize"`
	Imei               string `json:"imei"`
	Imsi               string `json:"imsi"`
	CpuName            string `json:"cpuName"`
	Brand              string `json:"brand"`
	// FreeMem 服务端以字符串形式返回。
	FreeMem            string `json:"freeMem"`
	// FreeStorage 服务端以字符串形式返回。
	FreeStorage        string `json:"freeStorage"`
	// FreeSdCard 服务端以字符串形式返回。
	FreeSdCard         string `json:"freeSdCard"`
	// TotalSD 服务端以字符串形式返回。
	TotalSD            string `json:"totalSD"`
	Mac                string `json:"mac"`
	Country            string `json:"country"`
	ChannelID          string `json:"channelId"`
	CallStack          string `json:"callStack"`
	RetraceCrashDetail string `json:"retraceCrashDetail"`
	RetraceResult      string `json:"retraceResult"`
	ROM                string `json:"rom"`
	BuildNumber        string `json:"buildNumber"`
	RetraceTimestamp   int64  `json:"retraceTimestamp"`
	Apn                string `json:"apn"`
	AppInAppstore      bool   `json:"appInAppstore"`
	DeviceID           string `json:"deviceId"`
	ModelOriginalName  string `json:"modelOriginalName"`
	CrashCount         int    `json:"crashCount"`
	// IsRooted 服务端以字符串 "true"/"false" 返回。
	IsRooted           string `json:"isRooted"`
	// IsVirtualMachine 服务端以整数返回（0/1 等），非 JSON boolean。
	IsVirtualMachine   int    `json:"isVirtualMachine"`
	MergeVersion       string `json:"mergeVersion"`
	MessageVersion     string `json:"messageVersion"`
	IsSystemStack      int    `json:"isSystemStack"`
	RqdUuid            string `json:"rqdUuid"`
	SubVersionIssueID  string `json:"subVersionIssueId"`
	KeyStack           string `json:"keyStack,omitempty"`
	// GPU GPU 名称（如 "NVIDIA GeForce RTX 2060 SUPER"）。
	GPU                string `json:"gpu"`
	// GpuDriverVersion GPU 驱动版本（如 "32.0.15.7680"）。
	GpuDriverVersion   string `json:"gpuDriverVersion"`
}

// DetailMap 崩溃附加详情（crashDoc 响应中的 detailMap）。
type DetailMap struct {
	// AttachCount 注意：服务端 key 拼写为 "attatchCount"（已保留原始拼写）。
	AttachCount        int        `json:"attatchCount"`
	AppInfo            string     `json:"appInfo"`
	StackName          string     `json:"stackName"`
	RetraceCrashDetail string     `json:"retraceCrashDetail"`
	FreeMem            int64      `json:"freeMem"`
	FreeSdCard         int64      `json:"freeSdCard,omitempty"`
	Battery            int        `json:"battery"`
	AttachName         string     `json:"attachName"`
	ID                 string     `json:"id"`
	FileList           []FileItem `json:"fileList"`
	SrcIP              string     `json:"srcIp"`
	UploadTimestamp    int64      `json:"uploadTimestamp"`
	ServerKey          string     `json:"serverKey"`
	// CPU 服务端以整数返回。
	CPU                int        `json:"cpu"`
	UploadTime         string     `json:"uploadTime"`
	UserKey            string     `json:"userKey"`
	RomName            string     `json:"romName"`
	CallStack          string     `json:"callStack"`
	SdkVersion         string     `json:"sdkVersion"`
	FreeStorage        int64      `json:"freeStorage"`
	// IsGZIP 服务端以整数返回。
	IsGZIP             int        `json:"isGZIP,omitempty"`
	ThreadName         string     `json:"threadName,omitempty"`
	ContactAll         string     `json:"contactAll,omitempty"`
	SdkID              string     `json:"sdkId,omitempty"`
	FileDir            string     `json:"fileDir,omitempty"`
	Comment            string     `json:"comment,omitempty"`
}

// CrashDocResponse GetCrashDoc 的完整响应。
// 所有设备字段（GPU/CPU/内存/驱动版本）均在 CrashMap 中。
type CrashDocResponse struct {
	StatusCode           int       `json:"statusCode"`
	Message              string    `json:"message"`
	NumFound             int       `json:"numFound"`
	CrashMap             CrashMap  `json:"crashMap"`
	DetailMap            DetailMap `json:"detailMap"`
	LaunchTime           int64     `json:"launchTime"`
	ReqSendTimestamp     int64     `json:"reqSendTimestamp,omitempty"`
	RspReceivedTimestamp int64     `json:"rspReceivedTimestamp,omitempty"`
	RspSendTimestamp     int64     `json:"rspSendTimestamp,omitempty"`
}

// QueryCrashListParams QueryCrashList 的参数。
type QueryCrashListParams struct {
	// CrashType 异常类型，默认 CrashTypeCrash。
	CrashType CrashType
	// Start 分页偏移，默认 0。
	Start int
	// Rows 每页条数，默认 50。
	Rows int
	// Version 版本过滤。
	Version string
	// StartDate 起始日期，格式 YYYYMMDD。
	StartDate string
	// EndDate 结束日期，格式 YYYYMMDD。
	EndDate string
	// Model 设备型号过滤。
	Model string
	// OsVersion 系统版本过滤。
	OsVersion string
	// Keyword 堆栈关键字过滤。
	Keyword string
	// UserID 用户 ID 过滤。
	UserID string
	// DeviceID 设备 ID 过滤。
	DeviceID string
	// CrashID 崩溃 ID 过滤。
	CrashID string
}

// AdvancedSearchParams AdvancedSearch 的参数。
type AdvancedSearchParams struct {
	// StartHour 格式 YYYYMMDDHH。必填。
	StartHour string
	// EndHour 格式 YYYYMMDDHH。必填。
	EndHour string
	// CrashType 异常类型，默认 CrashTypeCrash。
	CrashType CrashType
	// Version 版本号，默认 "-1"。
	Version string
	// VM 设备类型，默认 VmTypeAll。
	VM VmType
}

// StackCrashStatItem 堆栈关键字崩溃统计条目。
type StackCrashStatItem struct {
	KeyName    string `json:"keyName"`
	CrashNums  int64  `json:"crashNums"`
	CrashUsers int64  `json:"crashUsers"`
}

// StackCrashStatResponse GetStackCrashStat 的响应。
type StackCrashStatResponse struct {
	RequestID string               `json:"requestid"`
	Code      int                  `json:"code"`
	ErrMsg    string               `json:"errmsg"`
	Results   []StackCrashStatItem `json:"results"`
	Cost      int64                `json:"cost"`
}

// GetCrashDocParams GetCrashDoc 的可选参数。
type GetCrashDocParams struct {
	// LogType 仅 PC 有效：""（默认）/ "interface" / "file" / "all"。
	LogType string
	// NeedCustomKV 是否返回自定义 KV 字段。
	NeedCustomKV bool
}

// GetStackCrashStatParams GetStackCrashStat 的参数。
type GetStackCrashStatParams struct {
	// KeyName 堆栈关键字，支持 * 通配符。必填。
	KeyName string
	// StartTime 格式 YYYY-MM-DD HH:MM:SS。必填。
	StartTime string
	// EndTime 格式 YYYY-MM-DD HH:MM:SS。必填。
	EndTime string
	// Limit 返回条数，0 表示不限制。
	Limit int
}

// ─────────────────────────────────────────────────────────────────────────────
//  用户与设备
// ─────────────────────────────────────────────────────────────────────────────

// QueryUserAccessListParams QueryUserAccessList 的参数。
type QueryUserAccessListParams struct {
	// UploadTimeBeginMillis 查询起始时间（Unix 毫秒）。必填。
	UploadTimeBeginMillis int64
	// UserIDList 用户 ID 列表（与 DeviceIDList 二选一）。
	UserIDList []string
	// DeviceIDList 设备 ID 列表（与 UserIDList 二选一）。
	DeviceIDList []string
	// PageNumber 页码，默认 1。
	PageNumber int
	// PageSize 每页条数，默认 3000。
	PageSize int
	// SkipDistinctQuery 是否跳过去重查询，默认 true。
	SkipDistinctQuery bool
}

// GetCrashUserInfoParams GetCrashUserInfo 的参数。
type GetCrashUserInfoParams struct {
	// UserIDs 用户 ID 列表。必填。
	UserIDs []string
	// StartTime 格式 YYYY-MM-DD HH:MM:SS。必填。
	StartTime string
	// EndTime 格式 YYYY-MM-DD HH:MM:SS。必填。
	EndTime string
	// Limit 返回条数，默认 1000。
	Limit int
	// RequestID 可选 trace ID。
	RequestID string
}

// CrashUserInfoItem 用户崩溃详情条目。
type CrashUserInfoItem struct {
	IssueID   string `json:"issueId"`
	CrashTime string `json:"crashTime"`
	CrashID   string `json:"crashId"`
	User      string `json:"user"`
}

// CrashUserInfoResponse GetCrashUserInfo 的响应。
type CrashUserInfoResponse struct {
	RequestID string              `json:"requestid"`
	Code      int                 `json:"code"`
	ErrMsg    string              `json:"errmsg"`
	Results   []CrashUserInfoItem `json:"results"`
	Cost      int64               `json:"cost"`
}

// GetCrashUserListParams GetCrashUserList 的参数。
type GetCrashUserListParams struct {
	// StartDate 起始日期，格式 YYYYMMDD。必填。
	StartDate string
	// EndDate 结束日期，格式 YYYYMMDD（最多 30 天范围）。必填。
	EndDate string
	// CrashType 异常类型，默认 CrashTypeCrash。
	CrashType CrashType
	// Version 版本号，默认 "-1"。
	Version string
}

// GetMostReportUsersParams GetMostReportUsers 的参数。
type GetMostReportUsersParams struct {
	// Versions 版本列表，支持通配符。默认 ["-1"]（全版本）。
	Versions []string
	// TimeRangeMillis 时间范围（毫秒），默认 7 天 (604800000)。
	TimeRangeMillis int64
	// ExceptionCategory "CRASH" / "ANR" / "ERROR"，默认 "CRASH"。
	ExceptionCategory string
	// Limit 返回 Top N 用户数，默认 10。
	Limit int
	// NeedDistinctCount 是否对用户去重统计。nil 时默认为 true（与 Python SDK 行为一致）。
	NeedDistinctCount *bool
}

// GetNetworkDevicesParams GetNetworkDevices 的参数。
type GetNetworkDevicesParams struct {
	// StartTime 格式 YYYY-MM-DD HH:MM:SS。必填。
	StartTime string
	// EndTime 格式 YYYY-MM-DD HH:MM:SS。必填。
	EndTime string
	// RequestID 可选 trace ID。
	RequestID string
}

// GetCrashDeviceStatParams GetCrashDeviceStat 的参数。
type GetCrashDeviceStatParams struct {
	// DeviceIDs 设备 ID 列表。必填。
	DeviceIDs []string
	// StartTime 格式 YYYY-MM-DD HH:MM:SS。必填。
	StartTime string
	// EndTime 格式 YYYY-MM-DD HH:MM:SS。必填。
	EndTime string
	// Limit 返回条数，0 表示不限制。
	Limit int
}

// CrashDeviceStatItem 设备崩溃统计条目。
type CrashDeviceStatItem struct {
	ExceptionType string `json:"exceptionType"`
	DeviceID      string `json:"deviceId"`
	IssueID       string `json:"issueId"`
	CrashID       string `json:"crashId"`
	User          string `json:"user"`
	Hardware      string `json:"hardware"`
	Model         string `json:"model"`
}

// CrashDeviceStatResponse GetCrashDeviceStat 的响应。
type CrashDeviceStatResponse struct {
	RequestID string                `json:"requestid"`
	Code      int                   `json:"code"`
	ErrMsg    string                `json:"errmsg"`
	Results   []CrashDeviceStatItem `json:"results"`
	Cost      int64                 `json:"cost"`
}

// GetCrashDeviceInfoParams GetCrashDeviceInfo 的参数。
type GetCrashDeviceInfoParams struct {
	// IssueIDs 问题 ID 列表。必填。
	IssueIDs []string
	// StartTime 格式 YYYY-MM-DD HH:MM:SS。必填。
	StartTime string
	// EndTime 格式 YYYY-MM-DD HH:MM:SS。必填。
	EndTime string
	// Limit 返回条数，默认 1000。
	Limit int
	// RequestID 可选 trace ID。
	RequestID string
}

// CrashDeviceInfoItem 崩溃设备信息条目。
type CrashDeviceInfoItem struct {
	IssueID   string `json:"issueId"`
	CrashTime string `json:"crashTime"`
	CrashID   string `json:"crashId"`
	User      string `json:"user"`
}

// CrashDeviceInfoResponse GetCrashDeviceInfo 的响应。
type CrashDeviceInfoResponse struct {
	RequestID string                `json:"requestid"`
	Code      int                   `json:"code"`
	ErrMsg    string                `json:"errmsg"`
	Results   []CrashDeviceInfoItem `json:"results"`
	Cost      int64                 `json:"cost"`
}

// GetDeviceUserInfoParams GetDeviceUserInfo 的参数。
type GetDeviceUserInfoParams struct {
	// DeviceID 设备 ID。必填。
	DeviceID string
	// StartTime 格式 YYYY-MM-DD HH:MM:SS。必填。
	StartTime string
	// EndTime 格式 YYYY-MM-DD HH:MM:SS。必填。
	EndTime string
	// Limit 返回条数，默认 10。
	Limit int
	// RequestID 可选 trace ID。
	RequestID string
}

// DeviceUserInfoItem 设备关联用户条目。
type DeviceUserInfoItem struct {
	Time   string `json:"time"`
	UserID string `json:"userId"`
}

// DeviceUserInfoResponse GetDeviceUserInfo 的响应。
type DeviceUserInfoResponse struct {
	RequestID string               `json:"requestid"`
	Code      int                  `json:"code"`
	ErrMsg    string               `json:"errmsg"`
	Results   []DeviceUserInfoItem `json:"results"`
	Cost      int64                `json:"cost"`
}

// GetStackDeviceInfoParams GetStackDeviceInfo 的参数。
type GetStackDeviceInfoParams struct {
	// KeyName 堆栈关键字，支持 * 通配符。必填。
	KeyName string
	// StartTime 格式 YYYY-MM-DD HH:MM:SS。必填。
	StartTime string
	// EndTime 格式 YYYY-MM-DD HH:MM:SS。必填。
	EndTime string
	// Limit 返回条数，0 表示不限制。
	Limit int
}

// StackDeviceInfoItem 堆栈关键字对应机型条目。
type StackDeviceInfoItem struct {
	KeyName    string `json:"keyName"`
	Model      string `json:"model"`
	OsVersion  string `json:"osVersion"`
	CrashNums  int64  `json:"crashNums"`
	CrashUsers int64  `json:"crashUsers"`
}

// StackDeviceInfoResponse GetStackDeviceInfo 的响应。
type StackDeviceInfoResponse struct {
	RequestID string                `json:"requestid"`
	Code      int                   `json:"code"`
	ErrMsg    string                `json:"errmsg"`
	Results   []StackDeviceInfoItem `json:"results"`
	Cost      int64                 `json:"cost"`
}

// GetCrashDeviceInfoByExpUIDParams GetCrashDeviceInfoByExpUID 的参数。
type GetCrashDeviceInfoByExpUIDParams struct {
	// ExpUIDs expUid 列表。必填。
	ExpUIDs []string
	// StartTime 格式 YYYY-MM-DD HH:MM:SS。必填。
	StartTime string
	// EndTime 格式 YYYY-MM-DD HH:MM:SS。必填。
	EndTime string
	// Limit 返回条数，0 表示不限制。
	Limit int
}

// ─────────────────────────────────────────────────────────────────────────────
//  OOM 分析
// ─────────────────────────────────────────────────────────────────────────────

// QueryOOMListParams QueryOOMList 的参数。
type QueryOOMListParams struct {
	// SearchConditionGroup 搜索条件组。必填。
	SearchConditionGroup SearchConditionGroup
	// Limit 返回条数，默认 10。
	Limit int
}

// OOMItem OOM 崩溃条目（字段按实际服务端响应动态扩展，使用 RawMessage 保留原始）。
type OOMItem = json.RawMessage

// OOMAggItem OOM 聚合条目。
type OOMAggItem = json.RawMessage

// OOMListResponse QueryOOMList 的响应。
type OOMListResponse struct {
	Total               int64                      `json:"total"`
	Items               []OOMItem                  `json:"items"`
	AggList             []OOMAggItem               `json:"aggList"`
	ModelProductNameMap map[string]string          `json:"modelProductNameMap"`
}

// ─────────────────────────────────────────────────────────────────────────────
//  附件管理
// ─────────────────────────────────────────────────────────────────────────────

// FetchCrashAttachmentsParams FetchCrashAttachments 的参数。
type FetchCrashAttachmentsParams struct {
	// CrashIDList crashId 列表。必填。
	CrashIDList []string
	// AttachmentFilenameList 需要的附件文件名列表，默认 ["SDK_LOG"]。
	// 支持: SDK_LOG, CustomizedAttachFile.zip, CustomizedLogFile.log,
	//       extraMessage.txt, anrMessage.txt, trace.zip
	AttachmentFilenameList []string
}

// AttachmentInfo 单个附件信息。
type AttachmentInfo struct {
	DownloadURL  string `json:"downloadUrl"`
	FileName     string `json:"fileName"`
	FileSize     int64  `json:"fileSize,omitempty"`
	ExpireTime   int64  `json:"expireTime,omitempty"`
}

// CrashAttachmentItem 单个 crashId 的附件列表。
type CrashAttachmentItem struct {
	CrashID     string           `json:"crashId"`
	Attachments []AttachmentInfo `json:"attachments"`
}

// FetchCrashAttachmentsResponse FetchCrashAttachments 的响应。
type FetchCrashAttachmentsResponse struct {
	CrashIDAndAttachmentsList []CrashAttachmentItem `json:"crashIdAndAttachmentsList"`
}

// ─────────────────────────────────────────────────────────────────────────────
//  选择器与元数据
// ─────────────────────────────────────────────────────────────────────────────

// VersionItem 版本条目。
// 注意：服务端 enable 以整数 0/1 返回；isShow/enableAutoUpgrade 以 bool 返回。
type VersionItem struct {
	AppID             string `json:"appId"`
	PlatformID        int    `json:"platformId"`
	ProductVersion    string `json:"productVersion"`
	// Enable 服务端以整数返回（1=启用，0=禁用）。
	Enable            int    `json:"enable"`
	IsShow            bool   `json:"isShow"`
	EnableAutoUpgrade bool   `json:"enableAutoUpgrade"`
	SdkVersion        string `json:"sdkVersion"`
}

// ProcessorItem 处理人条目。
type ProcessorItem struct {
	AppID        string `json:"appId"`
	PlatformID   int    `json:"platformId"`
	Type         int    `json:"type"`
	UserID       string `json:"userId"`
	NewUserID    string `json:"newUserId"`
	RegisterTime string `json:"registerTime"`
	LogoURL      string `json:"logoUrl"`
	Email        string `json:"email"`
	Phone        string `json:"phone"`
	// IsShow 服务端以字符串 "true"/"false" 返回。
	IsShow       string `json:"isShow"`
	QQNickName   string `json:"qqNickName"`
	Name         string `json:"name"`
	// IsOperator 服务端以整数 0/1 返回。
	IsOperator   int    `json:"isOperator"`
}

// GetSelectorDataParams GetSelectorData 的参数。
type GetSelectorDataParams struct {
	// Types 需要查询的数据类型，逗号分隔，默认 "version,member,bundle,tag,channel"。
	Types string
}

// SelectorDataResponse GetSelectorData 的响应。
type SelectorDataResponse struct {
	VersionList              []VersionItem   `json:"versionList"`
	TagList                  []TagInfo       `json:"tagList"`
	ProcessorList            []ProcessorItem `json:"processorList"`
	DisableMemberInvitation  bool            `json:"disableMemberInvitation"`
	ChannelList              []string        `json:"channelList"`
	BundleIDList             []string        `json:"bundleIdList"`
}

// VersionDateItem 版本首次出现日期条目。
type VersionDateItem struct {
	Version string `json:"version"`
	Date    string `json:"date"`
}
