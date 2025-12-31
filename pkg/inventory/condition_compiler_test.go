package inventory

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConditionCompiler_AssignBit(t *testing.T) {
	cc := NewConditionCompiler()

	// First assignment
	bit1 := cc.assignBit("ctx:is_cca")
	assert.Equal(t, uint8(0), bit1)

	// Same key returns same bit
	bit2 := cc.assignBit("ctx:is_cca")
	assert.Equal(t, bit1, bit2)

	// Different key gets new bit
	bit3 := cc.assignBit("ff:web_search")
	assert.Equal(t, uint8(1), bit3)

	// After freeze, unknown keys get bit 63
	cc.Freeze()
	bit4 := cc.assignBit("ctx:unknown")
	assert.Equal(t, uint8(63), bit4)

	// Known keys still work after freeze
	bit5 := cc.assignBit("ctx:is_cca")
	assert.Equal(t, bit1, bit5)
}

func TestConditionCompiler_CompileStatic(t *testing.T) {
	cc := NewConditionCompiler()

	// Static true
	cond := cc.Compile(Always())
	require.NotNil(t, cond)
	assert.Equal(t, evalStatic, cond.evalType)
	assert.True(t, cond.static)

	// Static false
	cond = cc.Compile(Never())
	require.NotNil(t, cond)
	assert.Equal(t, evalStatic, cond.evalType)
	assert.False(t, cond.static)

	// Nil returns nil
	assert.Nil(t, cc.Compile(nil))
}

func TestConditionCompiler_CompileContextBool(t *testing.T) {
	cc := NewConditionCompiler()

	cond := cc.Compile(ContextBool("is_cca"))
	require.NotNil(t, cond)
	assert.Equal(t, evalBitCheck, cond.evalType)
	assert.Equal(t, uint8(0), cond.requiredBit)
	assert.True(t, cond.requireTrue)
}

func TestConditionCompiler_CompileFeatureFlag(t *testing.T) {
	cc := NewConditionCompiler()

	cond := cc.Compile(FeatureFlag("web_search"))
	require.NotNil(t, cond)
	assert.Equal(t, evalBitCheck, cond.evalType)
	assert.Equal(t, uint8(0), cond.requiredBit)
	assert.True(t, cond.requireTrue)
}

func TestConditionCompiler_CompileNot(t *testing.T) {
	cc := NewConditionCompiler()

	// Not(static) -> static
	cond := cc.Compile(Not(Always()))
	require.NotNil(t, cond)
	assert.Equal(t, evalStatic, cond.evalType)
	assert.False(t, cond.static)

	// Not(contextBool) -> bitCheck with requireTrue=false
	cond = cc.Compile(Not(ContextBool("is_cca")))
	require.NotNil(t, cond)
	assert.Equal(t, evalBitCheck, cond.evalType)
	assert.False(t, cond.requireTrue)
}

func TestConditionCompiler_CompileAnd(t *testing.T) {
	cc := NewConditionCompiler()

	// And of two context bools -> bitmaskAnd
	cond := cc.Compile(And(
		ContextBool("is_cca"),
		ContextBool("has_access"),
	))
	require.NotNil(t, cond)
	assert.Equal(t, evalBitmaskAnd, cond.evalType)
	assert.Equal(t, uint64(0b11), cond.bitmask) // bits 0 and 1

	// And with static false -> static false
	cond = cc.Compile(And(
		ContextBool("is_cca"),
		Never(),
	))
	require.NotNil(t, cond)
	assert.Equal(t, evalStatic, cond.evalType)
	assert.False(t, cond.static)

	// And with static true filtered out -> single condition
	cc2 := NewConditionCompiler()
	cond = cc2.Compile(And(
		Always(),
		ContextBool("is_cca"),
	))
	require.NotNil(t, cond)
	assert.Equal(t, evalBitCheck, cond.evalType)
}

func TestConditionCompiler_CompileOr(t *testing.T) {
	cc := NewConditionCompiler()

	// Or of two context bools -> bitmaskOr
	cond := cc.Compile(Or(
		ContextBool("is_cca"),
		ContextBool("is_bypass"),
	))
	require.NotNil(t, cond)
	assert.Equal(t, evalBitmaskOr, cond.evalType)
	assert.Equal(t, uint64(0b11), cond.bitmask)

	// Or with static true -> static true
	cond = cc.Compile(Or(
		ContextBool("is_cca"),
		Always(),
	))
	require.NotNil(t, cond)
	assert.Equal(t, evalStatic, cond.evalType)
	assert.True(t, cond.static)
}

func TestCompiledCondition_Evaluate(t *testing.T) {
	cc := NewConditionCompiler()

	// Compile conditions
	ccaCheck := cc.Compile(ContextBool("is_cca"))
	ffCheck := cc.Compile(FeatureFlag("web_search"))
	andCond := cc.Compile(And(ContextBool("is_cca"), FeatureFlag("web_search")))
	orCond := cc.Compile(Or(ContextBool("is_cca"), FeatureFlag("web_search")))
	notCond := cc.Compile(Not(ContextBool("is_cca")))

	cc.Freeze()

	builder := NewRequestMaskBuilder(cc)

	tests := []struct {
		name      string
		condition *CompiledCondition
		bools     ContextBools
		flags     map[string]bool
		want      bool
	}{
		{
			name:      "context bool true",
			condition: ccaCheck,
			bools:     ContextBools{"is_cca": true},
			want:      true,
		},
		{
			name:      "context bool false",
			condition: ccaCheck,
			bools:     ContextBools{"is_cca": false},
			want:      false,
		},
		{
			name:      "feature flag true",
			condition: ffCheck,
			flags:     map[string]bool{"web_search": true},
			want:      true,
		},
		{
			name:      "feature flag false",
			condition: ffCheck,
			flags:     map[string]bool{"web_search": false},
			want:      false,
		},
		{
			name:      "and both true",
			condition: andCond,
			bools:     ContextBools{"is_cca": true},
			flags:     map[string]bool{"web_search": true},
			want:      true,
		},
		{
			name:      "and one false",
			condition: andCond,
			bools:     ContextBools{"is_cca": true},
			flags:     map[string]bool{"web_search": false},
			want:      false,
		},
		{
			name:      "or one true",
			condition: orCond,
			bools:     ContextBools{"is_cca": true},
			flags:     map[string]bool{"web_search": false},
			want:      true,
		},
		{
			name:      "or both false",
			condition: orCond,
			bools:     ContextBools{"is_cca": false},
			flags:     map[string]bool{"web_search": false},
			want:      false,
		},
		{
			name:      "not true -> false",
			condition: notCond,
			bools:     ContextBools{"is_cca": true},
			want:      false,
		},
		{
			name:      "not false -> true",
			condition: notCond,
			bools:     ContextBools{"is_cca": false},
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mask := builder.Build(context.Background(), tt.bools, tt.flags)
			got, err := tt.condition.Evaluate(mask)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestToolConditionSet_FilterEnabled(t *testing.T) {
	tools := []*ServerTool{
		{
			Tool:            mcp.Tool{Name: "always_on"},
			EnableCondition: nil, // nil means always enabled
		},
		{
			Tool:            mcp.Tool{Name: "cca_only"},
			EnableCondition: ContextBool("is_cca"),
		},
		{
			Tool:            mcp.Tool{Name: "ff_required"},
			EnableCondition: FeatureFlag("web_search"),
		},
		{
			Tool: mcp.Tool{Name: "cca_and_ff"},
			EnableCondition: And(
				ContextBool("is_cca"),
				FeatureFlag("code_search"),
			),
		},
		{
			Tool: mcp.Tool{Name: "cca_or_ff"},
			EnableCondition: Or(
				ContextBool("is_cca"),
				FeatureFlag("bypass_flag"),
			),
		},
	}

	tcs := NewToolConditionSet(tools)

	tests := []struct {
		name  string
		bools ContextBools
		flags map[string]bool
		want  []string
	}{
		{
			name:  "no bools or flags - only always_on",
			bools: nil,
			flags: nil,
			want:  []string{"always_on"},
		},
		{
			name:  "cca only",
			bools: ContextBools{"is_cca": true},
			flags: nil,
			want:  []string{"always_on", "cca_only", "cca_or_ff"},
		},
		{
			name:  "web_search flag only",
			bools: nil,
			flags: map[string]bool{"web_search": true},
			want:  []string{"always_on", "ff_required"},
		},
		{
			name:  "cca and code_search",
			bools: ContextBools{"is_cca": true},
			flags: map[string]bool{"code_search": true},
			want:  []string{"always_on", "cca_only", "cca_and_ff", "cca_or_ff"},
		},
		{
			name:  "all enabled",
			bools: ContextBools{"is_cca": true},
			flags: map[string]bool{"web_search": true, "code_search": true, "bypass_flag": true},
			want:  []string{"always_on", "cca_only", "ff_required", "cca_and_ff", "cca_or_ff"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mask := tcs.BuildMask(context.Background(), tt.bools, tt.flags)
			enabled := tcs.FilterEnabled(mask)

			names := make([]string, len(enabled))
			for i, tool := range enabled {
				names[i] = tool.Tool.Name
			}
			assert.Equal(t, tt.want, names)
		})
	}
}

func TestToolConditionSet_ComplexConditions(t *testing.T) {
	// Test complex real-world patterns from remote server
	tools := []*ServerTool{
		{
			// CCA bypass: CCA OR feature_flag
			Tool: mcp.Tool{Name: "cca_bypass"},
			EnableCondition: Or(
				ContextBool("is_cca"),
				FeatureFlag("agent_search"),
			),
		},
		{
			// Feature + policy: feature AND paid_access
			Tool: mcp.Tool{Name: "paid_feature"},
			EnableCondition: And(
				FeatureFlag("premium_search"),
				ContextBool("has_paid_access"),
			),
		},
		{
			// Complex: (CCA OR copilot_chat) AND feature AND NOT disabled
			Tool: mcp.Tool{Name: "complex"},
			EnableCondition: And(
				Or(
					ContextBool("is_cca"),
					ContextBool("is_copilot_chat"),
				),
				FeatureFlag("advanced_feature"),
				Not(FeatureFlag("kill_switch")),
			),
		},
	}

	tcs := NewToolConditionSet(tools)

	tests := []struct {
		name  string
		bools ContextBools
		flags map[string]bool
		want  []string
	}{
		{
			name:  "cca enables cca_bypass",
			bools: ContextBools{"is_cca": true},
			flags: nil,
			want:  []string{"cca_bypass"},
		},
		{
			name:  "agent_search flag enables cca_bypass",
			bools: nil,
			flags: map[string]bool{"agent_search": true},
			want:  []string{"cca_bypass"},
		},
		{
			name:  "premium + paid enables paid_feature",
			bools: ContextBools{"has_paid_access": true},
			flags: map[string]bool{"premium_search": true},
			want:  []string{"paid_feature"},
		},
		{
			name:  "complex enabled with cca + feature",
			bools: ContextBools{"is_cca": true},
			flags: map[string]bool{"advanced_feature": true},
			want:  []string{"cca_bypass", "complex"},
		},
		{
			name:  "complex disabled by kill_switch",
			bools: ContextBools{"is_cca": true},
			flags: map[string]bool{"advanced_feature": true, "kill_switch": true},
			want:  []string{"cca_bypass"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mask := tcs.BuildMask(context.Background(), tt.bools, tt.flags)
			enabled := tcs.FilterEnabled(mask)

			names := make([]string, len(enabled))
			for i, tool := range enabled {
				names[i] = tool.Tool.Name
			}
			assert.Equal(t, tt.want, names)
		})
	}
}

func TestConditionCompiler_NumBits(t *testing.T) {
	cc := NewConditionCompiler()

	assert.Equal(t, 0, cc.NumBits())

	cc.assignBit("ctx:a")
	assert.Equal(t, 1, cc.NumBits())

	cc.assignBit("ctx:b")
	cc.assignBit("ff:c")
	assert.Equal(t, 3, cc.NumBits())

	// Same key doesn't increase count
	cc.assignBit("ctx:a")
	assert.Equal(t, 3, cc.NumBits())
}

func TestConditionCompiler_Keys(t *testing.T) {
	cc := NewConditionCompiler()

	cc.assignBit("ctx:is_cca")
	cc.assignBit("ff:web_search")
	cc.assignBit("ctx:has_access")

	keys := cc.Keys()
	assert.Len(t, keys, 3)
	assert.Contains(t, keys, "ctx:is_cca")
	assert.Contains(t, keys, "ff:web_search")
	assert.Contains(t, keys, "ctx:has_access")
}
