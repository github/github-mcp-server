// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package mcp

import (
	"context"
)

// A PromptHandler handles a call to prompts/get.
type PromptHandler func(context.Context, *GetPromptRequest) (*GetPromptResult, error)

type serverPrompt struct {
	prompt  *Prompt
	handler PromptHandler
}
