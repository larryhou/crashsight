package crashsight

import "context"

// ─────────────────────────────────────────────────────────────────────────────
//  OOM 分析 API
// ─────────────────────────────────────────────────────────────────────────────

// QueryOOMList 查询 OOM 或非 OOM 崩溃列表。
//
// 对应接口: POST /uniform/openapi/queryOomList
//
// 示例（仅查询 OOM 崩溃）:
//
//	resp, err := client.QueryOOMList(ctx, "appId", crashsight.QueryOOMListParams{
//	    Limit: 20,
//	    SearchConditionGroup: crashsight.SearchConditionGroup{
//	        Conditions: []crashsight.SearchCondition{
//	            {
//	                QueryType: crashsight.QueryTypeTerm,
//	                Term:      string(crashsight.OOMStatusOnlyIsOOM),
//	                Field:     "oomStatus",
//	            },
//	        },
//	    },
//	})
func (c *Client) QueryOOMList(ctx context.Context, appID string, p QueryOOMListParams) (*OOMListResponse, error) {
	limit := intDefault(p.Limit, 10)
	body := map[string]any{
		"appId": appID,
		"limit": limit,
		"search": map[string]any{
			"searchConditionGroup": p.SearchConditionGroup,
		},
	}

	var out OOMListResponse
	if err := c.post(ctx, apiPathPrefix+"/queryOomList", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
