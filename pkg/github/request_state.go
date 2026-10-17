package github

import "context"

// RequestStateSealer protects opaque state sent to clients during multi-round-trip requests.
type RequestStateSealer interface {
	Seal(context.Context, []byte) (string, error)
	Open(string) ([]byte, error)
}

// RequestStateSealerProvider optionally supplies request-state protection to tools.
// Keeping this separate from ToolDependencies preserves compatibility for integrators
// that do not expose tools which use multi-round-trip request state. Stateless HTTP
// integrators should implement it or exclude tools that return request state.
type RequestStateSealerProvider interface {
	GetRequestStateSealer() RequestStateSealer
}

func requestStateSealerFromDeps(deps ToolDependencies) RequestStateSealer {
	provider, ok := deps.(RequestStateSealerProvider)
	if !ok {
		return nil
	}
	return provider.GetRequestStateSealer()
}
