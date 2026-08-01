package main

import (
	"bufio"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type (
	// SchemaResponse 表示包含工具的顶层响应。
	SchemaResponse struct {
		Result  Result `json:"result"`
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
	}

	// Result 包含可用工具列表。
	Result struct {
		Tools []Tool `json:"tools"`
	}

	// Tool 表示单个命令及其 schema。
	Tool struct {
		Name        string      `json:"name"`
		Description string      `json:"description"`
		InputSchema InputSchema `json:"inputSchema"`
	}

	// InputSchema 定义工具输入参数的结构。
	InputSchema struct {
		Type                 string              `json:"type"`
		Properties           map[string]Property `json:"properties"`
		Required             []string            `json:"required"`
		AdditionalProperties bool                `json:"additionalProperties"`
		Schema               string              `json:"$schema"`
	}

	// Property 定义单个参数的类型和约束。
	Property struct {
		Type        string        `json:"type"`
		Description string        `json:"description"`
		Enum        []string      `json:"enum,omitempty"`
		Minimum     *float64      `json:"minimum,omitempty"`
		Maximum     *float64      `json:"maximum,omitempty"`
		Items       *PropertyItem `json:"items,omitempty"`
	}

	// PropertyItem 定义数组属性中元素的类型。
	PropertyItem struct {
		Type                 string              `json:"type"`
		Properties           map[string]Property `json:"properties,omitempty"`
		Required             []string            `json:"required,omitempty"`
		AdditionalProperties bool                `json:"additionalProperties,omitempty"`
	}

	// JSONRPCRequest 表示 JSON-RPC 2.0 请求。
	JSONRPCRequest struct {
		JSONRPC string        `json:"jsonrpc"`
		ID      int           `json:"id"`
		Method  string        `json:"method"`
		Params  RequestParams `json:"params"`
	}

	// RequestParams 包含工具名称和参数。
	RequestParams struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}

	// Content 与文本内容响应的格式相匹配。
	Content struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}

	ResponseResult struct {
		Content []Content `json:"content"`
	}

	Response struct {
		Result  ResponseResult `json:"result"`
		JSONRPC string         `json:"jsonrpc"`
		ID      int            `json:"id"`
	}
)

var (
	// 创建根命令。
	rootCmd = &cobra.Command{
		Use:   "mcpcurl",
		Short: "CLI tool with dynamically generated commands",
		Long:  "A CLI tool for interacting with MCP API based on dynamically loaded schemas",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			// 跳过 help 和 completion 命令的验证。
			if cmd.Name() == "help" || cmd.Name() == "completion" {
				return nil
			}

			// 检查是否提供必需的全局标志。
			serverCmd, _ := cmd.Flags().GetString("stdio-server-cmd")
			if serverCmd == "" {
				return fmt.Errorf("--stdio-server-cmd is required")
			}
			return nil
		},
	}

	// 添加 schema 命令。
	schemaCmd = &cobra.Command{
		Use:   "schema",
		Short: "Fetch schema from MCP server",
		Long:  "Fetches the tools schema from the MCP server specified by --stdio-server-cmd",
		RunE: func(cmd *cobra.Command, _ []string) error {
			serverCmd, _ := cmd.Flags().GetString("stdio-server-cmd")
			if serverCmd == "" {
				return fmt.Errorf("--stdio-server-cmd is required")
			}

			// 构建 tools/list 的 JSON-RPC 请求。
			jsonRequest, err := buildJSONRPCRequest("tools/list", "", nil)
			if err != nil {
				return fmt.Errorf("failed to build JSON-RPC request: %w", err)
			}

			// 执行服务器命令并传入 JSON-RPC 请求。
			response, err := executeServerCommand(serverCmd, jsonRequest)
			if err != nil {
				return fmt.Errorf("error executing server command: %w", err)
			}

			// 输出响应。
			fmt.Println(response)
			return nil
		},
	}

	// 创建 tools 命令。
	toolsCmd = &cobra.Command{
		Use:   "tools",
		Short: "Access available tools",
		Long:  "Contains all dynamically generated tool commands from the schema",
	}
)

func main() {
	rootCmd.AddCommand(schemaCmd)

	// 添加 stdio 服务器命令的全局标志。
	rootCmd.PersistentFlags().String("stdio-server-cmd", "", "Shell command to invoke MCP server via stdio (required)")
	_ = rootCmd.MarkPersistentFlagRequired("stdio-server-cmd")

	// 添加美化输出的全局标志。
	rootCmd.PersistentFlags().Bool("pretty", true, "Pretty print MCP response (only for JSON or JSONL responses)")

	// 将 tools 命令添加到根命令。
	rootCmd.AddCommand(toolsCmd)

	// 执行一次根命令以解析标志。
	_ = rootCmd.ParseFlags(os.Args[1:])

	// 获取 pretty 标志。
	prettyPrint, err := rootCmd.Flags().GetBool("pretty")
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error getting pretty flag: %v\n", err)
		os.Exit(1)
	}
	// 获取服务器命令。
	serverCmd, err := rootCmd.Flags().GetString("stdio-server-cmd")
	if err == nil && serverCmd != "" {
		// 从服务器获取 schema。
		jsonRequest, err := buildJSONRPCRequest("tools/list", "", nil)
		if err == nil {
			response, err := executeServerCommand(serverCmd, jsonRequest)
			if err == nil {
				// 解析 schema 响应。
				var schemaResp SchemaResponse
				if err := json.Unmarshal([]byte(response), &schemaResp); err == nil {
					// 将所有生成的命令作为 tools 的子命令添加。
					for _, tool := range schemaResp.Result.Tools {
						addCommandFromTool(toolsCmd, &tool, prettyPrint)
					}
				}
			}
		}
	}

	// 执行。
	if err := rootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error executing command: %v\n", err)
		os.Exit(1)
	}
}

// addCommandFromTool 根据工具 schema 创建 cobra 命令。
func addCommandFromTool(toolsCmd *cobra.Command, tool *Tool, prettyPrint bool) {
	// 根据工具创建命令。
	cmd := &cobra.Command{
		Use:   tool.Name,
		Short: tool.Description,
		Run: func(cmd *cobra.Command, _ []string) {
			// 根据标志构建参数映射。
			arguments, err := buildArgumentsMap(cmd, tool)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "failed to build arguments map: %v\n", err)
				return
			}

			jsonData, err := buildJSONRPCRequest("tools/call", tool.Name, arguments)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "failed to build JSONRPC request: %v\n", err)
				return
			}

			// 执行服务器命令。
			serverCmd, err := cmd.Flags().GetString("stdio-server-cmd")
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "failed to get stdio-server-cmd: %v\n", err)
				return
			}
			response, err := executeServerCommand(serverCmd, jsonData)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "error executing server command: %v\n", err)
				return
			}
			if err := printResponse(response, prettyPrint); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "error printing response: %v\n", err)
				return
			}
		},
	}

	// 为此命令初始化 viper。
	viperInit := func() {
		viper.Reset()
		viper.AutomaticEnv()
		viper.SetEnvPrefix(strings.ToUpper(tool.Name))
		viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	}

	// 直接调用初始化函数而不使用 cobra.OnInitialize，以避免命令间冲突。
	viperInit()

	// 根据 schema 属性添加标志。
	for name, prop := range tool.InputSchema.Properties {
		isRequired := slices.Contains(tool.InputSchema.Required, name)

		// 补充描述以表明参数是否可选。
		description := prop.Description
		if !isRequired {
			description += " (optional)"
		}

		switch prop.Type {
		case "string":
			cmd.Flags().String(name, "", description)
			if len(prop.Enum) > 0 {
				// 在 PreRun 中添加枚举值验证。
				cmd.PreRunE = func(cmd *cobra.Command, _ []string) error {
					for flagName, property := range tool.InputSchema.Properties {
						if len(property.Enum) > 0 {
							value, _ := cmd.Flags().GetString(flagName)
							if value != "" && !slices.Contains(property.Enum, value) {
								return fmt.Errorf("%s must be one of: %s", flagName, strings.Join(property.Enum, ", "))
							}
						}
					}
					return nil
				}
			}
		case "number":
			cmd.Flags().Float64(name, 0, description)
		case "integer":
			cmd.Flags().Int64(name, 0, description)
		case "boolean":
			cmd.Flags().Bool(name, false, description)
		case "array":
			if prop.Items != nil {
				switch prop.Items.Type {
				case "string":
					cmd.Flags().StringSlice(name, []string{}, description)
				case "object":
					cmd.Flags().String(name+"-json", "", description+" (provide as JSON array)")
				}
			}
		}

		if isRequired {
			_ = cmd.MarkFlagRequired(name)
		}

		// 将标志绑定到 viper。
		_ = viper.BindPFlag(name, cmd.Flags().Lookup(name))
	}

	// 将命令添加到根命令。
	toolsCmd.AddCommand(cmd)
}

// buildArgumentsMap 将标志值提取到参数映射中。
func buildArgumentsMap(cmd *cobra.Command, tool *Tool) (map[string]any, error) {
	arguments := make(map[string]any)

	for name, prop := range tool.InputSchema.Properties {
		switch prop.Type {
		case "string":
			if value, _ := cmd.Flags().GetString(name); value != "" {
				arguments[name] = value
			}
		case "number":
			if value, _ := cmd.Flags().GetFloat64(name); value != 0 {
				arguments[name] = value
			}
		case "integer":
			if value, _ := cmd.Flags().GetInt64(name); value != 0 {
				arguments[name] = value
			}
		case "boolean":
			// 对于布尔值，需要检查是否显式设置。
			if cmd.Flags().Changed(name) {
				value, _ := cmd.Flags().GetBool(name)
				arguments[name] = value
			}
		case "array":
			if prop.Items != nil {
				switch prop.Items.Type {
				case "string":
					if values, _ := cmd.Flags().GetStringSlice(name); len(values) > 0 {
						arguments[name] = values
					}
				case "object":
					if jsonStr, _ := cmd.Flags().GetString(name + "-json"); jsonStr != "" {
						var jsonArray []any
						if err := json.Unmarshal([]byte(jsonStr), &jsonArray); err != nil {
							return nil, fmt.Errorf("error parsing JSON for %s: %w", name, err)
						}
						arguments[name] = jsonArray
					}
				}
			}
		}
	}

	return arguments, nil
}

// buildJSONRPCRequest 使用给定的工具名称和参数创建 JSON-RPC 请求。
func buildJSONRPCRequest(method, toolName string, arguments map[string]any) (string, error) {
	id, err := rand.Int(rand.Reader, big.NewInt(10000))
	if err != nil {
		return "", fmt.Errorf("failed to generate random ID: %w", err)
	}
	request := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      int(id.Int64()), // 介于 0 和 9999 之间的随机 ID
		Method:  method,
		Params: RequestParams{
			Name:      toolName,
			Arguments: arguments,
		},
	}
	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON request: %w", err)
	}
	return string(jsonData), nil
}

// executeServerCommand 运行指定命令，执行 MCP 初始化握手，将 JSON 请求发送到 stdin，
// 并从 stdout 返回响应。
func executeServerCommand(cmdStr, jsonRequest string) (string, error) {
	// 将命令字符串拆分为命令和参数。
	cmdParts := strings.Fields(cmdStr)
	if len(cmdParts) == 0 {
		return "", fmt.Errorf("empty command")
	}

	cmd := exec.Command(cmdParts[0], cmdParts[1:]...) //nolint:gosec //mcpcurl is a test command that needs to execute arbitrary shell commands

	// 设置 stdin 管道。
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	// 设置用于逐行读取的 stdout 管道。
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// stderr 仍使用缓冲区。
	var stderr strings.Builder
	cmd.Stderr = &stderr

	// 启动命令。
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start command: %w", err)
	}

	// 确保在每条返回路径上清理子进程。
	// 必须在 Wait 前关闭 stdin，以便服务器收到 EOF 并退出；EOF 时的非零退出状态是预期行为，故忽略该错误。
	defer func() {
		_ = stdin.Close()
		_ = cmd.Wait()
	}()

	// 使用大缓冲区 scanner 读取 JSON-RPC 响应。
	scanner := bufio.NewScanner(stdoutPipe)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024) // 最大行大小为 1MB

	// 第 1 步：发送 MCP 初始化请求。
	initReq, err := buildInitializeRequest()
	if err != nil {
		return "", fmt.Errorf("failed to build initialize request: %w", err)
	}
	if _, err := io.WriteString(stdin, initReq+"\n"); err != nil {
		return "", fmt.Errorf("failed to write initialize request: %w", err)
	}

	// 第 2 步：读取初始化响应（跳过任何服务器通知）。
	if _, err := readJSONRPCResponse(scanner); err != nil {
		return "", fmt.Errorf("failed to read initialize response: %w, stderr: %s", err, stderr.String())
	}

	// 第 3 步：发送已初始化通知。
	if _, err := io.WriteString(stdin, buildInitializedNotification()+"\n"); err != nil {
		return "", fmt.Errorf("failed to write initialized notification: %w", err)
	}

	// 第 4 步：发送实际请求。
	if _, err := io.WriteString(stdin, jsonRequest+"\n"); err != nil {
		return "", fmt.Errorf("failed to write request: %w", err)
	}

	// 第 5 步：读取实际响应（跳过任何服务器通知）。
	response, err := readJSONRPCResponse(scanner)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w, stderr: %s", err, stderr.String())
	}

	return response, nil
}

// buildInitializeRequest 创建 MCP 初始化握手请求。
func buildInitializeRequest() (string, error) {
	id, err := rand.Int(rand.Reader, big.NewInt(10000))
	if err != nil {
		return "", fmt.Errorf("failed to generate random ID: %w", err)
	}
	msg := map[string]any{
		"jsonrpc": "2.0",
		"id":      int(id.Int64()),
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    "mcpcurl",
				"version": "0.1.0",
			},
		},
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return "", fmt.Errorf("failed to marshal initialize request: %w", err)
	}
	return string(data), nil
}

// buildInitializedNotification 创建 MCP 已初始化通知。
func buildInitializedNotification() string {
	return `{"jsonrpc":"2.0","method":"notifications/initialized"}`
}

// readJSONRPCResponse 从 scanner 读取行，跳过服务器发起的通知（没有 "id" 字段的消息），
// 并返回第一个响应。
func readJSONRPCResponse(scanner *bufio.Scanner) (string, error) {
	for scanner.Scan() {
		line := scanner.Text()
		// JSON-RPC 响应具有 "id" 字段，通知没有。
		var msg map[string]json.RawMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			return "", fmt.Errorf("failed to parse JSON-RPC message: %w", err)
		}
		if _, hasID := msg["id"]; hasID {
			if errField, hasErr := msg["error"]; hasErr {
				return "", fmt.Errorf("server returned error: %s", string(errField))
			}
			return line, nil
		}
		// 没有 "id"，这是通知，跳过它。
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("unexpected end of output")
}

func printResponse(response string, prettyPrint bool) error {
	if !prettyPrint {
		fmt.Println(response)
		return nil
	}

	// 解析 JSON 响应。
	var resp Response
	if err := json.Unmarshal([]byte(response), &resp); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	// 从类型为 "text" 的内容项中提取文本。
	for _, content := range resp.Result.Content {
		if content.Type == "text" {
			var textContentObj map[string]any
			err := json.Unmarshal([]byte(content.Text), &textContentObj)

			if err == nil {
				prettyText, err := json.MarshalIndent(textContentObj, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to pretty print text content: %w", err)
				}
				fmt.Println(string(prettyText))
				continue
			}

			// 回退为按 JSONL 解析。
			var textContentList []map[string]any
			if err := json.Unmarshal([]byte(content.Text), &textContentList); err != nil {
				return fmt.Errorf("failed to parse text content as a list: %w", err)
			}
			prettyText, err := json.MarshalIndent(textContentList, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to pretty print array content: %w", err)
			}
			fmt.Println(string(prettyText))
		}
	}

	// 如果未找到文本内容，则打印原始响应。
	if len(resp.Result.Content) == 0 {
		fmt.Println(response)
	}

	return nil
}
