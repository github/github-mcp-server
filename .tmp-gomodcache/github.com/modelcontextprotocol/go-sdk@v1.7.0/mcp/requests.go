// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// This file holds the request types.

package mcp

type (
	CallToolRequest                   = ServerRequest[*CallToolParamsRaw]
	CompleteRequest                   = ServerRequest[*CompleteParams]
	GetPromptRequest                  = ServerRequest[*GetPromptParams]
	InitializedRequest                = ServerRequest[*InitializedParams]
	ListPromptsRequest                = ServerRequest[*ListPromptsParams]
	ListResourcesRequest              = ServerRequest[*ListResourcesParams]
	ListResourceTemplatesRequest      = ServerRequest[*ListResourceTemplatesParams]
	ListToolsRequest                  = ServerRequest[*ListToolsParams]
	ProgressNotificationServerRequest = ServerRequest[*ProgressNotificationParams]
	ReadResourceRequest               = ServerRequest[*ReadResourceParams]
	RootsListChangedRequest           = ServerRequest[*RootsListChangedParams]
	SubscribeRequest                  = ServerRequest[*SubscribeParams]
	SubscriptionsListenRequest        = ServerRequest[*SubscriptionsListenParams]
	UnsubscribeRequest                = ServerRequest[*UnsubscribeParams]
)

type (
	CreateMessageRequest                   = ClientRequest[*CreateMessageParams]
	CreateMessageWithToolsRequest          = ClientRequest[*CreateMessageWithToolsParams]
	DiscoverRequest                        = ClientRequest[*DiscoverParams]
	ElicitRequest                          = ClientRequest[*ElicitParams]
	initializedClientRequest               = ClientRequest[*InitializedParams]
	InitializeRequest                      = ClientRequest[*InitializeParams]
	ListRootsRequest                       = ClientRequest[*ListRootsParams]
	LoggingMessageRequest                  = ClientRequest[*LoggingMessageParams]
	ProgressNotificationClientRequest      = ClientRequest[*ProgressNotificationParams]
	PromptListChangedRequest               = ClientRequest[*PromptListChangedParams]
	ResourceListChangedRequest             = ClientRequest[*ResourceListChangedParams]
	ResourceUpdatedNotificationRequest     = ClientRequest[*ResourceUpdatedNotificationParams]
	ToolListChangedRequest                 = ClientRequest[*ToolListChangedParams]
	ElicitationCompleteNotificationRequest = ClientRequest[*ElicitationCompleteParams]
)
