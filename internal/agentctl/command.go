// Package agentctl implements a local Copilot CLI wrapper.
package agentctl

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

type executor func(context.Context, *exec.Cmd) error

// NewCommand creates the standalone command; it does not configure the MCP server.
func NewCommand(version string) *cobra.Command {
	return newCommand(version, exec.LookPath, func(_ context.Context, cmd *exec.Cmd) error {
		return cmd.Run()
	})
}

func newCommand(version string, lookPath func(string) (string, error), execute executor) *cobra.Command {
	root := &cobra.Command{
		Use: "agentctl", Short: "Run custom agents using the local GitHub Copilot CLI",
		Version: version, SilenceErrors: true, SilenceUsage: true,
		Args: cobra.NoArgs,
	}
	root.CompletionOptions.DisableDefaultCmd = true
	root.AddCommand(&cobra.Command{
		Use: "list", Short: "List bundled profiles (install explicitly before use)", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			for _, id := range profileIDs() {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), id); err != nil {
					return err
				}
			}
			return nil
		},
	})

	var repo, path, prompt, promptFile string
	var allowTools []string
	run := &cobra.Command{
		Use: "run <agent>", Short: "Invoke an installed custom agent in a validated local GitHub checkout",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().Changed("repo") && repo == "" {
				return fmt.Errorf("--repo must have the form owner/repo")
			}
			if cmd.Flags().Changed("path") && path == "" {
				return fmt.Errorf("--path must name a checkout")
			}
			if !agentID.MatchString(args[0]) {
				return fmt.Errorf("agent must be a custom-agent ID containing letters, digits, dots, underscores or hyphens")
			}
			if cmd.Flags().Changed("prompt") == cmd.Flags().Changed("prompt-file") {
				return fmt.Errorf("provide exactly one of --prompt or --prompt-file")
			}
			task := prompt
			if cmd.Flags().Changed("prompt-file") {
				if promptFile == "" {
					return fmt.Errorf("--prompt-file must name a file")
				}
				data, err := os.ReadFile(promptFile)
				if err != nil {
					return fmt.Errorf("read prompt file: %w", err)
				}
				task = string(data)
			}
			if strings.TrimSpace(task) == "" || strings.ContainsRune(task, 0) {
				return fmt.Errorf("prompt must be nonempty and contain no NUL bytes")
			}
			for _, tool := range allowTools {
				if !narrowPermission(tool) {
					return fmt.Errorf("--allow-tool must be read, shell(an exact command) or write(a specific file), without wildcards or comma-separated grants")
				}
			}
			checkout, err := validateCheckout(cmd.Context(), path, repo)
			if err != nil {
				return err
			}
			binary, err := lookPath("copilot")
			if err != nil {
				return fmt.Errorf("copilot executable not found on PATH; install GitHub Copilot CLI and run copilot login: %w", err)
			}
			argv := []string{"--agent", args[0], "--prompt", task}
			for _, tool := range allowTools {
				argv = append(argv, "--allow-tool", tool)
			}
			process := exec.CommandContext(cmd.Context(), binary, argv...)
			process.Dir = checkout
			process.Env = checkoutEnvironment()
			process.Stdin = cmd.InOrStdin()
			process.Stdout = cmd.OutOrStdout()
			process.Stderr = cmd.ErrOrStderr()
			if err := execute(cmd.Context(), process); err != nil {
				if cmd.Context().Err() != nil {
					return fmt.Errorf("copilot execution cancelled: %w", cmd.Context().Err())
				}
				return fmt.Errorf("copilot CLI failed (see its output for authentication, quota or tool-permission errors; no retry performed): %w", err)
			}
			return nil
		},
	}
	run.Flags().StringVar(&repo, "repo", "", "Expected GitHub owner/repo; must match origin")
	run.Flags().StringVar(&path, "path", "", "Local checkout or subdirectory (default: current Git checkout)")
	run.Flags().StringVar(&prompt, "prompt", "", "Task text, passed as one argument")
	run.Flags().StringVar(&promptFile, "prompt-file", "", "Read task text from a file relative to the caller's directory")
	run.Flags().StringArrayVar(&allowTools, "allow-tool", nil, "Explicit permission: read, shell(exact command) or write(specific file); repeatable")
	root.AddCommand(run, installCommand())
	return root
}

var agentID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func narrowPermission(tool string) bool {
	if tool == "read" {
		return true
	}
	kind, filter, ok := strings.Cut(tool, "(")
	if !ok || !strings.HasSuffix(filter, ")") || (kind != "shell" && kind != "write") {
		return false
	}
	filter = strings.TrimSuffix(filter, ")")
	if strings.TrimSpace(filter) == "" || strings.ContainsAny(filter, "*(),\r\n\x00") {
		return false
	}
	if kind == "write" {
		clean := filepath.Clean(filter)
		return clean != "." && clean != ".." && !strings.HasSuffix(filter, "/") && !strings.HasSuffix(filter, `\`)
	}
	return !strings.ContainsAny(filter, ";&|<>`$")
}

// ExitCode preserves Copilot's normal exit status and uses 130 for cancellation.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return 130
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() > 0 {
		return exitErr.ExitCode()
	}
	return 1
}
