package tooldiscovery

import (
	"sort"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/lithammer/fuzzysearch/fuzzy"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type SearchResult struct {
	Tool      mcp.Tool `json:"tool"`
	Score     float64  `json:"score"`
	MatchedIn []string `json:"matchedIn"` // Signals that contributed to scoring (e.g. name:token, description, parameter:token).
}

const (
	DefaultMaxSearchResults = 3

	// Scoring weights used by scoreTool.
	substringMatchScore   = 5
	exactTokensMatchScore = 2.5
	descriptionMatchScore = 2
	prefixMatchScore      = 1.5
	parameterMatchScore   = 1
)

// SearchOptions configures search behavior.
type SearchOptions struct {
	MaxResults int `json:"maxResults"` // Maximum number of results to return (default: 3)
}

// Search returns the most relevant tools for a free-text query.
//
// Prefer using SearchTools and passing an explicit tool list. This function is
// kept for API compatibility and currently searches an empty tool set.
func Search(query string, options ...SearchOptions) ([]SearchResult, error) {
	return SearchTools(nil, query, options...)
}

// SearchTools is like Search, but searches across the provided tool list.
//
// Matching uses a weighted combination of:
//   - tool name matches (strongest)
//   - description matches
//   - input parameter name matches (JSON schema property names)
//   - fuzzy similarity as a tie-breaker
//
// Empty or whitespace-only queries return (nil, nil).
func SearchTools(tools []mcp.Tool, query string, options ...SearchOptions) ([]SearchResult, error) {
	maxResults := getMaxResults(options)

	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}

	queryLower := strings.ToLower(query)
	queryTokens := strings.Fields(queryLower)
	normalizedQueryCompact := strings.ReplaceAll(strings.ReplaceAll(queryLower, " ", ""), "_", "")

	results := make([]SearchResult, 0, len(tools))
	for _, tool := range tools {
		score, matchedIn := scoreTool(tool, queryLower, queryTokens, normalizedQueryCompact)
		results = append(results, SearchResult{
			Tool:      tool,
			Score:     score,
			MatchedIn: matchedIn,
		})
	}

	sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })

	// Filter out low-relevance results
	const minScore = 1.0
	filtered := results[:0]
	for _, r := range results {
		if r.Score > minScore {
			filtered = append(filtered, r)
		}
	}
	results = filtered

	// Limit results
	if len(results) > maxResults {
		results = results[:maxResults]
	}

	return results, nil
}

// scoreTool assigns a relevance score to a tool for the given query.
//
// It combines several signals (substrings, token coverage, and similarity) from:
//   - tool name
//   - tool description
//   - input parameter names (schema property names)
//
// MatchedIn records which signals contributed to the score for debugging/tuning.
func scoreTool(
	tool mcp.Tool,
	queryLower string,
	queryTokens []string,
	normalizedQueryCompact string,
) (score float64, matchedIn []string) {
	nameLower := strings.ToLower(tool.Name)
	descLower := strings.ToLower(tool.Description)

	normalizedNameCompact := strings.ReplaceAll(nameLower, "_", "")
	nameTokens := splitTokens(nameLower)
	propertyNames := lowerInputPropertyNames(tool.InputSchema)

	matches := newMatchTracker(3)
	score = 0.0

	// Strong boosts for direct substring matches
	if strings.Contains(nameLower, queryLower) {
		score += substringMatchScore
		matches.Add("name:substring")
	}
	if strings.HasPrefix(nameLower, queryLower) {
		score += prefixMatchScore
		matches.Add("name:prefix")
	}
	if normalizedNameCompact == normalizedQueryCompact && len(queryTokens) > 1 {
		score += exactTokensMatchScore
		matches.Add("name:exact-tokens")
	}
	if strings.Contains(descLower, queryLower) {
		score += descriptionMatchScore
		matches.Add("description")
	}

	for _, prop := range propertyNames {
		if strings.Contains(prop, queryLower) {
			score += parameterMatchScore
			matches.Add("parameter")
		}
	}

	matchedTokens := make(map[string]struct{})

	// Token-level matches for multi-word queries
	for _, token := range queryTokens {
		if strings.Contains(nameLower, token) {
			score++
			matchedTokens[token] = struct{}{}
			matches.Add("name:token")
		} else if strings.Contains(descLower, token) {
			score += 0.6
			matchedTokens[token] = struct{}{}
			matches.Add("description:token")
		}

		for _, prop := range propertyNames {
			if strings.Contains(prop, token) {
				// Only credit the first parameter match per token to avoid double-counting
				score += 0.4
				matchedTokens[token] = struct{}{}
				matches.Add("parameter:token")
				break
			}
		}
	}

	tokenCoverage := float64(len(matchedTokens))
	score += tokenCoverage * 0.8
	if len(queryTokens) > 1 && len(matchedTokens) == len(queryTokens) {
		score += 2 // bonus when all tokens are matched somewhere
	}

	// Prefer names that cover query tokens directly, with fewer extra tokens
	nameTokenMatches := 0
	for _, qt := range queryTokens {
		for _, nt := range nameTokens {
			if strings.Contains(nt, qt) {
				nameTokenMatches++
				break
			}
		}
	}
	if nameTokenMatches == len(queryTokens) {
		score += 4.0 // all tokens present in name tokens
		if len(nameTokens) == len(queryTokens) {
			score += 2.0 // exact token count match (e.g., issue_write vs sub_issue_write)
		}
	}
	extraTokens := len(nameTokens) - nameTokenMatches
	if extraTokens > 0 {
		score -= float64(extraTokens) * 0.5 // stronger penalty for extra unrelated tokens
	}

	// Similarity scores to soften ordering among close matches
	nameSim := normalizedSimilarity(nameLower, queryLower)
	descSim := normalizedSimilarity(descLower, queryLower)

	var propSim float64
	for _, prop := range propertyNames {
		if sim := normalizedSimilarity(prop, queryLower); sim > propSim {
			propSim = sim
		}
	}

	searchText := nameLower + " " + descLower
	if len(propertyNames) > 0 {
		searchText += " " + strings.Join(propertyNames, " ")
	}
	fuzzySim := normalizedSimilarity(searchText, queryLower)

	score += nameSim * 2
	score += descSim * 0.8
	score += propSim * 0.6
	score += fuzzySim * 0.5

	return score, matches.List()
}

func getMaxResults(options []SearchOptions) int {
	maxResults := DefaultMaxSearchResults
	if len(options) > 0 && options[0].MaxResults > 0 {
		maxResults = options[0].MaxResults
	}
	return maxResults
}

func lowerInputPropertyNames(inputSchema any) []string {
	if inputSchema == nil {
		return nil
	}

	// From the server, this is commonly a *jsonschema.Schema.
	if schema, ok := inputSchema.(*jsonschema.Schema); ok {
		if len(schema.Properties) == 0 {
			return nil
		}
		out := make([]string, 0, len(schema.Properties))
		for prop := range schema.Properties {
			out = append(out, strings.ToLower(prop))
		}
		return out
	}

	// From the client (or when unmarshaled), schemas arrive as map[string]any.
	if schema, ok := inputSchema.(map[string]any); ok {
		propsAny, ok := schema["properties"]
		if !ok {
			return nil
		}
		props, ok := propsAny.(map[string]any)
		if !ok || len(props) == 0 {
			return nil
		}
		out := make([]string, 0, len(props))
		for prop := range props {
			out = append(out, strings.ToLower(prop))
		}
		return out
	}

	return nil
}

type matchTracker struct {
	list []string
	seen map[string]struct{}
}

func newMatchTracker(capacity int) *matchTracker {
	return &matchTracker{
		list: make([]string, 0, capacity),
		seen: make(map[string]struct{}, capacity),
	}
}

func (m *matchTracker) Add(part string) {
	if _, ok := m.seen[part]; ok {
		return
	}
	m.seen[part] = struct{}{}
	m.list = append(m.list, part)
}

func (m *matchTracker) List() []string {
	return m.list
}

func normalizedSimilarity(a, b string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}

	distance := fuzzy.LevenshteinDistance(a, b)
	maxLen := max(len(b), len(a))

	similarity := 1 - (float64(distance) / float64(maxLen))
	if similarity < 0 {
		return 0
	}

	return similarity
}

func splitTokens(s string) []string {
	if s == "" {
		return nil
	}
	return strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == '-' || r == ' '
	})
}
