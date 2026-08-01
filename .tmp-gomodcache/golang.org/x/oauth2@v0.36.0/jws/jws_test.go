// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jws

import (
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"strings"
	"testing"
)

func TestSignAndVerify(t *testing.T) {
	header := &Header{
		Algorithm: "RS256",
		Typ:       "JWT",
	}
	payload := &ClaimSet{
		Iss: "http://google.com/",
		Aud: "",
		Exp: 3610,
		Iat: 10,
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	token, err := Encode(header, payload, privateKey)
	if err != nil {
		t.Fatal(err)
	}

	err = Verify(token, &privateKey.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
}

func TestVerifyFailsOnMalformedClaim(t *testing.T) {
	cases := []struct {
		desc  string
		token string
	}{
		{
			desc:  "no periods",
			token: "aa",
		}, {
			desc:  "only one period",
			token: "a.a",
		}, {
			desc:  "more than two periods",
			token: "a.a.a.a",
		},
	}
	for _, tc := range cases {
		f := func(t *testing.T) {
			err := Verify(tc.token, nil)
			if err == nil {
				t.Error("got no errors; want improperly formed JWT not to be verified")
			}
		}
		t.Run(tc.desc, f)
	}
}

func BenchmarkVerify(b *testing.B) {
	cases := []struct {
		desc  string
		token string
	}{
		{
			desc:  "full of periods",
			token: strings.Repeat(".", http.DefaultMaxHeaderBytes),
		}, {
			desc:  "two trailing periods",
			token: strings.Repeat("a", http.DefaultMaxHeaderBytes-2) + "..",
		},
	}
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		b.Fatal(err)
	}
	for _, bc := range cases {
		f := func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				Verify(bc.token, &privateKey.PublicKey)
			}
		}
		b.Run(bc.desc, f)
	}
}
