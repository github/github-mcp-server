package agentctl

import (
	"crypto/rand"
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

//go:embed profiles/*.agent.md
var profiles embed.FS

func profileIDs() []string {
	entries, _ := profiles.ReadDir("profiles")
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		ids = append(ids, strings.TrimSuffix(entry.Name(), ".agent.md"))
	}
	return ids
}

func installCommand() *cobra.Command {
	var user, dryRun, force bool
	var path string
	cmd := &cobra.Command{
		Use: "install [agent...]", Short: "Install bundled profiles explicitly; no arguments selects all",
		RunE: func(cmd *cobra.Command, ids []string) error {
			if user == cmd.Flags().Changed("path") {
				return fmt.Errorf("select exactly one of --user or --path <checkout>")
			}
			var base, directory string
			if user {
				base = os.Getenv("COPILOT_HOME")
				if base == "" {
					home, err := os.UserHomeDir()
					if err != nil {
						return err
					}
					base = filepath.Join(home, ".copilot")
				}
				directory = "agents"
			} else {
				if path == "" {
					return fmt.Errorf("--path must name a checkout")
				}
				checkout, err := validateCheckout(cmd.Context(), path, "")
				if err != nil {
					return err
				}
				base, directory = checkout, filepath.Join(".github", "agents")
			}
			if len(ids) == 0 {
				ids = profileIDs()
			}
			return installProfiles(base, directory, ids, dryRun, force, cmd.OutOrStdout())
		},
	}
	cmd.Flags().BoolVar(&user, "user", false, "Install into ~/.copilot/agents (or $COPILOT_HOME/agents)")
	cmd.Flags().StringVar(&path, "path", "", "Install into the selected Git checkout's .github/agents")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show destinations without creating directories or files")
	cmd.Flags().BoolVar(&force, "force", false, "Replace existing regular profile files (never symlinks)")
	return cmd
}

func installProfiles(base, directory string, ids []string, dryRun, force bool, out io.Writer) error {
	contents := make(map[string][]byte, len(ids))
	for _, id := range ids {
		if !agentID.MatchString(id) {
			return fmt.Errorf("invalid bundled profile ID %q", id)
		}
		data, err := profiles.ReadFile("profiles/" + id + ".agent.md")
		if err != nil {
			return fmt.Errorf("unknown bundled profile %q; use agentctl list", id)
		}
		if _, duplicate := contents[id]; duplicate {
			return fmt.Errorf("duplicate profile %q", id)
		}
		contents[id] = data
	}
	if dryRun {
		for _, id := range ids {
			if _, err := fmt.Fprintf(out, "Would install %s\n", filepath.Join(base, directory, id+".agent.md")); err != nil {
				return err
			}
		}
		return nil
	}
	if err := os.MkdirAll(base, 0o700); err != nil { // #nosec G703 -- base is the explicitly selected install target; profile writes below are confined by os.Root.
		return err
	}
	root, err := os.OpenRoot(base)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := root.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	// Check all destinations before installing any profiles. Rooted operations
	// also reject directory symlinks that escape the explicitly selected base.
	for _, id := range ids {
		destination := filepath.Join(directory, id+".agent.md")
		info, err := root.Lstat(destination)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err == nil {
			if !info.Mode().IsRegular() {
				return fmt.Errorf("refusing to replace non-regular profile %s", destination)
			}
			if !force {
				return fmt.Errorf("profile %s already exists; use --force to replace it", destination)
			}
		}
	}
	for _, id := range ids {
		destination := filepath.Join(directory, id+".agent.md")
		if err := writeProfile(root, destination, contents[id], force); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "Installed %s\n", filepath.Join(base, destination)); err != nil {
			return err
		}
	}
	return nil
}

func writeProfile(root *os.Root, destination string, content []byte, force bool) error {
	filePath := destination
	if force {
		filePath = filepath.Join(filepath.Dir(destination), ".agentctl-"+rand.Text())
	}
	file, err := root.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	installed := false
	defer func() {
		if !installed {
			_ = root.Remove(filePath)
		}
	}()
	_, writeErr := file.Write(content)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	if force {
		if err := root.Rename(filePath, destination); err != nil {
			return err
		}
	}
	installed = true
	return nil
}
