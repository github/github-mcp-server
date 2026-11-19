package oauth

import (
	"context"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

type gatedRefreshSource struct {
	started chan struct{}
	release chan struct{}
	calls   atomic.Int32
}

func (s *gatedRefreshSource) Token() (*oauth2.Token, error) {
	if s.calls.Add(1) == 1 {
		close(s.started)
	}
	<-s.release
	return &oauth2.Token{AccessToken: "refreshed-token", Expiry: time.Now().Add(time.Hour)}, nil
}

func TestCanceledAuthorizationDoesNotWaitForTokenRefresh(t *testing.T) {
	const flowID = "authorized-flow"
	for _, tc := range []struct {
		name string
		wait func(context.Context, *Manager) (*Outcome, error)
	}{
		{name: "authenticate", wait: func(ctx context.Context, m *Manager) (*Outcome, error) { return m.Authenticate(ctx, nil) }},
		{name: "await token", wait: func(ctx context.Context, m *Manager) (*Outcome, error) { return m.AwaitToken(ctx, flowID) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				source := &gatedRefreshSource{started: make(chan struct{}), release: make(chan struct{})}
				m := NewManager(Config{}, testLogger())
				m.flowID, m.source = flowID, source
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				canceled := make(chan error, 1)
				go func() {
					_, err := tc.wait(ctx, m)
					canceled <- err
				}()
				<-source.started
				active := make(chan string, 1)
				go func() { active <- m.AccessToken() }()
				synctest.Wait()

				cancel()
				synctest.Wait()
				select {
				case err := <-canceled:
					assert.ErrorIs(t, err, context.Canceled)
				default:
					t.Error("canceled authorization still waits for the token source")
				}
				assert.Zero(t, len(active), "the active caller still needs the token")
				assert.EqualValues(t, 1, source.calls.Load(), "both callers share one refresh")

				close(source.release)
				assert.Equal(t, "refreshed-token", <-active)
			})
		})
	}
}

func TestAlreadyCanceledAuthorizationDoesNotStartTokenRefresh(t *testing.T) {
	source := &gatedRefreshSource{started: make(chan struct{}), release: make(chan struct{})}
	close(source.release)
	m := NewManager(Config{}, testLogger())
	m.source = source
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := m.Authenticate(ctx, nil)
	require.ErrorIs(t, err, context.Canceled)
	assert.Zero(t, source.calls.Load())
}

func TestNewAuthorizationSupersedesPendingTokenRefresh(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const flowID = "new-authorization"
		const installedToken = "installed-token"
		source := &gatedRefreshSource{started: make(chan struct{}), release: make(chan struct{})}
		m := NewManager(Config{}, testLogger())
		m.flowID, m.source = flowID, source
		waiters := make(chan string, 2)
		go func() { waiters <- m.AccessToken() }()
		<-source.started
		go func() { waiters <- m.AccessToken() }()
		synctest.Wait()
		assert.EqualValues(t, 1, source.calls.Load(), "both callers share the old refresh")

		m.complete(flowID, &oauth2.Token{AccessToken: installedToken, Expiry: time.Now().Add(time.Hour)}, nil)
		synctest.Wait()
		for range 2 {
			select {
			case got := <-waiters:
				assert.Equal(t, installedToken, got)
			default:
				t.Error("installed token did not wake a caller waiting on the old refresh")
			}
		}

		close(source.release)
		synctest.Wait()
		assert.Equal(t, installedToken, m.AccessToken(), "the old refresh cannot replace the installed token")
	})
}
