package schema

import (
	"sync"

	"github.com/google/jsonschema-go/jsonschema"
)

type Cache struct {
	cachedSchemas map[string]*jsonschema.Resolved
	mu            sync.Mutex
}

func NewCache() *Cache {
	return &Cache{
		cachedSchemas: make(map[string]*jsonschema.Resolved),
	}
}

func (sc *Cache) GetCachedOrResolveSchema(toolName string, schema *jsonschema.Schema) (*jsonschema.Resolved, error) {
	if resolvedSchema, ok := sc.cachedSchemas[toolName]; ok {
		return resolvedSchema, nil
	}

	sc.mu.Lock()
	defer sc.mu.Unlock()
	resolvedSchema, err := schema.Resolve(nil)
	if err != nil {
		return nil, err
	}

	sc.cachedSchemas[toolName] = resolvedSchema
	return resolvedSchema, nil
}
