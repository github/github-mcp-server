// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

// Package mcpgodebug provides a mechanism to configure compatibility parameters
// via the MCPGODEBUG environment variable.
//
// The value of MCPGODEBUG is a comma-separated list of key=value pairs.
// For example:
//
//	MCPGODEBUG=someoption=1,otheroption=value
package mcpgodebug

import (
	"fmt"
	"os"
	"strings"
)

const compatibilityEnvKey = "MCPGODEBUG"

var compatibilityParams map[string]string

func init() {
	var err error
	compatibilityParams, err = parseCompatibility(os.Getenv(compatibilityEnvKey))
	if err != nil {
		panic(err)
	}
}

// Value returns the value of the compatibility parameter with the given key.
// It returns an empty string if the key is not set.
func Value(key string) string {
	return compatibilityParams[key]
}

func parseCompatibility(envValue string) (map[string]string, error) {
	if envValue == "" {
		return nil, nil
	}

	params := make(map[string]string)
	for part := range strings.SplitSeq(envValue, ",") {
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			return nil, fmt.Errorf("MCPGODEBUG: invalid format: %q", part)
		}
		params[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return params, nil
}
