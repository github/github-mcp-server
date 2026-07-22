package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v89/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
)

// Unknown outcomes cannot be attributed or retried safely because the pinned
// client drops errors[].path.
type batchItemStatus string

const (
	batchItemSucceeded batchItemStatus = "succeeded"
	batchItemFailed    batchItemStatus = "failed"
	batchItemUnknown   batchItemStatus = "unknown"
)

type batchItemResult struct {
	Index  int                `json:"index"`
	Status batchItemStatus    `json:"status"`
	Item   *batchItemIdentity `json:"item,omitempty"`
	Error  *batchItemError    `json:"error,omitempty"`
	// Ref preserves the request identity when resolution fails.
	Ref map[string]any `json:"ref,omitempty"`
}

type batchItemIdentity struct {
	NodeID         string `json:"node_id,omitempty"`
	FullDatabaseID string `json:"full_database_id,omitempty"`
	ItemID         int64  `json:"item_id,omitempty"`
}

type batchItemError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Candidates []any  `json:"candidates,omitempty"`
	Hint       string `json:"hint,omitempty"`
}

type resolvedBatchItem struct {
	index          int
	ref            map[string]any
	nodeID         string
	fullDatabaseID int64
}

type batchWriteOperation struct {
	gqlClient *githubv4.Client
	kind      batchMutationKind
	projectID githubv4.ID
	fieldID   githubv4.ID
	value     githubv4.ProjectV2FieldValue
}

func updateProjectItemsBatch(ctx context.Context, client *github.Client, gqlClient *githubv4.Client, owner, ownerType string, projectNumber int, args map[string]any) (*mcp.CallToolResult, any, error) {
	rawItems, exists := args["items"]
	if !exists {
		return utils.NewToolResultError("missing required parameter: items"), nil, nil
	}
	itemsRaw, ok := rawItems.([]any)
	if !ok {
		return utils.NewToolResultError("items must be an array"), nil, nil
	}
	if len(itemsRaw) == 0 {
		return utils.NewToolResultError("items must contain at least one entry"), nil, nil
	}
	if len(itemsRaw) > maxProjectItemsPerBatch {
		return utils.NewToolResultError(fmt.Sprintf("items exceeds maximum of %d entries per call (got %d)", maxProjectItemsPerBatch, len(itemsRaw))), nil, nil
	}

	rawField, hasField := args["updated_field"]
	if !hasField {
		return utils.NewToolResultError("missing required parameter: updated_field"), nil, nil
	}
	fieldSpec, fieldSpecErr := parseBatchFieldSpec(rawField)
	if fieldSpecErr != nil {
		return utils.NewToolResultError(fieldSpecErr.Error()), nil, nil
	}

	if gqlClient == nil {
		return utils.NewToolResultError("internal error: gqlClient is required for update_project_items"), nil, nil
	}

	parsed := make([]parsedBatchItem, len(itemsRaw))
	for i, raw := range itemsRaw {
		parsed[i] = parseBatchItemEntry(i, raw)
	}

	results := make([]batchItemResult, len(itemsRaw))
	pending := 0
	for i, p := range parsed {
		if p.err != nil {
			results[i] = batchItemResult{Index: i, Status: batchItemFailed, Ref: p.ref, Error: p.err}
		} else {
			pending++
		}
	}
	if pending == 0 {
		return newUpdateProjectItemsResult(results)
	}

	projectID, err := resolveProjectNodeID(ctx, gqlClient, owner, ownerType, projectNumber)
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}

	field, fieldErr := resolveBatchProjectField(ctx, gqlClient, owner, ownerType, projectNumber, fieldSpec)
	if fieldErr != nil {
		return batchTopLevelError(fieldErr), nil, nil
	}

	kind := batchMutationUpdate
	var value githubv4.ProjectV2FieldValue
	if fieldSpec.value == nil {
		kind = batchMutationClear
	} else {
		value, fieldErr = convertProjectFieldValue(field, fieldSpec.value)
		if fieldErr != nil {
			return batchTopLevelError(fieldErr), nil, nil
		}
	}

	var numericIDs []int64
	for _, p := range parsed {
		if p.err == nil && p.refKind == batchRefItemID {
			numericIDs = append(numericIDs, p.itemID)
		}
	}
	itemIDLookups := resolveItemNodeIDsByNumericID(ctx, client, owner, ownerType, projectNumber, numericIDs)

	issueLookups := resolveIssueRefs(ctx, gqlClient, projectID, parsed)

	var work []resolvedBatchItem
	seenTargets := make(map[string]int)

	for i, p := range parsed {
		if p.err != nil {
			continue
		}

		nodeID, fullDatabaseID, lookupErr := resolveItemReference(p, itemIDLookups, issueLookups)
		if lookupErr != nil {
			results[i] = batchItemResult{Index: i, Status: batchItemFailed, Ref: p.ref, Error: batchErrorFromResolution(lookupErr)}
			continue
		}

		if firstIndex, dup := seenTargets[nodeID]; dup {
			results[i] = batchItemResult{
				Index: i, Status: batchItemFailed, Ref: p.ref,
				Error: &batchItemError{
					Code:    "duplicate_target",
					Message: fmt.Sprintf("items[%d] targets the same project item as items[%d]; each item may only be written once per call", i, firstIndex),
				},
			}
			continue
		}

		seenTargets[nodeID] = i
		work = append(work, resolvedBatchItem{index: i, ref: p.ref, nodeID: nodeID, fullDatabaseID: fullDatabaseID})
	}

	executeBatchWrites(ctx, batchWriteOperation{
		gqlClient: gqlClient,
		kind:      kind,
		projectID: projectID,
		fieldID:   githubv4.ID(field.NodeID),
		value:     value,
	}, work, results)

	return newUpdateProjectItemsResult(results)
}

func batchTopLevelError(err error) *mcp.CallToolResult {
	var structured *ghErrors.StructuredResolutionError
	if errors.As(err, &structured) {
		return ghErrors.NewStructuredResolutionErrorResponse(structured)
	}
	return utils.NewToolResultError(err.Error())
}

func newUpdateProjectItemsResult(results []batchItemResult) (*mcp.CallToolResult, any, error) {
	succeeded, failed, unknown := 0, 0, 0
	for _, r := range results {
		switch r.Status {
		case batchItemSucceeded:
			succeeded++
		case batchItemUnknown:
			unknown++
		default:
			failed++
		}
	}

	response := map[string]any{
		"total":     len(results),
		"succeeded": succeeded,
		"failed":    failed,
		"unknown":   unknown,
		"results":   results,
	}
	r, err := json.Marshal(response)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	result := utils.NewToolResultText(string(r))
	if succeeded == 0 {
		result.IsError = true
	}
	return result, nil, nil
}

func resolveItemReference(p parsedBatchItem, itemIDLookups map[int64]itemLookupResult, issueLookups map[issueRefKey]itemLookupResult) (nodeID string, fullDatabaseID int64, err error) {
	switch p.refKind {
	case batchRefNodeID:
		return p.nodeID, 0, nil
	case batchRefItemID:
		lookup := itemIDLookups[p.itemID]
		if lookup.err != nil {
			return "", 0, lookup.err
		}
		return lookup.nodeID, p.itemID, nil
	case batchRefIssue:
		key := issueRefKey{owner: p.issueOwner, repo: p.issueRepo, number: p.issueNumber}
		lookup := issueLookups[key]
		if lookup.err != nil {
			return "", 0, lookup.err
		}
		return lookup.nodeID, lookup.fullDatabaseID, nil
	default:
		return "", 0, fmt.Errorf("internal error: unrecognised item reference kind")
	}
}

// Transport, cancellation, or incomplete-data ambiguity stops later chunks;
// GraphQL response errors do not because populated aliases still confirm writes.
func executeBatchWrites(ctx context.Context, operation batchWriteOperation, items []resolvedBatchItem, results []batchItemResult) {
	for start := 0; start < len(items); start += batchMutationWireChunkSize {
		if ctx.Err() != nil {
			markChunkUnknown(items[start:], results, ctx.Err())
			return
		}

		end := min(start+batchMutationWireChunkSize, len(items))
		chunk := items[start:end]

		inputs := make([]githubv4.Input, len(chunk))
		for i, item := range chunk {
			if operation.kind == batchMutationClear {
				inputs[i] = githubv4.ClearProjectV2ItemFieldValueInput{
					ProjectID: operation.projectID,
					ItemID:    githubv4.ID(item.nodeID),
					FieldID:   operation.fieldID,
				}
			} else {
				inputs[i] = githubv4.UpdateProjectV2ItemFieldValueInput{
					ProjectID: operation.projectID,
					ItemID:    githubv4.ID(item.nodeID),
					FieldID:   operation.fieldID,
					Value:     operation.value,
				}
			}
		}

		outcomes, mutateErr := executeAliasedMutation(ctx, operation.gqlClient, operation.kind, inputs)

		populated := 0
		for i, oc := range outcomes {
			if oc.Populated {
				populated++
				results[chunk[i].index] = batchItemResult{
					Index:  chunk[i].index,
					Status: batchItemSucceeded,
					Ref:    chunk[i].ref,
					Item: &batchItemIdentity{
						NodeID:         oc.NodeID,
						FullDatabaseID: oc.FullDatabaseID,
						ItemID:         chunk[i].fullDatabaseID,
					},
				}
			}
		}

		if isGraphQLResponseError(mutateErr) {
			markUnpopulatedUnknown(chunk, outcomes, results, mutateErr)
			continue
		}

		if mutateErr != nil {
			markChunkUnknown(items[start:], results, mutateErr)
			return
		}

		if populated != len(chunk) {
			markChunkUnknown(items[start:], results, fmt.Errorf("mutation response did not include every item"))
			return
		}
	}
}

func markUnpopulatedUnknown(chunk []resolvedBatchItem, outcomes []mutationAliasOutcome, results []batchItemResult, err error) {
	for i, oc := range outcomes {
		if oc.Populated {
			continue
		}
		results[chunk[i].index] = batchItemResult{
			Index:  chunk[i].index,
			Status: batchItemUnknown,
			Ref:    chunk[i].ref,
			Error:  &batchItemError{Code: "mutation_unconfirmed", Message: err.Error()},
		}
	}
}

func markChunkUnknown(chunk []resolvedBatchItem, results []batchItemResult, err error) {
	for _, item := range chunk {
		if results[item.index].Status == batchItemSucceeded {
			continue
		}
		results[item.index] = batchItemResult{
			Index:  item.index,
			Status: batchItemUnknown,
			Ref:    item.ref,
			Error:  &batchItemError{Code: "mutation_unconfirmed", Message: err.Error()},
		}
	}
}

const batchItemLookupConcurrency = 5

type batchItemRefKind int

const (
	batchRefNodeID batchItemRefKind = iota
	batchRefItemID
	batchRefIssue
)

type parsedBatchItem struct {
	index   int
	ref     map[string]any
	refKind batchItemRefKind

	nodeID string
	itemID int64

	issueOwner  string
	issueRepo   string
	issueNumber int

	err *batchItemError
}

func parseBatchItemEntry(index int, raw any) parsedBatchItem {
	p := parsedBatchItem{index: index}

	entry, ok := raw.(map[string]any)
	if !ok || entry == nil {
		p.err = &batchItemError{Code: "invalid_item", Message: fmt.Sprintf("items[%d] must be an object", index)}
		return p
	}
	p.ref = itemRefEcho(entry)

	if _, hasUpdatedField := entry["updated_field"]; hasUpdatedField {
		p.err = &batchItemError{Code: "invalid_item", Message: fmt.Sprintf("items[%d].updated_field is not supported; use the top-level updated_field", index)}
		return p
	}

	if refErr := p.parseItemRef(entry); refErr != nil {
		p.err = &batchItemError{Code: "invalid_item_ref", Message: refErr.Error()}
	}
	return p
}

func (p *parsedBatchItem) parseItemRef(entry map[string]any) error {
	_, hasNodeID := entry["node_id"]
	_, hasItemID := entry["item_id"]
	_, hasOwner := entry["item_owner"]
	_, hasRepo := entry["item_repo"]
	_, hasIssueNumber := entry["issue_number"]
	hasIssueRef := hasOwner || hasRepo || hasIssueNumber

	formsPresent := 0
	if hasNodeID {
		formsPresent++
	}
	if hasItemID {
		formsPresent++
	}
	if hasIssueRef {
		formsPresent++
	}

	switch {
	case formsPresent == 0:
		return fmt.Errorf("each item requires exactly one of node_id, item_id, or item_owner + item_repo + issue_number")
	case formsPresent > 1:
		return fmt.Errorf("each item must set exactly one of node_id, item_id, or item_owner + item_repo + issue_number, not more than one")
	}

	switch {
	case hasNodeID:
		s, ok := entry["node_id"].(string)
		if !ok || s == "" {
			return fmt.Errorf("node_id must be a non-empty string")
		}
		p.refKind = batchRefNodeID
		p.nodeID = s
	case hasItemID:
		id, err := validatePositiveInt64(entry["item_id"])
		if err != nil {
			return fmt.Errorf("item_id: %w", err)
		}
		p.refKind = batchRefItemID
		p.itemID = id
	default:
		issueOwner, ownerErr := stringFromEntry(entry, "item_owner")
		issueRepo, repoErr := stringFromEntry(entry, "item_repo")
		issueNumber, numErr := intFromEntry(entry, "issue_number")
		for _, err := range []error{ownerErr, repoErr, numErr} {
			if err != nil {
				return fmt.Errorf("item_owner, item_repo, and issue_number must all be provided together: %w", err)
			}
		}
		p.refKind = batchRefIssue
		p.issueOwner = issueOwner
		p.issueRepo = issueRepo
		p.issueNumber = issueNumber
	}
	return nil
}

func itemRefEcho(entry map[string]any) map[string]any {
	ref := map[string]any{}
	for _, key := range []string{"node_id", "item_id", "item_owner", "item_repo", "issue_number"} {
		if v, ok := entry[key]; ok {
			ref[key] = v
		}
	}
	if len(ref) == 0 {
		return nil
	}
	return ref
}

func stringFromEntry(entry map[string]any, key string) (string, error) {
	v, ok := entry[key]
	if !ok {
		return "", fmt.Errorf("missing %s", key)
	}
	s, ok := v.(string)
	if !ok || s == "" {
		return "", fmt.Errorf("%s must be a non-empty string", key)
	}
	return s, nil
}

func intFromEntry(entry map[string]any, key string) (int, error) {
	v, ok := entry[key]
	if !ok {
		return 0, fmt.Errorf("missing %s", key)
	}
	n, err := validatePositiveInt64(v)
	if err != nil {
		return 0, fmt.Errorf("%s must be a positive integer: %w", key, err)
	}
	if n > math.MaxInt32 {
		return 0, fmt.Errorf("%s exceeds the GraphQL Int maximum of %d", key, int64(math.MaxInt32))
	}
	return int(n), nil
}

func validatePositiveInt64(value any) (int64, error) {
	n, err := validateAndConvertToInt64(value)
	if err != nil {
		return 0, err
	}
	if n <= 0 {
		return 0, fmt.Errorf("value must be greater than zero (got %d)", n)
	}
	return n, nil
}

type batchFieldSpec struct {
	id    int64
	name  string
	value any
}

func parseBatchFieldSpec(raw any) (batchFieldSpec, error) {
	var spec batchFieldSpec
	input, ok := raw.(map[string]any)
	if !ok || input == nil {
		return spec, fmt.Errorf("updated_field must be an object")
	}

	value, hasValue := input["value"]
	if !hasValue {
		return spec, fmt.Errorf("updated_field.value is required")
	}
	spec.value = value

	idField, hasID := input["id"]
	nameField, hasName := input["name"]
	switch {
	case hasID && hasName:
		return spec, fmt.Errorf("updated_field must set either id or name, not both")
	case !hasID && !hasName:
		return spec, fmt.Errorf("updated_field requires either id or name")
	case hasID:
		id, err := validatePositiveInt64(idField)
		if err != nil {
			return spec, fmt.Errorf("updated_field.id: %w", err)
		}
		spec.id = id
	default:
		name, ok := nameField.(string)
		if !ok || name == "" {
			return spec, fmt.Errorf("updated_field.name must be a non-empty string")
		}
		spec.name = name
	}
	return spec, nil
}

func resolveBatchProjectField(ctx context.Context, gqlClient *githubv4.Client, owner, ownerType string, projectNumber int, spec batchFieldSpec) (*ResolvedField, error) {
	if spec.name != "" {
		return resolveProjectFieldByName(ctx, gqlClient, owner, ownerType, projectNumber, spec.name, "")
	}

	fields, err := listAllProjectFields(ctx, gqlClient, owner, ownerType, projectNumber)
	if err != nil {
		return nil, err
	}

	id := fmt.Sprintf("%d", spec.id)
	for _, field := range fields {
		if field.ID == id {
			return &field, nil
		}
	}
	return nil, ghErrors.NewStructuredResolutionError(
		"field_not_found",
		id,
		fmt.Sprintf("no project field with id %s on project %s#%d; see candidates for available fields", id, owner, projectNumber),
		projectFieldCandidates(fields),
	)
}

func projectFieldCandidates(fields []ResolvedField) []any {
	candidates := make([]any, 0, len(fields))
	for _, field := range fields {
		candidates = append(candidates, map[string]any{
			"id":        field.ID,
			"name":      field.Name,
			"data_type": field.DataType,
		})
	}
	return candidates
}

func convertProjectFieldValue(field *ResolvedField, raw any) (githubv4.ProjectV2FieldValue, error) {
	var zero githubv4.ProjectV2FieldValue

	switch field.DataType {
	case "TEXT":
		s, ok := raw.(string)
		if !ok {
			return zero, fmt.Errorf("field %q is TEXT; value must be a string", field.Name)
		}
		v := githubv4.String(s)
		return githubv4.ProjectV2FieldValue{Text: &v}, nil

	case "NUMBER":
		f, ok := toFloat64(raw)
		if !ok {
			return zero, fmt.Errorf("field %q is NUMBER; value must be a number", field.Name)
		}
		v := githubv4.Float(f)
		return githubv4.ProjectV2FieldValue{Number: &v}, nil

	case "DATE":
		s, ok := raw.(string)
		if !ok {
			return zero, fmt.Errorf("field %q is DATE; value must be a YYYY-MM-DD string", field.Name)
		}
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			return zero, fmt.Errorf("field %q is DATE; value %q is not in YYYY-MM-DD format: %w", field.Name, s, err)
		}
		return githubv4.ProjectV2FieldValue{Date: &githubv4.Date{Time: t}}, nil

	case "SINGLE_SELECT":
		s, ok := raw.(string)
		if !ok || s == "" {
			return zero, fmt.Errorf("field %q is SINGLE_SELECT; value must be a non-empty string (option name or ID)", field.Name)
		}
		optID := s
		if resolvedID, optErr := resolveSingleSelectOptionByName(field, s); optErr == nil {
			optID = resolvedID
		} else {
			known := false
			for _, opt := range field.Options {
				if opt.ID == s {
					known = true
					break
				}
			}
			if !known {
				return zero, optErr
			}
		}
		v := githubv4.String(optID)
		return githubv4.ProjectV2FieldValue{SingleSelectOptionID: &v}, nil

	case "ITERATION":
		s, ok := raw.(string)
		if !ok || s == "" {
			return zero, fmt.Errorf("field %q is ITERATION; value must be a non-empty iteration ID string", field.Name)
		}
		v := githubv4.String(s)
		return githubv4.ProjectV2FieldValue{IterationID: &v}, nil

	default:
		return zero, fmt.Errorf("field %q has unsupported data type %q for update_project_items; use update_project_item instead", field.Name, field.DataType)
	}
}

func toFloat64(raw any) (float64, bool) {
	var number float64
	switch v := raw.(type) {
	case float64:
		number = v
	case int:
		number = float64(v)
	case int64:
		number = float64(v)
	default:
		return 0, false
	}
	if math.IsNaN(number) || math.IsInf(number, 0) {
		return 0, false
	}
	return number, true
}

type itemLookupResult struct {
	nodeID         string
	fullDatabaseID int64
	err            error
}

// Numeric lookups are deduplicated and concurrency-bounded; individual failures
// remain isolated while cancellation stops pending work.
func resolveItemNodeIDsByNumericID(ctx context.Context, client *github.Client, owner, ownerType string, projectNumber int, ids []int64) map[int64]itemLookupResult {
	seen := make(map[int64]struct{}, len(ids))
	var unique []int64
	for _, id := range ids {
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}

	out := make(map[int64]itemLookupResult, len(unique))
	if len(unique) == 0 {
		return out
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, batchItemLookupConcurrency)

	for _, id := range unique {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				mu.Lock()
				out[id] = itemLookupResult{err: ctx.Err()}
				mu.Unlock()
				return
			}
			defer func() { <-sem }()

			if ctx.Err() != nil {
				mu.Lock()
				out[id] = itemLookupResult{err: ctx.Err()}
				mu.Unlock()
				return
			}

			var item *github.ProjectV2Item
			var err error
			if ownerType == "org" {
				item, _, err = client.Projects.GetOrganizationProjectItem(ctx, owner, projectNumber, id, nil)
			} else {
				item, _, err = client.Projects.GetUserProjectItem(ctx, owner, projectNumber, id, nil)
			}

			var res itemLookupResult
			switch {
			case err != nil:
				res = itemLookupResult{err: fmt.Errorf("project item %d: %w", id, err)}
			case item == nil || item.NodeID == nil || *item.NodeID == "":
				res = itemLookupResult{err: fmt.Errorf("project item %d: response did not include a node id", id)}
			default:
				res = itemLookupResult{nodeID: *item.NodeID, fullDatabaseID: id}
			}

			mu.Lock()
			out[id] = res
			mu.Unlock()
		}(id)
	}
	wg.Wait()
	return out
}

type issueRefKey struct {
	owner  string
	repo   string
	number int
}

func resolveIssueRefs(ctx context.Context, gqlClient *githubv4.Client, projectID githubv4.ID, items []parsedBatchItem) map[issueRefKey]itemLookupResult {
	out := make(map[issueRefKey]itemLookupResult)
	for _, it := range items {
		if it.err != nil || it.refKind != batchRefIssue {
			continue
		}
		key := issueRefKey{owner: it.issueOwner, repo: it.issueRepo, number: it.issueNumber}
		if _, done := out[key]; done {
			continue
		}
		nodeID, itemID, err := resolveProjectItemByIssueNumberWithProjectID(ctx, gqlClient, projectID, it.issueOwner, it.issueRepo, it.issueNumber)
		out[key] = itemLookupResult{nodeID: nodeID, fullDatabaseID: itemID, err: err}
	}
	return out
}

func batchErrorFromResolution(err error) *batchItemError {
	var structured *ghErrors.StructuredResolutionError
	if errors.As(err, &structured) {
		return &batchItemError{
			Code:       structured.Kind,
			Message:    fmt.Sprintf("%s: %s", structured.Kind, structured.Name),
			Hint:       structured.Hint,
			Candidates: structured.Candidates,
		}
	}
	return &batchItemError{
		Code:    "resolution_failed",
		Message: err.Error(),
	}
}
