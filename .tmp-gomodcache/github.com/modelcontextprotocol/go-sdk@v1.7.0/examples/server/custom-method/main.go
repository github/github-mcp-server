// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

// The custom-method example demonstrates registering and calling a custom
// JSON-RPC method that is not part of the standard MCP spec.
//
// The server registers a "latin/translate" method that translates simple
// English phrases into Latin. A client connects over an in-memory transport,
// calls the custom method, and prints the result.
package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type TranslateParams struct {
	mcp.ParamsBase
	Text string `json:"text"`
}

type TranslateResult struct {
	mcp.ResultBase
	Latin string `json:"latin"`
}

var translations = map[string]string{
	"hello":                    "salve",
	"goodbye":                  "vale",
	"thank you":                "gratias tibi ago",
	"how are you":              "quid agis",
	"good morning":             "bonum mane",
	"good night":               "bonam noctem",
	"friend":                   "amicus",
	"water":                    "aqua",
	"love":                     "amor",
	"war":                      "bellum",
	"peace":                    "pax",
	"truth":                    "veritas",
	"light":                    "lux",
	"time":                     "tempus",
	"life":                     "vita",
	"death":                    "mors",
	"star":                     "stella",
	"earth":                    "terra",
	"sea":                      "mare",
	"the die is cast":          "alea iacta est",
	"i came i saw i conquered": "veni vidi vici",
	"seize the day":            "carpe diem",
}

func main() {
	ctx := context.Background()

	server := mcp.NewServer(&mcp.Implementation{Name: "latin-server", Version: "v1.0.0"}, nil)

	if err := mcp.AddReceivingCustomMethod(server, "latin/translate",
		func(ctx context.Context, ss *mcp.ServerSession, params *TranslateParams) (*TranslateResult, error) {
			key := strings.ToLower(strings.TrimSpace(params.Text))
			latin, ok := translations[key]
			if !ok {
				latin = fmt.Sprintf("[unknown: %q — try: %s]", params.Text, knownPhrases())
			}
			return &TranslateResult{Latin: latin}, nil
		}); err != nil {
		log.Fatal(err)
	}

	ct, st := mcp.NewInMemoryTransports()

	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer ss.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "latin-client", Version: "v1.0.0"}, nil)
	if err := mcp.AddSendingCustomMethod[*TranslateParams, *TranslateResult](client, "latin/translate"); err != nil {
		log.Fatal(err)
	}

	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer cs.Close()

	phrases := []string{"Hello", "Seize the day", "Peace", "Truth", "I came I saw I conquered"}
	for _, phrase := range phrases {
		result, err := mcp.CallCustomMethod[*TranslateParams, *TranslateResult](
			ctx, cs, "latin/translate", &TranslateParams{Text: phrase})
		if err != nil {
			log.Fatalf("translate %q: %v", phrase, err)
		}
		fmt.Printf("%-35s → %s\n", phrase, result.Latin)
	}
}

func knownPhrases() string {
	phrases := make([]string, 0, len(translations))
	for k := range translations {
		phrases = append(phrases, fmt.Sprintf("%q", k))
	}
	return strings.Join(phrases, ", ")
}
