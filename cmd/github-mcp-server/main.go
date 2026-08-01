package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/github/github-mcp-server/internal/buildinfo"
	"github.com/github/github-mcp-server/internal/ghmcp"
	"github.com/github/github-mcp-server/internal/githubapp"
	"github.com/github/github-mcp-server/internal/oauth"
	"github.com/github/github-mcp-server/pkg/github"
	ghhttp "github.com/github/github-mcp-server/pkg/http"
	ghoauth "github.com/github/github-mcp-server/pkg/http/oauth"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// 这些变量由构建过程通过 ldflags 设置。
var version = "version"
var commit = "commit"
var date = "date"

var (
	rootCmd = &cobra.Command{
		Use:     "server",
		Short:   "GitHub MCP 服务器",
		Long:    `一个用于处理各种工具和资源的 GitHub MCP 服务器。`,
		Version: fmt.Sprintf("版本：%s\n提交：%s\n构建日期：%s", version, commit, date),
	}

	stdioCmd = &cobra.Command{
		Use:   "stdio",
		Short: "启动 stdio 服务器",
		Long:  `启动一个通过标准输入/输出流使用 JSON-RPC 消息进行通信的服务器。`,
		RunE: func(_ *cobra.Command, _ []string) error {
			token := viper.GetString("personal_access_token")
			appID := viper.GetString("app-id")
			appInstallationID := viper.GetString("app-installation-id")
			appPrivateKeyPath := viper.GetString("app-private-key-path")
			appPrivateKeyInline := viper.GetString("app-private-key")
			appAuthRequested := appID != "" || appInstallationID != "" || appPrivateKeyPath != "" || appPrivateKeyInline != ""

			oauthClientID := viper.GetString("oauth-client-id")
			oauthClientSecret := viper.GetString("oauth-client-secret")
			// 如果没有显式配置，则回退使用构建时内置的客户端（官方发布版本）。
			// 内置应用注册在 github.com 上，因此它仅适用于默认主机；
			// GHES/ghe.com 用户必须提供自己的 --oauth-client-id。通过 NormalizeHost
			// 识别主机，意味着显式设置 GITHUB_HOST=github.com（或 api.github.com）
			// 仍会被视为默认主机，并继续支持零配置登录。secret 与 id 保持关联，
			// 因此显式提供 id 但未提供 secret 时，绝不会使用内置的 secret。
			if oauthClientID == "" && !appAuthRequested && oauth.NormalizeHost(viper.GetString("host")) == "https://github.com" {
				oauthClientID = buildinfo.OAuthClientID
				oauthClientSecret = buildinfo.OAuthClientSecret
			}
			if token == "" && !appAuthRequested && oauthClientID == "" {
				return errors.New("需要身份认证：请设置 GITHUB_PERSONAL_ACCESS_TOKEN、配置 GitHub App 身份认证，或传入 --oauth-client-id 通过 OAuth 登录")
			}
			if appAuthRequested && token != "" {
				return errors.New("GitHub App 身份认证与 GITHUB_PERSONAL_ACCESS_TOKEN 互斥：只能设置其中一种")
			}
			if appAuthRequested && oauthClientID != "" {
				return errors.New("GitHub App 身份认证与 OAuth 登录（--oauth-client-id）互斥：只能设置其中一种")
			}

			// 如果你想知道为什么这里没有使用 viper.GetStringSlice("toolsets")，
			// 原因是使用 GetStringSlice 时，viper 无法正确处理环境变量中的
			// 逗号分隔值。
			// https://github.com/spf13/viper/issues/380
			//
			// 此外，即使未设置该 flag，viper.UnmarshalKey 也会返回空切片，
			// 但这里需要使用 nil 表示“使用默认值”。因此先检查 IsSet。
			var enabledToolsets []string
			if viper.IsSet("toolsets") {
				if err := viper.UnmarshalKey("toolsets", &enabledToolsets); err != nil {
					return fmt.Errorf("解析 toolsets 失败：%w", err)
				}
			}
			// 否则：enabledToolsets 保持为 nil，表示“使用默认值”

			// 解析 tools（与 toolsets 类似）
			var enabledTools []string
			if viper.IsSet("tools") {
				if err := viper.UnmarshalKey("tools", &enabledTools); err != nil {
					return fmt.Errorf("解析 tools 失败：%w", err)
				}
			}

			// 解析排除的 tools（与 tools 类似）
			var excludeTools []string
			if viper.IsSet("exclude_tools") {
				if err := viper.UnmarshalKey("exclude_tools", &excludeTools); err != nil {
					return fmt.Errorf("解析 exclude-tools 失败：%w", err)
				}
			}

			// 解析启用的 features（与 toolsets 类似）
			var enabledFeatures []string
			if viper.IsSet("features") {
				if err := viper.UnmarshalKey("features", &enabledFeatures); err != nil {
					return fmt.Errorf("解析 features 失败：%w", err)
				}
			}

			ttl := viper.GetDuration("repo-access-cache-ttl")
			stdioServerConfig := ghmcp.StdioServerConfig{
				Version:              version,
				Host:                 viper.GetString("host"),
				Token:                token,
				EnabledToolsets:      enabledToolsets,
				EnabledTools:         enabledTools,
				EnabledFeatures:      enabledFeatures,
				ReadOnly:             viper.GetBool("read-only"),
				ExportTranslations:   viper.GetBool("export-translations"),
				EnableCommandLogging: viper.GetBool("enable-command-logging"),
				LogFilePath:          viper.GetString("log-file"),
				ContentWindowSize:    viper.GetInt("content-window-size"),
				LockdownMode:         viper.GetBool("lockdown-mode"),
				InsidersMode:         viper.GetBool("insiders"),
				ExcludeTools:         excludeTools,
				RepoAccessCacheTTL:   &ttl,
			}

			// 未提供静态 token 时，使用给定的客户端通过 OAuth 登录。
			// 请求的 scopes 默认为完整的受支持集合
			// （不会过滤任何工具）；显式指定范围更小的 --oauth-scopes
			// 会同时缩小授权范围，并隐藏需要其他 scopes 的工具。
			if token == "" && !appAuthRequested {
				scopes := ghoauth.SupportedScopes
				if viper.IsSet("oauth-scopes") {
					if err := viper.UnmarshalKey("oauth-scopes", &scopes); err != nil {
						return fmt.Errorf("解析 oauth-scopes 失败：%w", err)
					}
				}
				oauthConfig := oauth.NewGitHubConfig(
					oauthClientID,
					oauthClientSecret,
					scopes,
					viper.GetString("host"),
					viper.GetInt("oauth-callback-port"),
				)
				stdioServerConfig.OAuthManager = oauth.NewManager(oauthConfig, nil)
				stdioServerConfig.OAuthScopes = scopes
			}

			if appAuthRequested {
				tokenProvider, err := newGitHubAppTokenProvider(appID, appInstallationID, appPrivateKeyPath, appPrivateKeyInline, viper.GetString("host"))
				if err != nil {
					return err
				}
				stdioServerConfig.TokenProvider = tokenProvider
			}

			return ghmcp.RunStdioServer(stdioServerConfig)
		},
	}

	httpCmd = &cobra.Command{
		Use:   "http",
		Short: "启动 HTTP 服务器",
		Long:  `启动一个通过 HTTP 监听 MCP 请求的 HTTP 服务器。`,
		RunE: func(_ *cobra.Command, _ []string) error {
			// 解析 toolsets（处理方式与 stdio 相同，参见对应注释）
			var enabledToolsets []string
			if viper.IsSet("toolsets") {
				if err := viper.UnmarshalKey("toolsets", &enabledToolsets); err != nil {
					return fmt.Errorf("解析 toolsets 失败：%w", err)
				}
			}

			var enabledTools []string
			if viper.IsSet("tools") {
				if err := viper.UnmarshalKey("tools", &enabledTools); err != nil {
					return fmt.Errorf("解析 tools 失败：%w", err)
				}
			}

			var excludeTools []string
			if viper.IsSet("exclude_tools") {
				if err := viper.UnmarshalKey("exclude_tools", &excludeTools); err != nil {
					return fmt.Errorf("解析 exclude-tools 失败：%w", err)
				}
			}

			var enabledFeatures []string
			if viper.IsSet("features") {
				if err := viper.UnmarshalKey("features", &enabledFeatures); err != nil {
					return fmt.Errorf("解析 features 失败：%w", err)
				}
			}

			ttl := viper.GetDuration("repo-access-cache-ttl")
			httpConfig := ghhttp.ServerConfig{
				Version:              version,
				Host:                 viper.GetString("host"),
				Port:                 viper.GetInt("port"),
				ListenHost:           viper.GetString("listen-host"),
				BaseURL:              viper.GetString("base-url"),
				ResourcePath:         viper.GetString("base-path"),
				ExportTranslations:   viper.GetBool("export-translations"),
				EnableCommandLogging: viper.GetBool("enable-command-logging"),
				LogFilePath:          viper.GetString("log-file"),
				ContentWindowSize:    viper.GetInt("content-window-size"),
				LockdownMode:         viper.GetBool("lockdown-mode"),
				RepoAccessCacheTTL:   &ttl,
				ScopeChallenge:       viper.GetBool("scope-challenge"),
				ReadOnly:             viper.GetBool("read-only"),
				EnabledToolsets:      enabledToolsets,
				EnabledTools:         enabledTools,
				ExcludeTools:         excludeTools,
				EnabledFeatures:      enabledFeatures,
				InsidersMode:         viper.GetBool("insiders"),
				TrustProxyHeaders:    viper.GetBool("trust-proxy-headers"),
			}

			return ghhttp.RunHTTPServer(httpConfig)
		},
	}
)

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.SetGlobalNormalizationFunc(wordSepNormalizeFunc)

	rootCmd.SetVersionTemplate("{{.Short}}\n{{.Version}}\n")

	// 添加由所有命令共享的全局 flags
	rootCmd.PersistentFlags().StringSlice("toolsets", nil, github.GenerateToolsetsHelp())
	rootCmd.PersistentFlags().StringSlice("tools", nil, "要启用的具体工具列表，使用逗号分隔")
	rootCmd.PersistentFlags().StringSlice("exclude-tools", nil, "无论其他设置如何都要禁用的工具名称列表，使用逗号分隔")
	rootCmd.PersistentFlags().StringSlice("features", nil, "要启用的 feature flags 列表，使用逗号分隔")
	rootCmd.PersistentFlags().Bool("read-only", false, "将服务器限制为只读操作")
	rootCmd.PersistentFlags().String("log-file", "", "日志文件路径")
	rootCmd.PersistentFlags().Bool("enable-command-logging", false, "启用后，服务器会将所有命令请求和响应记录到日志文件")
	rootCmd.PersistentFlags().Bool("export-translations", false, "将翻译内容保存到 JSON 文件")
	rootCmd.PersistentFlags().String("gh-host", "", "指定 GitHub 主机名（例如 GitHub Enterprise）")
	rootCmd.PersistentFlags().Int("content-window-size", 5000, "指定内容窗口大小")
	rootCmd.PersistentFlags().Bool("lockdown-mode", false, "启用 lockdown 模式")
	rootCmd.PersistentFlags().Bool("insiders", false, "启用 insiders 功能")
	rootCmd.PersistentFlags().Duration("repo-access-cache-ttl", 5*time.Minute, "覆盖仓库访问缓存 TTL（例如 1m，设置为 0s 表示禁用）")

	// stdio 专用的 OAuth flags。提供 --oauth-client-id（而不是 token）
	// 可在首次使用时通过基于浏览器的 OAuth 流程登录。同时适用于
	// OAuth Apps 和 GitHub Apps。
	stdioCmd.Flags().String("oauth-client-id", "", "OAuth App 或 GitHub App client ID，在未设置 token 时启用交互式 OAuth 登录")
	stdioCmd.Flags().String("oauth-client-secret", "", "OAuth client secret，如果应用需要则提供（对于分发式客户端，它是公开的非机密凭证）")
	stdioCmd.Flags().StringSlice("oauth-scopes", nil, "要请求的 OAuth scopes，使用逗号分隔；同时会将工具过滤为这些 scopes。默认使用完整的受支持集合")
	stdioCmd.Flags().Int("oauth-callback-port", 0, "OAuth 回调服务器的固定本地端口。默认使用随机端口；通过 Docker 映射时请设置固定端口")

	// private key 没有对应的 flag，因为通过 argv 传递会导致其暴露。
	stdioCmd.Flags().String("app-id", "", "GitHub App ID 或 client ID，用于启用非交互式的服务器到服务器身份认证")
	stdioCmd.Flags().String("app-installation-id", "", "用于生成 installation access token 的 GitHub App installation ID")
	stdioCmd.Flags().String("app-private-key-path", "", "GitHub App private key（PEM）的路径。推荐优先于 GITHUB_APP_PRIVATE_KEY：可避免密钥出现在命令行和环境变量中")

	// HTTP 专用 flags
	httpCmd.Flags().Int("port", 8082, "HTTP 服务器端口")
	httpCmd.Flags().String("listen-host", "", "HTTP 服务器绑定的主机地址（例如 127.0.0.1）。为空时绑定所有网络接口。")
	httpCmd.Flags().String("base-url", "", "该服务器可被公开访问的基础 URL（用于 OAuth resource metadata）")
	httpCmd.Flags().String("base-path", "", "HTTP 服务器对外可见的基础路径（用于 OAuth resource metadata）")
	httpCmd.Flags().Bool("scope-challenge", false, "启用 OAuth scope challenge 响应")
	httpCmd.Flags().Bool("trust-proxy-headers", false, "构造 OAuth resource metadata URL 时采用 X-Forwarded-Host 和 X-Forwarded-Proto。仅当服务器部署在可信代理之后，并且代理会设置这些 Header 时启用。设置 --base-url 后将忽略此选项。")

	// 将 flag 绑定到 viper
	_ = viper.BindPFlag("toolsets", rootCmd.PersistentFlags().Lookup("toolsets"))
	_ = viper.BindPFlag("tools", rootCmd.PersistentFlags().Lookup("tools"))
	_ = viper.BindPFlag("exclude_tools", rootCmd.PersistentFlags().Lookup("exclude-tools"))
	_ = viper.BindPFlag("features", rootCmd.PersistentFlags().Lookup("features"))
	_ = viper.BindPFlag("read-only", rootCmd.PersistentFlags().Lookup("read-only"))
	_ = viper.BindPFlag("log-file", rootCmd.PersistentFlags().Lookup("log-file"))
	_ = viper.BindPFlag("enable-command-logging", rootCmd.PersistentFlags().Lookup("enable-command-logging"))
	_ = viper.BindPFlag("export-translations", rootCmd.PersistentFlags().Lookup("export-translations"))
	_ = viper.BindPFlag("host", rootCmd.PersistentFlags().Lookup("gh-host"))
	_ = viper.BindPFlag("content-window-size", rootCmd.PersistentFlags().Lookup("content-window-size"))
	_ = viper.BindPFlag("lockdown-mode", rootCmd.PersistentFlags().Lookup("lockdown-mode"))
	_ = viper.BindPFlag("insiders", rootCmd.PersistentFlags().Lookup("insiders"))
	_ = viper.BindPFlag("repo-access-cache-ttl", rootCmd.PersistentFlags().Lookup("repo-access-cache-ttl"))
	_ = viper.BindPFlag("oauth-client-id", stdioCmd.Flags().Lookup("oauth-client-id"))
	_ = viper.BindPFlag("oauth-client-secret", stdioCmd.Flags().Lookup("oauth-client-secret"))
	_ = viper.BindPFlag("oauth-scopes", stdioCmd.Flags().Lookup("oauth-scopes"))
	_ = viper.BindPFlag("oauth-callback-port", stdioCmd.Flags().Lookup("oauth-callback-port"))
	_ = viper.BindPFlag("app-id", stdioCmd.Flags().Lookup("app-id"))
	_ = viper.BindPFlag("app-installation-id", stdioCmd.Flags().Lookup("app-installation-id"))
	_ = viper.BindPFlag("app-private-key-path", stdioCmd.Flags().Lookup("app-private-key-path"))
	_ = viper.BindPFlag("port", httpCmd.Flags().Lookup("port"))
	_ = viper.BindPFlag("listen-host", httpCmd.Flags().Lookup("listen-host"))
	_ = viper.BindPFlag("base-url", httpCmd.Flags().Lookup("base-url"))
	_ = viper.BindPFlag("base-path", httpCmd.Flags().Lookup("base-path"))
	_ = viper.BindPFlag("scope-challenge", httpCmd.Flags().Lookup("scope-challenge"))
	_ = viper.BindPFlag("trust-proxy-headers", httpCmd.Flags().Lookup("trust-proxy-headers"))
	// 添加子命令
	rootCmd.AddCommand(stdioCmd)
	rootCmd.AddCommand(httpCmd)
}

func initConfig() {
	// 初始化 Viper 配置
	viper.SetEnvPrefix("github")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func newGitHubAppTokenProvider(appID, installationID, keyPath, keyInline, host string) (func() string, error) {
	keyBytes, err := loadAppPrivateKey(keyPath, keyInline)
	if err != nil {
		return nil, err
	}

	apiHost, err := utils.NewAPIHost(host)
	if err != nil {
		return nil, fmt.Errorf("解析用于 GitHub App 身份认证的主机失败：%w", err)
	}
	restURL, err := apiHost.BaseRESTURL(context.Background())
	if err != nil {
		return nil, fmt.Errorf("解析用于 GitHub App 身份认证的 REST URL 失败：%w", err)
	}

	provider, err := githubapp.NewProvider(githubapp.Config{
		AppID:          appID,
		InstallationID: installationID,
		PrivateKeyPEM:  keyBytes,
		BaseRESTURL:    restURL.String(),
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("配置 GitHub App 身份认证失败：%w", err)
	}
	return provider.AccessToken, nil
}

func loadAppPrivateKey(path, inline string) ([]byte, error) {
	switch {
	case path != "":
		data, err := os.ReadFile(path) //#nosec G304 -- 由操作者提供的、指向其自有密钥的路径
		if err != nil {
			return nil, fmt.Errorf("读取 GitHub App private key 文件失败：%w", err)
		}
		return data, nil
	case inline != "":
		return []byte(strings.ReplaceAll(inline, `\n`, "\n")), nil
	default:
		return nil, errors.New("GitHub App 身份认证需要 private key：请设置 GITHUB_APP_PRIVATE_KEY_PATH（推荐）或 GITHUB_APP_PRIVATE_KEY")
	}
}

func wordSepNormalizeFunc(_ *pflag.FlagSet, name string) pflag.NormalizedName {
	from := []string{"_"}
	to := "-"
	for _, sep := range from {
		name = strings.ReplaceAll(name, sep, to)
	}
	return pflag.NormalizedName(name)
}
