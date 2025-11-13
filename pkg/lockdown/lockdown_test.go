package lockdown

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shurcooL/githubv4"
)

func TestShouldRemoveContentCachesResultsWithinInterval(t *testing.T) {
	clearRepoAccessCache()
	defer clearRepoAccessCache()

	originalInfoFunc := repoAccessInfoFunc
	defer func() { repoAccessInfoFunc = originalInfoFunc }()

	originalTimeNow := timeNow
	defer func() { timeNow = originalTimeNow }()

	fixed := time.Now()
	timeNow = func() time.Time { return fixed }

	callCount := 0
	repoAccessInfoFunc = func(_ context.Context, _ *githubv4.Client, _, _, _ string) (bool, bool, error) {
		callCount++
		return false, true, nil
	}

	ctx := context.Background()

	remove, err := ShouldRemoveContent(ctx, nil, "User", "Owner", "Repo")
	if err != nil {
		t.Fatalf("unexpected error on first call: %v", err)
	}
	if remove {
		t.Fatalf("expected remove=false when user has push access")
	}

	remove, err = ShouldRemoveContent(ctx, nil, "user", "owner", "repo")
	if err != nil {
		t.Fatalf("unexpected error on cached call: %v", err)
	}
	if remove {
		t.Fatalf("expected remove=false when cached entry reused")
	}
	if callCount != 1 {
		t.Fatalf("expected cached result to prevent additional repo access queries, got %d", callCount)
	}
}

func TestShouldRemoveContentRefreshesAfterInterval(t *testing.T) {
	clearRepoAccessCache()
	defer clearRepoAccessCache()

	originalInfoFunc := repoAccessInfoFunc
	defer func() { repoAccessInfoFunc = originalInfoFunc }()

	originalTimeNow := timeNow
	defer func() { timeNow = originalTimeNow }()

	base := time.Now()
	current := base
	timeNow = func() time.Time { return current }

	callCount := 0
	repoAccessInfoFunc = func(_ context.Context, _ *githubv4.Client, _, _, _ string) (bool, bool, error) {
		callCount++
		if callCount == 1 {
			return false, false, nil
		}
		return false, true, nil
	}

	ctx := context.Background()

	remove, err := ShouldRemoveContent(ctx, nil, "user", "owner", "repo")
	if err != nil {
		t.Fatalf("unexpected error on first call: %v", err)
	}
	if !remove {
		t.Fatalf("expected remove=true when user lacks push access")
	}
	if callCount != 1 {
		t.Fatalf("expected first call to query once, got %d", callCount)
	}

	current = base.Add(9 * time.Minute)
	remove, err = ShouldRemoveContent(ctx, nil, "user", "owner", "repo")
	if err != nil {
		t.Fatalf("unexpected error before refresh interval: %v", err)
	}
	if !remove {
		t.Fatalf("expected remove=true before refresh interval expires")
	}
	if callCount != 1 {
		t.Fatalf("expected cached value before refresh interval, got %d calls", callCount)
	}

	current = base.Add(11 * time.Minute)
	remove, err = ShouldRemoveContent(ctx, nil, "user", "owner", "repo")
	if err != nil {
		t.Fatalf("unexpected error after refresh interval: %v", err)
	}
	if remove {
		t.Fatalf("expected remove=false after permissions refreshed")
	}
	if callCount != 2 {
		t.Fatalf("expected refreshed access info after interval, got %d calls", callCount)
	}
}

func TestShouldRemoveContentDoesNotCacheErrors(t *testing.T) {
	clearRepoAccessCache()
	defer clearRepoAccessCache()

	originalInfoFunc := repoAccessInfoFunc
	defer func() { repoAccessInfoFunc = originalInfoFunc }()

	originalTimeNow := timeNow
	defer func() { timeNow = originalTimeNow }()

	now := time.Now()
	timeNow = func() time.Time { return now }

	callCount := 0
	repoAccessInfoFunc = func(_ context.Context, _ *githubv4.Client, _, _, _ string) (bool, bool, error) {
		callCount++
		if callCount == 1 {
			return false, false, errors.New("boom")
		}
		return false, false, nil
	}

	ctx := context.Background()

	if _, err := ShouldRemoveContent(ctx, nil, "user", "owner", "repo"); err == nil {
		t.Fatal("expected error on first call")
	}
	if callCount != 1 {
		t.Fatalf("expected single call after error, got %d", callCount)
	}

	remove, err := ShouldRemoveContent(ctx, nil, "user", "owner", "repo")
	if err != nil {
		t.Fatalf("unexpected error on retry: %v", err)
	}
	if !remove {
		t.Fatalf("expected remove=true when user lacks push access")
	}
	if callCount != 2 {
		t.Fatalf("expected repo access to be queried again after error, got %d calls", callCount)
	}
}
