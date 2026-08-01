package main

import "strings"

// formatToolsetName 将工具集 ID 转换为便于阅读的名称。
// generate_docs.go 和 list_scopes.go 都使用它来保持格式一致。
func formatToolsetName(name string) string {
	switch name {
	case "pull_requests":
		return "Pull Requests"
	case "repos":
		return "Repositories"
	case "code_security":
		return "Code Security"
	case "secret_protection":
		return "Secret Protection"
	case "orgs":
		return "Organizations"
	default:
		// 回退处理：将首字母大写并以空格替换下划线。
		parts := strings.Split(name, "_")
		for i, part := range parts {
			if len(part) > 0 {
				parts[i] = strings.ToUpper(string(part[0])) + part[1:]
			}
		}
		return strings.Join(parts, " ")
	}
}
