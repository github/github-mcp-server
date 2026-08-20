package github

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type recommendedSecurityPolicy struct {
	BlockedTools []string `yaml:"blocked_tools"`
	RateLimits   []struct {
		Tools    []string `yaml:"tools"`
		Category string   `yaml:"category"`
	} `yaml:"rate_limits"`
	ServerConfiguration struct {
		ExcludeTools []string `yaml:"exclude_tools"`
	} `yaml:"server_configuration"`
}

func destructiveToolNames(t *testing.T) []string {
	t.Helper()

	var names []string
	for _, tool := range AllTools(identityTranslationHelper) {
		annotations := tool.Tool.Annotations
		if annotations == nil || annotations.DestructiveHint == nil || !*annotations.DestructiveHint {
			continue
		}
		names = append(names, tool.Tool.Name)
	}

	return names
}

func loadRecommendedSecurityPolicy(t *testing.T) recommendedSecurityPolicy {
	t.Helper()

	policyPath := filepath.Join("..", "..", "docs", "examples", "recommended-security-policy.yaml")
	data, err := os.ReadFile(policyPath)
	require.NoError(t, err, "recommended security policy file should exist")

	var policy recommendedSecurityPolicy
	require.NoError(t, yaml.Unmarshal(data, &policy))

	return policy
}

func allToolNames(t *testing.T) map[string]struct{} {
	t.Helper()

	names := make(map[string]struct{})
	for _, tool := range AllTools(identityTranslationHelper) {
		names[tool.Tool.Name] = struct{}{}
	}

	return names
}

func TestRecommendedSecurityPolicyBlocksDestructiveTools(t *testing.T) {
	policy := loadRecommendedSecurityPolicy(t)
	destructiveTools := destructiveToolNames(t)

	blocked := make(map[string]struct{}, len(policy.BlockedTools))
	for _, name := range policy.BlockedTools {
		blocked[name] = struct{}{}
	}

	for _, name := range destructiveTools {
		_, ok := blocked[name]
		assert.True(t, ok, "blocked_tools should include destructive tool %q", name)
	}

	assert.ElementsMatch(t, destructiveTools, policy.BlockedTools,
		"blocked_tools should match tools annotated with DestructiveHint")
}

func TestRecommendedSecurityPolicyReferencesValidTools(t *testing.T) {
	policy := loadRecommendedSecurityPolicy(t)
	knownTools := allToolNames(t)

	referenced := append([]string{}, policy.BlockedTools...)
	referenced = append(referenced, policy.ServerConfiguration.ExcludeTools...)
	for _, limit := range policy.RateLimits {
		referenced = append(referenced, limit.Tools...)
	}

	for _, name := range referenced {
		if name == "" {
			continue
		}
		_, ok := knownTools[name]
		assert.True(t, ok, "policy references unknown tool %q", name)
	}
}

func TestRecommendedSecurityPolicyExcludeToolsMatchBlockedTools(t *testing.T) {
	policy := loadRecommendedSecurityPolicy(t)

	assert.ElementsMatch(t, policy.BlockedTools, policy.ServerConfiguration.ExcludeTools,
		"server_configuration.exclude_tools should mirror blocked_tools")
}

func identityTranslationHelper(key string, defaultValue string) string {
	if strings.HasPrefix(key, "TOOL_") {
		return defaultValue
	}
	return key
}
