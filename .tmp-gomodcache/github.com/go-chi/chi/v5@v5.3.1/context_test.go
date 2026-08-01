package chi

import "testing"

// TestRoutePattern tests correct in-the-middle wildcard removals.
// If user organizes a router like this:
//
// (router.go)
//
//	r.Route("/v1", func(r chi.Router) {
//		r.Mount("/resources", resourcesController{}.Router())
//	}
//
// (resources_controller.go)
//
//	r.Route("/", func(r chi.Router) {
//		r.Get("/{resource_id}", getResource())
//		// other routes...
//	}
//
// This test checks how the route pattern is calculated
// "/v1/resources/{resource_id}" (right)
// "/v1/resources/*/{resource_id}" (wrong)
func TestRoutePattern(t *testing.T) {
	routePatterns := []string{
		"/v1/*",
		"/resources/*",
		"/{resource_id}",
	}

	x := &Context{
		RoutePatterns: routePatterns,
	}

	if p := x.RoutePattern(); p != "/v1/resources/{resource_id}" {
		t.Fatal("unexpected route pattern: " + p)
	}

	x.RoutePatterns = []string{
		"/v1/*",
		"/resources/*",
		// Additional wildcard, depending on the router structure of the user
		"/*",
		"/{resource_id}",
	}

	// Correctly removes in-the-middle wildcards instead of "/v1/resources/*/{resource_id}"
	if p := x.RoutePattern(); p != "/v1/resources/{resource_id}" {
		t.Fatal("unexpected route pattern: " + p)
	}

	x.RoutePatterns = []string{
		"/v1/*",
		"/resources/*",
		// Even with many wildcards
		"/*",
		"/*",
		"/*",
		"/{resource_id}/*", // Keeping trailing wildcard
	}

	// Correctly removes in-the-middle wildcards instead of "/v1/resources/*/*/{resource_id}/*"
	if p := x.RoutePattern(); p != "/v1/resources/{resource_id}/*" {
		t.Fatal("unexpected route pattern: " + p)
	}

	x.RoutePatterns = []string{
		"/v1/*",
		"/resources/*",
		// And respects asterisks as part of the paths
		"/*special_path/*",
		"/with_asterisks*/*",
		"/{resource_id}",
	}

	// Correctly removes in-the-middle wildcards instead of "/v1/resourcesspecial_path/with_asterisks{resource_id}"
	if p := x.RoutePattern(); p != "/v1/resources/*special_path/with_asterisks*/{resource_id}" {
		t.Fatal("unexpected route pattern: " + p)
	}

	// Testing for the root route pattern
	x.RoutePatterns = []string{"/"}
	// It should just return "/" as the pattern
	if p := x.RoutePattern(); p != "/" {
		t.Fatal("unexpected route pattern for root: " + p)
	}

	// Testing empty route pattern for nil context
	var nilContext *Context
	if p := nilContext.RoutePattern(); p != "" {
		t.Fatalf("unexpected non-empty route pattern for nil context: %q", p)
	}
}

// TestReplaceWildcardsConsecutive ensures multiple consecutive wildcards are
// collapsed into a single slash.
func TestReplaceWildcardsConsecutive(t *testing.T) {
	if p := replaceWildcards("/foo/*/*/*/bar"); p != "/foo/bar" {
		t.Fatalf("unexpected wildcard replacement: %s", p)
	}
	if p := replaceWildcards("/foo/*/*/*/bar/*"); p != "/foo/bar/*" {
		t.Fatalf("unexpected trailing wildcard behavior: %s", p)
	}
}
