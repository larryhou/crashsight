// Package crashsight 提供 CrashSight OpenAPI 的 Go SDK。
//
// 用法示例:
//
//	client := crashsight.NewClient("your_user_id", "your_api_key",
//	    crashsight.WithRegion(crashsight.RegionCN),
//	)
//	ctx := context.Background()
//	items, err := client.GetTrend(ctx, "appId", crashsight.PlatformPC,
//	    crashsight.GetTrendParams{
//	        StartDate: "20260301",
//	        EndDate:   "20260327",
//	    },
//	)
package crashsight

import "fmt"

// Platform 平台类型。
type Platform int

const (
	PlatformAndroid Platform = 1
	PlatformIOS     Platform = 2
	PlatformPC      Platform = 10
)

// String 返回平台的可读名称。
func (p Platform) String() string {
	switch p {
	case PlatformAndroid:
		return "Android"
	case PlatformIOS:
		return "iOS"
	case PlatformPC:
		return "PC"
	default:
		return fmt.Sprintf("Platform(%d)", int(p))
	}
}

// CrashType 异常类型。
type CrashType string

const (
	CrashTypeCrash CrashType = "crash"
	CrashTypeANR   CrashType = "anr"
	CrashTypeError CrashType = "error"
)

// VmType 设备类型过滤。
type VmType int

const (
	VmTypeAll        VmType = 0 // 全部设备
	VmTypeRealDevice VmType = 1 // 真机
	VmTypeEmulator   VmType = 2 // 模拟器
)

// IssueStatus 问题状态。
type IssueStatus int

const (
	IssueStatusUnresolved IssueStatus = 0 // 未处理
	IssueStatusResolved   IssueStatus = 1 // 已处理
	IssueStatusResolving  IssueStatus = 2 // 处理中
)

// String 返回状态的可读名称。
func (s IssueStatus) String() string {
	switch s {
	case IssueStatusUnresolved:
		return "Unresolved"
	case IssueStatusResolved:
		return "Resolved"
	case IssueStatusResolving:
		return "Resolving"
	default:
		return fmt.Sprintf("IssueStatus(%d)", int(s))
	}
}

// Region 部署区域。
type Region string

const (
	// RegionCN 国内节点（默认）。
	RegionCN Region = "cn"
	// RegionSG 新加坡节点。
	RegionSG Region = "sg"
)

// BaseURL 返回对应区域的基础 URL。
func (r Region) BaseURL() string {
	switch r {
	case RegionSG:
		return "https://crashsight.wetest.net"
	default:
		return "https://crashsight.qq.com"
	}
}

// GranularityUnit 趋势数据聚合粒度。
type GranularityUnit string

const (
	GranularityDay  GranularityUnit = "DAY"
	GranularityHour GranularityUnit = "HOUR"
)

// ExceptionTypeList 常用异常类型组合（用于 issue 列表过滤）。
const (
	ExceptionTypeCrash  = "Crash,Native,ExtensionCrash"
	ExceptionTypeANR    = "ANR"
	ExceptionTypeError  = "AllCatched,Unity3D,Lua,JS"
	ExceptionTypeAll    = "AllCrash,ANR,AllCatched"
)

// TopIssueDataType TOP 问题数据类型。
type TopIssueDataType string

const (
	TopIssueDataTypeSystemExit   TopIssueDataType = "SystemExit"
	TopIssueDataTypeUnSystemExit TopIssueDataType = "unSystemExit"
)

// QueryType OOM/高级搜索条件查询类型。
type QueryType string

const (
	QueryTypeTerm                 QueryType = "TERM"
	QueryTypeTerms                QueryType = "TERMS"
	QueryTypeTermsWildcard        QueryType = "TERMS_WILDCARD"
	QueryTypeRangeRelativeDatetime QueryType = "RANGE_RELATIVE_DATETIME"
)

// OOMStatus OOM 状态过滤值。
type OOMStatus string

const (
	OOMStatusOnlyIsOOM    OOMStatus = "ONLY_IS_OOM"
	OOMStatusOnlyNotIsOOM OOMStatus = "ONLY_NOT_IS_OOM"
)
