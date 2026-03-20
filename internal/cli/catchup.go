package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"dv/internal/config"
	"dv/internal/docker"
	"dv/internal/xdg"
)

var catchupCmd = &cobra.Command{
	Use:   "catchup",
	Short: "Pull latest code and migrate databases",
	Args:  cobra.NoArgs,
	RunE:  catchupRunE,
}

func catchupRunE(cmd *cobra.Command, args []string) error {
	configDir, err := xdg.ConfigDir()
	if err != nil {
		return err
	}
	cfg, err := config.LoadOrCreate(configDir)
	if err != nil {
		return err
	}
	name, _ := cmd.Flags().GetString("name")
	if name == "" {
		name = currentAgentName(cfg)
	}

	if !docker.Exists(name) {
		fmt.Fprintf(cmd.OutOrStdout(), "Container '%s' does not exist. Run 'dv start' first.\n", name)
		return nil
	}
	if !docker.Running(name) {
		fmt.Fprintf(cmd.OutOrStdout(), "Starting container '%s'...\n", name)
		if err := docker.Start(name); err != nil {
			return err
		}
	}

	imgName := cfg.ContainerImages[name]
	var imgCfg config.ImageConfig
	if imgName != "" {
		imgCfg = cfg.Images[imgName]
	} else {
		_, imgCfg, err = resolveImage(cfg, "")
		if err != nil {
			return err
		}
	}
	workdir := imgCfg.Workdir
	if strings.TrimSpace(workdir) == "" {
		workdir = "/var/www/discourse"
	}
	pc, err := requireLifecycleSupport(imgCfg, "catchup")
	if err != nil {
		return fmt.Errorf("'dv catchup': %w", err)
	}
	user := imgCfg.EffectiveUser()

	if pc != nil {
		// Project config catchup — simpler flow, no plugin discovery
		skipConfirm, _ := cmd.Flags().GetBool("yes")
		if !skipConfirm {
			fmt.Fprintln(cmd.OutOrStdout(), "This will run the project catchup hooks.")
			fmt.Fprintln(cmd.OutOrStdout(), "")
			yes, err := promptYesNo(cmd.InOrStdin(), cmd.OutOrStdout(), "Continue? (y/N): ")
			if err != nil {
				return err
			}
			if !yes {
				fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
				return nil
			}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Catching up in container '%s'...\n", name)
		script := buildProjectCatchupScript(pc.Lifecycle.Catchup, pc.Services)
		argv := []string{"bash", "-lc", script}
		if err := docker.ExecInteractive(name, workdir, user, nil, argv); err != nil {
			return fmt.Errorf("container: catchup failed: %w", err)
		}
		return nil
	}

	// Discourse built-in catchup
	// Discover plugins with their own git repos
	findScript := "find plugins -maxdepth 2 -name .git -type d 2>/dev/null | sed 's|/.git$||' | sort"
	pluginOutput, err := docker.ExecOutput(name, workdir, user, nil, []string{"bash", "-c", findScript})
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to discover plugin repos: %v\n", err)
		pluginOutput = ""
	}
	var plugins []string
	for _, line := range strings.Split(strings.TrimSpace(pluginOutput), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			plugins = append(plugins, line)
		}
	}

	// Confirmation prompt
	skipConfirm, _ := cmd.Flags().GetBool("yes")
	if !skipConfirm {
		fmt.Fprintln(cmd.OutOrStdout(), "This will discard ALL local changes, pull latest code, and migrate databases.")
		fmt.Fprintln(cmd.OutOrStdout(), "")
		fmt.Fprintln(cmd.OutOrStdout(), "Repos that will be reset:")
		fmt.Fprintln(cmd.OutOrStdout(), "  - core (discourse)")
		for _, p := range plugins {
			fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", p)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "")
		yes, err := promptYesNo(cmd.InOrStdin(), cmd.OutOrStdout(), "Continue? (y/N): ")
		if err != nil {
			return err
		}
		if !yes {
			fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
			return nil
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Catching up in container '%s'...\n", name)

	script := buildCatchupScript(workdir, plugins)
	argv := []string{"bash", "-lc", script}
	if err := docker.ExecInteractive(name, workdir, user, nil, argv); err != nil {
		return fmt.Errorf("container: catchup failed: %w", err)
	}
	return nil
}

func buildCatchupScript(workdir string, plugins []string) string {
	lines := []string{
		"set -euo pipefail",
		"cleanup() { echo 'Restarting services: unicorn and ember-cli'; sudo /usr/bin/sv start unicorn || sudo sv start unicorn || true; sudo /usr/bin/sv start ember-cli || sudo sv start ember-cli || true; }",
		"trap cleanup EXIT",
		"",
		"sudo /usr/bin/sv force-stop unicorn || sudo sv force-stop unicorn || true",
		"sudo /usr/bin/sv force-stop ember-cli || sudo sv force-stop ember-cli || true",
		"",
		"echo '==> Resetting core...'",
		"git reset --hard",
		"git clean -df",
		"echo '==> Pulling latest core...'",
		"git fetch --prune",
		"git reset --hard @{u}",
	}

	for _, plugin := range plugins {
		quoted := shellQuote(plugin)
		lines = append(lines,
			"",
			fmt.Sprintf("echo %s", shellQuote("==> Resetting "+plugin+"...")),
			fmt.Sprintf("cd %s", quoted),
			"git reset --hard",
			"git clean -df",
			"git fetch --prune",
			"git reset --hard @{u}",
			fmt.Sprintf("cd %s", shellQuote(workdir)),
		)
	}

	lines = append(lines,
		"",
		"echo '==> Installing bundle dependencies...'",
		"bundle install",
		"echo '==> Installing pnpm dependencies...'",
		"pnpm install",
		"",
		"echo '==> Migrating development database...'",
		"bin/rake db:migrate",
		"echo '==> Migrating test database...'",
		"RAILS_ENV=test bin/rake db:migrate",
		"",
		"echo 'Catchup complete!'",
	)

	return strings.Join(lines, "\n")
}

func init() {
	catchupCmd.Flags().String("name", "", "Container name (defaults to selected or default)")
	catchupCmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")
}
