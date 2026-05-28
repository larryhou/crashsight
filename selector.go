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
func (c *Client) GetSelectorData(ctx context.Context, appID string, platform Platform, p GetSelectorDataParams) (*SelectorDataResponse, error) {
	types := strDefault(p.Types, "version,member,bundle,tag,channel")
	body := map[string]any{
		"appId": appID,
		"pid":   strconv.Itoa(int(platform)),
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
func (c *Client) GetVersionDateList(ctx context.Context, appID string, platform Platform) ([]VersionDateItem, error) {
	body := map[string]any{
		"appId":      appID,
		"platformId": int(platform),
	}

	// 该接口返回 {"columns": ["version","date"], "values": [["1.0","20260101"], ...]}
	// 需要手动解包 values 列表为 VersionDateItem
	var raw struct {
		Columns []string   `json:"columns"`
		Values  [][]string `json:"values"`
	}
	if err := c.post(ctx, apiPathPrefix+"/getVersionDateList", body, &raw); err != nil {
		return nil, err
	}

	items := make([]VersionDateItem, 0, len(raw.Values))
	for _, row := range raw.Values {
		if len(row) >= 2 {
			items = append(items, VersionDateItem{
				Version: row[0],
				Date:    row[1],
			})
		}
	}
	return items, nil
}
