package github

import (
	"context"
	"encoding/json"
	"strconv"
)

// Metric names 用于可选 `fields` 响应-筛选ing feature. They let a
// dashboard answer two questions on real traffic: how often model actually
// 筛选s (adoption) 和how many bytes that 筛选ing removes (effectiveness).
//
// Cardinality is kept deliberately low: 仅tags ever attached are `工具`
// (a sm所有fixed set of 工具 names) 和`筛选ed` (a boolean). Unbounded 值
// such as 仓库, owner, user, query, 或请求ed field 列出 are
// 绝不used as tags.
//
// realized savings (bytes_full - bytes_sent) is intentionally 不emitted as
// its own metric: it is derivable 在dashboard 来自two byte counters,
// since sum(bytes_full) - sum(bytes_sent) equals total saved at any rollup.
const (
	metricFieldsToolCall  = "mcp.fields.tool_call"
	metricFieldsBytesFull = "mcp.fields.bytes_full"
	metricFieldsBytesSent = "mcp.fields.bytes_sent"
)

// recordFieldsUsage emits telemetry f或一个单个 c所有to 一个工具 that supports
// `fields` 参数. It is best-effort: local 服务器 wires 一个no-op
// metrics sink, 当hosted deployments inject 一个real sink.
//
// 每个c所有increments mcp.fields.工具_c所有tagged by 工具 和whether the
// 响应 was 筛选ed, which yields adoption rate (筛选ed / total). When
// 响应 was 筛选ed, it 也records un筛选ed (fullBytes) and
// 返回ed (sentBytes) payload sizes. Byte counters are 仅emitted for
// 筛选ed 调用 so that "percent saved" (1 - bytes_sent / bytes_full) is
// computed over population where 筛选ing actually applied.
func recordFieldsUsage(ctx context.Context, deps ToolDependencies, tool string, filtered bool, fullBytes, sentBytes int) {
	m := deps.Metrics(ctx)
	if m == nil {
		return
	}

	m.Increment(metricFieldsToolCall, map[string]string{
		"tool":     tool,
		"filtered": strconv.FormatBool(filtered),
	})

	if !filtered {
		return
	}

	toolTag := map[string]string{"tool": tool}
	m.Counter(metricFieldsBytesFull, toolTag, int64(fullBytes))
	m.Counter(metricFieldsBytesSent, toolTag, int64(sentBytes))
}

// recordFieldsUsageF或emits fields telemetry f或一个工具 whose 响应 is a
// 列出 of items (可选ly wrapped in 一个元数据 envelope). sentBytes is the
// size 的payload actually 返回ed. 当响应 was 筛选ed, the
// un筛选ed size is computed by marshalling full so realized savings 可以
// measured; full 应当是 complete, un筛选ed payload. It centralizes the
// full-size computation shared by every fields-启用 工具.
func recordFieldsUsageFor(ctx context.Context, deps ToolDependencies, tool string, full any, filtered bool, sentBytes int) {
	fullBytes := sentBytes
	if filtered {
		if data, err := json.Marshal(full); err == nil {
			fullBytes = len(data)
		}
	}
	recordFieldsUsage(ctx, deps, tool, filtered, fullBytes, sentBytes)
}
