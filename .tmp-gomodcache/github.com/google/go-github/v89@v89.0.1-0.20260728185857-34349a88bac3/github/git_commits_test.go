// Copyright 2013 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func mockSigner(t *testing.T, signature string, emitErr error, wantMessage string) MessageSignerFunc {
	return func(w io.Writer, r io.Reader) error {
		t.Helper()
		message, err := io.ReadAll(r)
		assertNilError(t, err)
		if wantMessage != "" && string(message) != wantMessage {
			t.Errorf("MessageSignerFunc got %q, want %q", string(message), wantMessage)
		}
		assertWrite(t, w, []byte(signature))
		return emitErr
	}
}

func uncalledSigner(t *testing.T) MessageSignerFunc {
	return func(io.Writer, io.Reader) error {
		t.Error("MessageSignerFunc should not be called")
		return nil
	}
}

func TestGitService_GetCommit(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/git/commits/s", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{"sha":"s","message":"Commit Message.","author":{"name":"n"}}`)
	})

	ctx := t.Context()
	commit, _, err := client.Git.GetCommit(ctx, "o", "r", "s")
	if err != nil {
		t.Errorf("Git.GetCommit returned error: %v", err)
	}

	want := &Commit{SHA: Ptr("s"), Message: Ptr("Commit Message."), Author: &CommitAuthor{Name: Ptr("n")}}
	if !cmp.Equal(commit, want) {
		t.Errorf("Git.GetCommit returned %+v, want %+v", commit, want)
	}

	const methodName = "GetCommit"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Git.GetCommit(ctx, "\n", "\n", "\n")
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Git.GetCommit(ctx, "o", "r", "s")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestGitService_GetCommit_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Git.GetCommit(ctx, "%", "%", "%")
	testURLParseError(t, err)
}

func TestGitService_CreateCommit(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := Commit{
		Message: Ptr("Commit Message."),
		Tree:    &Tree{SHA: Ptr("t")},
		Parents: []*Commit{{SHA: Ptr("p")}},
	}

	mux.HandleFunc("/repos/o/r/git/commits", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		want := createCommit{
			Message: input.Message,
			Tree:    input.Tree.SHA,
			Parents: []string{*input.Parents[0].SHA},
		}
		testJSONBody(t, r, want)
		fmt.Fprint(w, `{"sha":"s"}`)
	})

	ctx := t.Context()
	commit, _, err := client.Git.CreateCommit(ctx, "o", "r", input, nil)
	if err != nil {
		t.Errorf("Git.CreateCommit returned error: %v", err)
	}

	want := &Commit{SHA: Ptr("s")}
	if !cmp.Equal(commit, want) {
		t.Errorf("Git.CreateCommit returned %+v, want %+v", commit, want)
	}

	const methodName = "CreateCommit"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Git.CreateCommit(ctx, "\n", "\n", input, nil)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Git.CreateCommit(ctx, "o", "r", input, nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestGitService_CreateSignedCommit(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := Commit{
		Message: Ptr("Commit Message."),
		Tree:    &Tree{SHA: Ptr("t")},
		Parents: []*Commit{{SHA: Ptr("p")}},
		Verification: &SignatureVerification{
			Signature: Ptr("----- BEGIN PGP SIGNATURE -----\n\naaaa\naaaa\n----- END PGP SIGNATURE -----"),
		},
	}

	mux.HandleFunc("/repos/o/r/git/commits", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		want := struct {
			Message   *string  `json:"message,omitempty"`
			Tree      *string  `json:"tree,omitempty"`
			Parents   []string `json:"parents,omitempty"`
			Signature *string  `json:"signature,omitempty"`
		}{
			Message:   input.Message,
			Tree:      input.Tree.SHA,
			Parents:   []string{*input.Parents[0].SHA},
			Signature: input.Verification.Signature,
		}
		testJSONBody(t, r, want)
		fmt.Fprint(w, `{"sha":"commitSha"}`)
	})

	ctx := t.Context()
	commit, _, err := client.Git.CreateCommit(ctx, "o", "r", input, nil)
	if err != nil {
		t.Errorf("Git.CreateCommit returned error: %v", err)
	}

	want := &Commit{SHA: Ptr("commitSha")}
	if !cmp.Equal(commit, want) {
		t.Errorf("Git.CreateCommit returned %+v, want %+v", commit, want)
	}

	const methodName = "CreateCommit"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Git.CreateCommit(ctx, "\n", "\n", input, nil)
		return err
	})

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Git.CreateCommit(ctx, "o", "r", input, nil)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestGitService_CreateSignedCommitWithInvalidParams(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	input := Commit{}

	ctx := t.Context()
	opts := CreateCommitOptions{Signer: uncalledSigner(t)}
	_, _, err := client.Git.CreateCommit(ctx, "o", "r", input, &opts)
	if err == nil {
		t.Error("Expected error to be returned because invalid params were passed")
	}
}

func TestGitService_CreateCommit_WithSigner(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	signature := "my voice is my password"
	date := time.Date(2017, time.May, 4, 0, 3, 43, 0, time.FixedZone("CEST", 2*3600))
	author := CommitAuthor{
		Name:  Ptr("go-github"),
		Email: Ptr("go-github@github.com"),
		Date:  &Timestamp{date},
	}
	wantMessage := `tree t
parent p
author go-github <go-github@github.com> 1493849023 +0200
committer go-github <go-github@github.com> 1493849023 +0200

Commit Message.`
	sha := "commitSha"
	input := Commit{
		SHA:     &sha,
		Message: Ptr("Commit Message."),
		Tree:    &Tree{SHA: Ptr("t")},
		Parents: []*Commit{{SHA: Ptr("p")}},
		Author:  &author,
	}
	wantBody := &createCommit{
		Message:   input.Message,
		Tree:      Ptr("t"),
		Parents:   []string{"p"},
		Author:    &author,
		Signature: &signature,
	}
	mux.HandleFunc("/repos/o/r/git/commits", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		testJSONBody(t, r, wantBody)
		fmt.Fprintf(w, `{"sha":"%v"}`, sha)
	})
	ctx := t.Context()
	wantCommit := &Commit{SHA: &sha}
	opts := CreateCommitOptions{Signer: mockSigner(t, signature, nil, wantMessage)}
	commit, _, err := client.Git.CreateCommit(ctx, "o", "r", input, &opts)
	assertNilError(t, err)
	if cmp.Diff(commit, wantCommit) != "" {
		t.Errorf("Git.CreateCommit returned %+v, want %+v\n%v", commit, wantCommit, cmp.Diff(commit, wantCommit))
	}
}

func TestGitService_createSignature_nilSigner(t *testing.T) {
	t.Parallel()
	a := &createCommit{
		Message: Ptr("Commit Message."),
		Tree:    Ptr("t"),
		Parents: []string{"p"},
	}

	_, err := createSignature(nil, a)

	if err == nil {
		t.Error("Expected error to be returned because no author was passed")
	}
}

func TestGitService_createSignature_nilCommit(t *testing.T) {
	t.Parallel()
	_, err := createSignature(uncalledSigner(t), nil)

	if err == nil {
		t.Error("Expected error to be returned because no author was passed")
	}
}

func TestGitService_createSignature_signerError(t *testing.T) {
	t.Parallel()
	a := &createCommit{
		Message: Ptr("Commit Message."),
		Tree:    Ptr("t"),
		Parents: []string{"p"},
		Author:  &CommitAuthor{Name: Ptr("go-github")},
	}

	signer := mockSigner(t, "", errors.New("signer error"), "")
	_, err := createSignature(signer, a)

	if err == nil {
		t.Error("Expected error to be returned because signer returned an error")
	}
}

func TestGitService_createSignatureMessage_nilCommit(t *testing.T) {
	t.Parallel()
	_, err := createSignatureMessage(nil)
	if err == nil {
		t.Error("Expected error to be returned due to nil key")
	}
}

func TestGitService_createSignatureMessage_nilMessage(t *testing.T) {
	t.Parallel()
	date, _ := time.Parse("Mon Jan 02 15:04:05 2006 -0700", "Thu May 04 00:03:43 2017 +0200")

	_, err := createSignatureMessage(&createCommit{
		Message: nil,
		Parents: []string{"p"},
		Author: &CommitAuthor{
			Name:  Ptr("go-github"),
			Email: Ptr("go-github@github.com"),
			Date:  &Timestamp{date},
		},
	})
	if err == nil {
		t.Error("Expected error to be returned due to nil key")
	}
}

func TestGitService_createSignatureMessage_emptyMessage(t *testing.T) {
	t.Parallel()
	date, _ := time.Parse("Mon Jan 02 15:04:05 2006 -0700", "Thu May 04 00:03:43 2017 +0200")
	_, err := createSignatureMessage(&createCommit{
		Message: Ptr(""),
		Parents: []string{"p"},
		Author: &CommitAuthor{
			Name:  Ptr("go-github"),
			Email: Ptr("go-github@github.com"),
			Date:  &Timestamp{date},
		},
	})
	if err == nil {
		t.Error("Expected error to be returned due to nil key")
	}
}

func TestGitService_createSignatureMessage_nilAuthor(t *testing.T) {
	t.Parallel()
	_, err := createSignatureMessage(&createCommit{
		Message: Ptr("Commit Message."),
		Parents: []string{"p"},
		Author:  nil,
	})
	if err == nil {
		t.Error("Expected error to be returned due to nil key")
	}
}

func TestGitService_createSignatureMessage_withoutTree(t *testing.T) {
	t.Parallel()
	date, _ := time.Parse("Mon Jan 02 15:04:05 2006 -0700", "Thu May 04 00:03:43 2017 +0200")

	msg, _ := createSignatureMessage(&createCommit{
		Message: Ptr("Commit Message."),
		Parents: []string{"p"},
		Author: &CommitAuthor{
			Name:  Ptr("go-github"),
			Email: Ptr("go-github@github.com"),
			Date:  &Timestamp{date},
		},
	})
	expected := `parent p
author go-github <go-github@github.com> 1493849023 +0200
committer go-github <go-github@github.com> 1493849023 +0200

Commit Message.`
	if msg != expected {
		t.Errorf("Returned message incorrect. returned %v, want %v", msg, expected)
	}
}

func TestGitService_createSignatureMessage_withoutCommitter(t *testing.T) {
	t.Parallel()
	date, _ := time.Parse("Mon Jan 02 15:04:05 2006 -0700", "Thu May 04 00:03:43 2017 +0200")

	msg, _ := createSignatureMessage(&createCommit{
		Message: Ptr("Commit Message."),
		Parents: []string{"p"},
		Author: &CommitAuthor{
			Name:  Ptr("go-github"),
			Email: Ptr("go-github@github.com"),
			Date:  &Timestamp{date},
		},
		Committer: &CommitAuthor{
			Name:  Ptr("foo"),
			Email: Ptr("foo@example.com"),
			Date:  &Timestamp{date},
		},
	})
	expected := `parent p
author go-github <go-github@github.com> 1493849023 +0200
committer foo <foo@example.com> 1493849023 +0200

Commit Message.`
	if msg != expected {
		t.Errorf("Returned message incorrect. returned %v, want %v", msg, expected)
	}
}

func TestGitService_CreateCommit_invalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Git.CreateCommit(ctx, "%", "%", Commit{}, nil)
	testURLParseError(t, err)
}
