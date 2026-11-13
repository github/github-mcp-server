package github

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/github/github-mcp-server/v2/pkg/inventory"
)

func normalizeTypedReadArguments(uppercaseFields []string, preserveZeroPage bool) inventory.InputNormalizer {
	return func(raw json.RawMessage) (json.RawMessage, error) {
		var args map[string]json.RawMessage
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, err
		}
		if args == nil {
			return raw, nil
		}

		for _, field := range uppercaseFields {
			var value string
			if err := json.Unmarshal(args[field], &value); err == nil {
				normalized, err := json.Marshal(strings.ToUpper(value))
				if err != nil {
					return nil, err
				}
				args[field] = normalized
			}
		}

		for _, field := range []string{"page", "perPage"} {
			value, exists := args[field]
			if !exists {
				continue
			}
			var number any
			if err := json.Unmarshal(value, &number); err != nil {
				return nil, err
			}
			var parsed float64
			switch v := number.(type) {
			case string:
				var err error
				parsed, err = strconv.ParseFloat(v, 64)
				if err != nil {
					return nil, fmt.Errorf("parameter %s is not a valid number", field)
				}
			case float64:
				parsed = v
			default:
				continue
			}
			if math.IsNaN(parsed) || math.IsInf(parsed, 0) || parsed != math.Trunc(parsed) || !inSignedIntRange(parsed, strconv.IntSize) {
				return nil, fmt.Errorf("parameter %s is not a valid number", field)
			}
			intVal := int(parsed)
			if field == "page" && !preserveZeroPage && intVal == 0 {
				delete(args, field)
				continue
			}
			if field == "perPage" && intVal == 0 {
				delete(args, field)
				continue
			}
			normalized, err := json.Marshal(intVal)
			if err != nil {
				return nil, err
			}
			args[field] = normalized
		}

		return json.Marshal(args)
	}
}
