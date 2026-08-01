package main

import (
	"crypto/sha256"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type replacement struct{ old, new string }

var phrases = []replacement{
	{"is the feature flag name for", "是以下功能标志名称："},
	{"is the list of feature flags that", "是以下功能标志列表："},
	{"is the allowlist of feature flags that can be enabled", "是可启用功能标志的允许列表"},
	{"is the single source of truth for", "是以下内容的唯一事实来源："},
	{"creates a tool to", "创建一个工具以"},
	{"creates the tool to", "创建用于"},
	{"creates a resource to", "创建一个资源以"},
	{"creates a prompt to", "创建一个提示以"},
	{"represents the response structure for", "表示以下内容的响应结构："},
	{"represents a single entry in", "表示以下内容中的单个条目："},
	{"returns whether", "返回是否"},
	{"returns a set (map) for", "返回一个集合（map），用于"},
	{"computes the effective set of enabled", "计算有效启用的"},
	{"validates that all", "验证所有"},
	{"ensures all", "确保所有"},
	{"should be", "应当是"},
	{"must be", "必须是"},
	{"must not", "不得"},
	{"can be", "可以"},
	{"are now", "现在"},
	{"is now", "现在是"},
	{"is used to", "用于"},
	{"used by", "由以下内容使用："},
	{"used for", "用于"},
	{"used when", "在以下情况使用："},
	{"used to", "用于"},
	{"if no ", "如果没有"},
	{"If no ", "如果没有"},
	{"if the ", "如果"},
	{"If the ", "如果"},
	{"when the ", "当"},
	{"When the ", "当"},
	{"where the ", "其中"},
	{"with the ", "使用"},
	{"for the ", "用于"},
	{"from the ", "来自"},
	{"to the ", "到"},
	{"of the ", "的"},
	{"and the ", "以及"},
	{"or the ", "或"},
	{" in the ", " 在"},
	{" on the ", " 在"},
	{" by the ", " 由"},
	{" a ", " 一个"},
	{" an ", " 一个"},
	{" the ", " "},
	{"This ", "此"},
	{"This", "此"},
	{"These ", "这些"},
	{"The ", ""},
	{"A ", "一个"},
	{"An ", "一个"},
	{"all ", "所有"},
	{"All ", "所有"},
	{"each ", "每个"},
	{"Every ", "每个"},
	{"only ", "仅"},
	{"Only ", "仅"},
	{"not ", "不"},
	{"Not ", "不"},
	{"and ", "和"},
	{"or ", "或"},
	{"but ", "但"},
	{"because ", "因为"},
	{"while ", "当"},
	{"then ", "然后"},
	{"also ", "也"},
	{"always ", "始终"},
	{"never ", "绝不"},
	{"default ", "默认"},
	{"Default ", "默认"},
	{"enabled", "启用"}, {"disabled", "禁用"}, {"enable", "启用"}, {"disable", "禁用"},
	{"feature flag", "功能标志"}, {"feature flags", "功能标志"},
	{"tool definition", "工具定义"}, {"tool definitions", "工具定义"},
	{"toolset", "工具集"}, {"tools", "工具"}, {"tool", "工具"},
	{"resource", "资源"}, {"resources", "资源"}, {"prompt", "提示"}, {"prompts", "提示"},
	{"repository", "仓库"}, {"repositories", "仓库"}, {"issue", "议题"}, {"issues", "议题"},
	{"pull request", "拉取请求"}, {"pull requests", "拉取请求"},
	{"workflow", "工作流"}, {"workflows", "工作流"}, {"branch", "分支"}, {"branches", "分支"},
	{"request", "请求"}, {"response", "响应"}, {"result", "结果"}, {"results", "结果"},
	{"client", "客户端"}, {"server", "服务器"}, {"handler", "处理器"}, {"function", "函数"},
	{"parameter", "参数"}, {"parameters", "参数"}, {"argument", "参数"}, {"arguments", "参数"},
	{"input", "输入"}, {"output", "输出"}, {"value", "值"}, {"values", "值"},
	{"list", "列出"}, {"get", "获取"}, {"create", "创建"}, {"update", "更新"}, {"delete", "删除"},
	{"read", "读取"}, {"write", "写入"}, {"filter", "筛选"}, {"validate", "验证"},
	{"check", "检查"}, {"setup", "设置"}, {"mock", "模拟"}, {"success", "成功"}, {"error", "错误"},
	{"private", "私有"}, {"public", "公开"}, {"trusted", "受信任"}, {"untrusted", "不受信任"},
	{"required", "必需"}, {"optional", "可选"}, {"empty", "空"}, {"unknown", "未知"},
	{"first", "第一个"}, {"last", "最后一个"}, {"next", "下一个"}, {"previous", "上一个"},
	{"single", "单个"}, {"multiple", "多个"}, {"same", "相同"}, {"new", "新的"},
	{"file", "文件"}, {"path", "路径"}, {"line", "行"}, {"range", "范围"}, {"page", "页"},
	{"content", "内容"}, {"data", "数据"}, {"information", "信息"}, {"metadata", "元数据"},
	{"context", "上下文"}, {"session", "会话"}, {"capability", "能力"}, {"capabilities", "能力"},
	{"call", "调用"}, {"calls", "调用"}, {"return", "返回"}, {"returns", "返回"},
	{"true", "真"}, {"false", "假"}, {"nil", "nil"}, {"map", "map"},
}

func translate(s string) string {
	if strings.HasPrefix(s, "//go:") || strings.HasPrefix(s, "//line ") {
		return s
	}
	for _, p := range phrases {
		s = strings.ReplaceAll(s, p.old, p.new)
	}
	return s
}

func mask(src []byte, groups []*ast.CommentGroup, fset *token.FileSet) []byte {
	var out []byte
	last := 0
	for _, g := range groups {
		for _, c := range g.List {
			start, end := fset.Position(c.Pos()).Offset, fset.Position(c.End()).Offset
			out = append(out, src[last:start]...)
			last = end
		}
	}
	return append(out, src[last:]...)
}

func main() {
	sort.SliceStable(phrases, func(i, j int) bool { return len(phrases[i].old) > len(phrases[j].old) })
	root, _ := os.Getwd()
	var files []string
	filepath.Walk(filepath.Join(root, "pkg", "github"), func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if filepath.Ext(path) == ".go" && !strings.Contains(rel, "__toolsnaps__") && !strings.Contains(rel, "third-party") {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	changed, commentCount := 0, 0
	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			panic(err)
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
		if err != nil {
			panic(err)
		}
		out := append([]byte(nil), src...)
		for _, g := range f.Comments {
			commentCount += len(g.List)
		}
		// Re-run replacement after each byte-offset-changing edit.
		for {
			fset = token.NewFileSet()
			f, err = parser.ParseFile(fset, path, out, parser.ParseComments)
			if err != nil {
				panic(err)
			}
			did := false
			for _, g := range f.Comments {
				for _, c := range g.List {
					start, end := fset.Position(c.Pos()).Offset, fset.Position(c.End()).Offset
					replaced := translate(string(out[start:end]))
					if replaced != string(out[start:end]) {
						out = append(append(out[:start:start], []byte(replaced)...), out[end:]...)
						did = true
						break
					}
				}
				if did {
					break
				}
			}
			if !did {
				break
			}
		}
		fset = token.NewFileSet()
		before, err := parser.ParseFile(fset, path, src, parser.ParseComments)
		if err != nil {
			panic(err)
		}
		fset2 := token.NewFileSet()
		after, err := parser.ParseFile(fset2, path, out, parser.ParseComments)
		if err != nil {
			panic(err)
		}
		b, a := sha256.Sum256(mask(src, before.Comments, fset)), sha256.Sum256(mask(out, after.Comments, fset2))
		if b != a {
			panic("non-comment content changed: " + path)
		}
		if string(src) != string(out) {
			if err := os.WriteFile(path, out, 0644); err != nil {
				panic(err)
			}
			changed++
		}
	}
	fmt.Printf("files=%d changed=%d comments=%d\n", len(files), changed, commentCount)
}
