package github

import (
	"context"
	"encoding/json"
	"strconv"
)

// Metric names for the optional `fields` response-filtering feature. They let a
// dashboard answer two questions on real traffic: how often the model actually
// filters (adoption) and how many bytes that filtering removes (effectiveness).
//
// Cardinality is kept deliberately low: the only tags ever attached are `tool`
// (a small fixed set of tool names) and `filtered` (a boolean). Unbounded values
// such as repository, owner, user, the query, or the requested field list are
// never used as tags.
//
// The realized savings (bytes_full - bytes_sent) is intentionally not emitted as
// its own metric: it is derivable on the dashboard from the two byte counters,
// since sum(bytes_full) - sum(bytes_sent) equals the total saved at any rollup.
const (
	metricFieldsToolCall  = "mcp.fields.tool_call"
	metricFieldsBytesFull = "mcp.fields.bytes_full"
	metricFieldsBytesSent = "mcp.fields.bytes_sent"
)

// recordFieldsUsage emits telemetry for a single call to a tool that supports
// the `fields` parameter. It is best-effort: the local server wires a no-op
// metrics sink, while hosted deployments inject a real sink.
//
// Every call increments mcp.fields.tool_call tagged by tool and whether the
// response was filtered, which yields the adoption rate (filtered / total). When
// the response was filtered, it also records the unfiltered (fullBytes) and
// returned (sentBytes) payload sizes. Byte counters are only emitted for
// filtered calls so that "percent saved" (1 - bytes_sent / bytes_full) is
// computed over the population where filtering actually applied.
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

// recordFieldsUsageFor emits fields telemetry for a tool whose response is a
// list of items (optionally wrapped in a metadata envelope). sentBytes is the
// size of the payload actually returned. When the response was filtered, the
// unfiltered size is computed by marshalling full so the realized savings can be
// measured; full should be the complete, unfiltered payload. It centralizes the
// full-size computation shared by every fields-enabled tool.
func recordFieldsUsageFor(ctx context.Context, deps ToolDependencies, tool string, full any, filtered bool, sentBytes int) {
	fullBytes := sentBytes
	if filtered {
		if data, err := json.Marshal(full); err == nil {
			fullBytes = len(data)
		}
	}
	recordFieldsUsage(ctx, deps, tool, filtered, fullBytes, sentBytes)
}
