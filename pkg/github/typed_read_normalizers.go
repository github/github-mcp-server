package github

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/github/github-mcp-server/v2/pkg/inventory"
)

// maxNumericTokenLen and maxNumericExponent bound decimal or exponent number
// tokens before they are checked as exact rationals.
const (
	maxNumericTokenLen = 64
	maxNumericExponent = 40
)

// exactIntegerToken reports the int64 value of a JSON number token written in
// decimal or exponent form, but only when the token is exactly an integer that
// fits in int64.
func exactIntegerToken(token string) (int64, bool) {
	if len(token) > maxNumericTokenLen {
		return 0, false
	}
	if i := strings.IndexAny(token, "eE"); i >= 0 {
		exp, err := strconv.Atoi(token[i+1:])
		if err != nil || exp > maxNumericExponent || exp < -maxNumericExponent {
			return 0, false
		}
	}
	exact, ok := new(big.Rat).SetString(token)
	if !ok || !exact.IsInt() || !exact.Num().IsInt64() {
		return 0, false
	}
	return exact.Num().Int64(), true
}

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
			decoder := json.NewDecoder(bytes.NewReader(value))
			decoder.UseNumber()
			if err := decoder.Decode(&number); err != nil {
				return nil, err
			}
			var intVal int
			switch v := number.(type) {
			case string:
				parsed, err := strconv.ParseFloat(v, 64)
				if err != nil {
					return nil, fmt.Errorf("parameter %s is not a valid number", field)
				}
				if math.IsNaN(parsed) || math.IsInf(parsed, 0) || parsed != math.Trunc(parsed) || !inSignedIntRange(parsed, strconv.IntSize) {
					return nil, fmt.Errorf("parameter %s is not a valid number", field)
				}
				intVal = int(parsed)
			case json.Number:
				// Validate the original token before any lossy float64 conversion.
				// Plain integer tokens are parsed exactly. Decimal or exponent
				// forms are checked as exact rationals; the token length and
				// exponent are bounded first so a hostile exponent cannot make
				// the exact check expensive.
				integer, err := strconv.ParseInt(v.String(), 10, strconv.IntSize)
				if err != nil {
					var numErr *strconv.NumError
					if errors.As(err, &numErr) && errors.Is(numErr.Err, strconv.ErrRange) {
						return nil, fmt.Errorf("parameter %s is not a valid number", field)
					}
					exact, ok := exactIntegerToken(v.String())
					if !ok {
						return nil, fmt.Errorf("parameter %s is not a valid number", field)
					}
					integer = exact
				}
				if int64(int(integer)) != integer {
					return nil, fmt.Errorf("parameter %s is not a valid number", field)
				}
				intVal = int(integer)
			default:
				continue
			}
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
