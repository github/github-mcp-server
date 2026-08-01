// Package toolsnaps 提供测试工具，用于确保工具的 JSON schema 未发生意外变化。
package toolsnaps

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/josephburnett/jd/v2"
)

// Test 检查工具的 JSON schema 是否发生意外变化。
// 它将提供工具序列化后的 JSON 与存储的 snapshot 文件比较。
// 若 UPDATE_TOOLSNAPS 环境变量设置为 "true"，则改为更新 snapshot 文件。
// 若 snapshot 不存在且未运行于 CI，则创建 snapshot 文件。
// 若 snapshot 不存在且运行于 CI（GITHUB_ACTIONS="true"），则返回错误。
// 若 snapshot 存在，则将工具的 JSON 与其比较，并在二者不同时时返回错误。
// 序列化、读取或比较失败时返回错误。
func Test(toolName string, tool any) error {
	toolJSON, err := json.MarshalIndent(tool, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tool %s: %w", toolName, err)
	}

	snapPath := fmt.Sprintf("__toolsnaps__/%s.snap", toolName)

	// 若已设置 UPDATE_TOOLSNAPS，则将工具 JSON 写入 snapshot 文件后退出。
	if os.Getenv("UPDATE_TOOLSNAPS") == "true" {
		return writeSnap(snapPath, toolJSON)
	}

	snapJSON, err := os.ReadFile(snapPath) //nolint:gosec // 文件路径由测试套件控制，因而安全。
	// 若 snapshot 文件不存在，必定是首次运行此测试。
	// 将工具 JSON 写入 snapshot 文件后退出。
	if os.IsNotExist(err) {
		// 若运行于 CI 且没有 snapshot，则返回错误，因为 snapshot 必须与测试一并提交，
		// 而不能只在 CI 运行期间生成却未提交。
		if os.Getenv("GITHUB_ACTIONS") == "true" {
			return fmt.Errorf("tool snapshot does not exist for %s. Please run the tests with UPDATE_TOOLSNAPS=true to create it", toolName)
		}

		return writeSnap(snapPath, toolJSON)
	}

	// 否则将工具 JSON 与 snapshot JSON 比较。
	toolNode, err := jd.ReadJsonString(string(toolJSON))
	if err != nil {
		return fmt.Errorf("failed to parse tool JSON for %s: %w", toolName, err)
	}

	snapNode, err := jd.ReadJsonString(string(snapJSON))
	if err != nil {
		return fmt.Errorf("failed to parse snapshot JSON for %s: %w", toolName, err)
	}

	// jd.Set 支持不考虑顺序地比较数组；公开工具 schema 时不需要关心这一点。
	diff := toolNode.Diff(snapNode, jd.SET).Render()
	if diff != "" {
		// 若存在差异，则返回带有 diff 的错误。
		return fmt.Errorf("tool schema for %s has changed unexpectedly:\n%s\nrun with `UPDATE_TOOLSNAPS=true` if this is expected", toolName, diff)
	}

	return nil
}

func writeSnap(snapPath string, contents []byte) error {
	// 递归排序 JSON key，以确保输出一致。
	// 通过反序列化和重新序列化实现，这确保 Go 的 JSON encoder 会在每层按字母顺序排序所有 map key。
	sortedJSON, err := sortJSONKeys(contents)
	if err != nil {
		return fmt.Errorf("failed to sort JSON keys: %w", err)
	}

	// 确保目录存在。
	if err := os.MkdirAll(filepath.Dir(snapPath), 0700); err != nil {
		return fmt.Errorf("failed to create snapshot directory: %w", err)
	}

	// 写入 snapshot 文件。
	if err := os.WriteFile(snapPath, sortedJSON, 0600); err != nil {
		return fmt.Errorf("failed to write snapshot file: %w", err)
	}

	return nil
}

// sortJSONKeys 通过反序列化为 map[string]any 后重新序列化，递归排序 JSON byte array 中的所有对象 key。
// Go 的 JSON encoder 会自动按字母顺序排序 map key。
func sortJSONKeys(jsonData []byte) ([]byte, error) {
	var data any
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return nil, err
	}

	return json.MarshalIndent(data, "", "  ")
}
