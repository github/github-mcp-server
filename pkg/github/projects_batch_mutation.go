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

// Reflected types are cached 仅by operation 和chunk size to bound
// reflect.StructOf's runtime cache; positional names 和tags keep 请求 数据
// out of type identities. pinned Client.Mutate binds its third 参数 to
// $输入, so item0 uses $输入 和later aliases use $输入1, $输入2, ...
// supplied through variables map.
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
	// Populated confirms this alias 返回ed 一个project item, even when the
	// 响应 也contains GraphQL 错误s.
	Populated      bool
	NodeID         string
	FullDatabaseID string
}

// pinned 客户端 decodes partial 数据 before 返回ing GraphQL 错误s but
// discards 错误s[].路径. Populated aliases confirm 写入s; unpopulated aliases
// remain 未知 和不得 be retried individually.
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

// pinned 客户端's GraphQL 响应 错误 type is unexported; transport and
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
