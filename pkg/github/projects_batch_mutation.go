package github

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"

	"github.com/shurcooL/githubv4"
)

const batchMutationWireChunkSize = 20

type batchMutationKind int

const (
	batchMutationUpdate batchMutationKind = iota
	batchMutationClear
)

func (k batchMutationKind) fieldName() string {
	if k == batchMutationClear {
		return "clearProjectV2ItemFieldValue"
	}
	return "updateProjectV2ItemFieldValue"
}

type projectV2ItemMutationResult struct {
	ProjectV2Item struct {
		ID             string
		FullDatabaseID string `graphql:"fullDatabaseId"`
	} `graphql:"projectV2Item"`
}

type reflectedMutationTypeKey struct {
	kind batchMutationKind
	size int
}

var reflectedMutationTypeCache sync.Map

// Reflected types are cached only by operation and chunk size to bound
// reflect.StructOf's runtime cache; positional names and tags keep request data
// out of type identities. The pinned Client.Mutate binds its third argument to
// $input, so item0 uses $input and later aliases use $input1, $input2, ...
// supplied through the variables map.
func buildAliasedMutationType(kind batchMutationKind, size int) reflect.Type {
	key := reflectedMutationTypeKey{kind: kind, size: size}
	if cached, ok := reflectedMutationTypeCache.Load(key); ok {
		return cached.(reflect.Type)
	}

	resultType := reflect.TypeFor[projectV2ItemMutationResult]()
	fields := make([]reflect.StructField, size)
	for i := range size {
		varName := "input"
		if i > 0 {
			varName = fmt.Sprintf("input%d", i)
		}
		fields[i] = reflect.StructField{
			Name: fmt.Sprintf("Item%d", i),
			Type: resultType,
			Tag:  reflect.StructTag(fmt.Sprintf(`graphql:"item%d: %s(input: $%s)"`, i, kind.fieldName(), varName)),
		}
	}

	t := reflect.StructOf(fields)
	actual, _ := reflectedMutationTypeCache.LoadOrStore(key, t)
	return actual.(reflect.Type)
}

type mutationAliasOutcome struct {
	// Populated confirms this alias returned a project item, even when the
	// response also contains GraphQL errors.
	Populated      bool
	NodeID         string
	FullDatabaseID string
}

// The pinned client decodes partial data before returning GraphQL errors but
// discards errors[].path. Populated aliases confirm writes; unpopulated aliases
// remain unknown and must not be retried individually.
func executeAliasedMutation(ctx context.Context, gqlClient *githubv4.Client, kind batchMutationKind, inputs []githubv4.Input) ([]mutationAliasOutcome, error) {
	if len(inputs) == 0 {
		return nil, nil
	}
	if len(inputs) > batchMutationWireChunkSize {
		return nil, fmt.Errorf("internal error: chunk of %d exceeds wire chunk size %d", len(inputs), batchMutationWireChunkSize)
	}

	mutationType := buildAliasedMutationType(kind, len(inputs))
	mutationPtr := reflect.New(mutationType)

	var variables map[string]any
	if len(inputs) > 1 {
		variables = make(map[string]any, len(inputs)-1)
		for i := 1; i < len(inputs); i++ {
			variables[fmt.Sprintf("input%d", i)] = inputs[i]
		}
	}

	mutateErr := gqlClient.Mutate(ctx, mutationPtr.Interface(), inputs[0], variables)

	outcomes := make([]mutationAliasOutcome, len(inputs))
	elem := mutationPtr.Elem()
	for i := range inputs {
		result, ok := elem.Field(i).Interface().(projectV2ItemMutationResult)
		if !ok || result.ProjectV2Item.ID == "" {
			continue
		}
		outcomes[i] = mutationAliasOutcome{
			Populated:      true,
			NodeID:         result.ProjectV2Item.ID,
			FullDatabaseID: result.ProjectV2Item.FullDatabaseID,
		}
	}
	return outcomes, mutateErr
}

// The pinned client's GraphQL response error type is unexported; transport and
// decoding failures must remain distinguishable.
func isGraphQLResponseError(err error) bool {
	for err != nil {
		errType := reflect.TypeOf(err)
		if errType.PkgPath() == "github.com/shurcooL/graphql" && errType.Name() == "errors" {
			return true
		}
		err = errors.Unwrap(err)
	}
	return false
}
