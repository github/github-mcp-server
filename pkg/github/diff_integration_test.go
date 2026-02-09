package github

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplySemanticDiffToUnifiedPatch(t *testing.T) {
	t.Run("JSON file gets semantic diff", func(t *testing.T) {
		patch := `@@ -1,3 +1,3 @@
 {
-  "name": "old"
+  "name": "new"
 }`
		result := applySemanticDiffToUnifiedPatch("config.json", patch)
		assert.NotEqual(t, patch, result)
		assert.Contains(t, result, `name: "old" → "new"`)
	})

	t.Run("Go file gets structural diff", func(t *testing.T) {
		patch := `@@ -1,3 +1,4 @@
 func hello() {
+	fmt.Println("world")
 }`
		result := applySemanticDiffToUnifiedPatch("main.go", patch)
		assert.NotEqual(t, patch, result)
		assert.Contains(t, result, "function_declaration")
	})

	t.Run("Markdown file keeps original patch", func(t *testing.T) {
		patch := `@@ -1,3 +1,3 @@
 # Title
-old text
+new text`
		result := applySemanticDiffToUnifiedPatch("README.md", patch)
		assert.Equal(t, patch, result)
	})

	t.Run("empty patch returns empty", func(t *testing.T) {
		result := applySemanticDiffToUnifiedPatch("config.json", "")
		assert.Equal(t, "", result)
	})

	t.Run("YAML file gets semantic diff", func(t *testing.T) {
		patch := `@@ -1,2 +1,2 @@
-name: old
+name: new`
		result := applySemanticDiffToUnifiedPatch("config.yaml", patch)
		assert.NotEqual(t, patch, result)
		assert.Contains(t, result, `name: "old" → "new"`)
	})
}

func TestReconstructFromPatch(t *testing.T) {
	t.Run("simple patch", func(t *testing.T) {
		patch := `@@ -1,3 +1,3 @@
 {
-  "name": "old"
+  "name": "new"
 }`
		base, head, ok := reconstructFromPatch(patch)
		require.True(t, ok)
		assert.Contains(t, string(base), `"name": "old"`)
		assert.Contains(t, string(head), `"name": "new"`)
	})

	t.Run("addition only", func(t *testing.T) {
		patch := `@@ -0,0 +1,3 @@
+{
+  "new": true
+}`
		base, head, ok := reconstructFromPatch(patch)
		require.True(t, ok)
		assert.Empty(t, string(base))
		assert.Contains(t, string(head), `"new": true`)
	})

	t.Run("empty patch", func(t *testing.T) {
		_, _, ok := reconstructFromPatch("")
		assert.False(t, ok)
	})
}

func TestSplitDiffByFile(t *testing.T) {
	rawDiff := `diff --git a/config.json b/config.json
index abc..def 100644
--- a/config.json
+++ b/config.json
@@ -1,3 +1,3 @@
 {
-  "name": "old"
+  "name": "new"
 }
diff --git a/main.go b/main.go
index abc..def 100644
--- a/main.go
+++ b/main.go
@@ -1,3 +1,4 @@
 func hello() {
+	fmt.Println("world")
 }`

	sections := splitDiffByFile(rawDiff)
	require.Len(t, sections, 2)
	assert.Equal(t, "config.json", sections[0].filename)
	assert.Equal(t, "main.go", sections[1].filename)
	assert.Contains(t, sections[0].patch, `"name": "old"`)
	assert.Contains(t, sections[1].patch, `fmt.Println`)
}

func TestProcessMultiFileDiff(t *testing.T) {
	rawDiff := `diff --git a/config.json b/config.json
index abc..def 100644
--- a/config.json
+++ b/config.json
@@ -1,3 +1,3 @@
 {
-  "name": "old"
+  "name": "new"
 }
diff --git a/README.md b/README.md
index abc..def 100644
--- a/README.md
+++ b/README.md
@@ -1,3 +1,3 @@
 # Title
-old text
+new text`

	result := processMultiFileDiff(rawDiff)

	// JSON file should get semantic diff
	assert.Contains(t, result, "semantic diff")
	assert.Contains(t, result, `name: "old" → "new"`)

	// Markdown should keep original patch format
	assert.Contains(t, result, "README.md")
	assert.Contains(t, result, "-old text")
	assert.Contains(t, result, "+new text")
}
