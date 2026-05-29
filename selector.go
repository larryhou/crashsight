package crashsight

import (
	"context"
	"strconv"
)

// ─────────────────────────────────────────────────────────────────────────────
//  选择器与元数据 API
// ─────────────────────────────────────────────────────────────────────────────

// GetSelectorData 获取版本列表、包名列表、处理人列表、标签列表及渠道列表。
//
// 对应接口: POST /uniform/openapi/getSelectorDatas
//
// p.Types 支持 "version"、"member"、"bundle"、"tag"、"channel" 的逗号组合，
// 默认返回全部类型。
func (c *Client) GetSelectorData(ctx context.Context, p GetSelectorDataParams) (*SelectorDataResponse, error) {
	types := strDefault(p.Types, "version,member,bundle,tag,channel")
	body := map[string]any{
		"appId": c.appID,
		"pid":   strconv.Itoa(int(c.platform)),
		"types": types,
	}

	var out SelectorDataResponse
	if err := c.post(ctx, apiPathPrefix+"/getSelectorDatas", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetVersionDateList 获取版本号与首次出现日期的对应列表。
//
// 对应接口: POST /uniform/openapi/getVersionDateList
//
// 服务端实际返回 3 列: ["dtEventTime", "product_version", "first_date"]，
// 按 columns 字段动态定位 product_version 和 first_date 的索引。
func (c *Client) GetVersionDateList(ctx context.Context) ([]VersionDateItem, error) {
	body := map[string]any{
		"appId":      c.appID,
		"platformId": int(c.platform),
	}

	var raw struct {
		Columns []string   `json:"columns"`
		Values  [][]string `json:"values"`
	}
	if err := c.post(ctx, apiPathPrefix+"/getVersionDateList", body, &raw); err != nil {
		return nil, err
	}

	// 按 columns 动态定位 product_version / first_date 列索引
	versionIdx, dateIdx := -1, -1
	for i, col := range raw.Columns {
		switch col {
		case "product_version", "version":
			versionIdx = i
		case "first_date", "date":
			dateIdx = i
		}
	}
	if versionIdx < 0 || dateIdx < 0 {
		// 兜底：假设两列顺序为 [version, date]
		versionIdx, dateIdx = 0, 1
	}

	items := make([]VersionDateItem, 0, len(raw.Values))
	for _, row := range raw.Values {
		if len(row) > versionIdx && len(row) > dateIdx {
			items = append(items, VersionDateItem{
				Version: row[versionIdx],
				Date:    row[dateIdx],
			})
		}
	}
	return items, nil
}
