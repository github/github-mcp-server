package graphql_test

import (
	"testing"

	"github.com/shurcooL/graphql"
)

func TestNewScalars(t *testing.T) {
	if got := graphql.NewBoolean(false); got == nil {
		t.Error("NewBoolean returned nil")
	}
	if got := graphql.NewFloat(0.0); got == nil {
		t.Error("NewFloat returned nil")
	}
	// ID with underlying type string.
	if got := graphql.NewID(""); got == nil {
		t.Error("NewID returned nil")
	}
	// ID with underlying type int.
	if got := graphql.NewID(0); got == nil {
		t.Error("NewID returned nil")
	}
	if got := graphql.NewInt(0); got == nil {
		t.Error("NewInt returned nil")
	}
	if got := graphql.NewString(""); got == nil {
		t.Error("NewString returned nil")
	}
}
