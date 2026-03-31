package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "dv",
	Short:         "dv: manage local dev containers",
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if verbose, _ := cmd.Flags().GetBool("verbose"); verbose {
			os.Setenv("DV_VERBOSE", "1")
		}
	},
	CompletionOptions: cobra.CompletionOptions{
		DisableDefaultCmd: true,
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func addPersistentFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")
}

func init() {
	addPersistentFlags(rootCmd)

	rootCmd.AddGroup(
		&cobra.Group{ID: "container", Title: "Container Commands:"},
		&cobra.Group{ID: "workflow", Title: "Workflow Commands:"},
		&cobra.Group{ID: "discourse", Title: "Discourse Commands:"},
		&cobra.Group{ID: "tools", Title: "Tools & Configuration:"},
	)

	// Custom usage template that keeps the command list aligned by padding only the
	// primary command name; aliases are shown after the description to avoid
	// breaking column alignment.
	rootCmd.SetUsageTemplate(`Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{range .Groups}}
{{$gid := .ID}}
{{.Title}}{{range $.Commands}}{{if (and (eq .GroupID $gid) .IsAvailableCommand)}}
  {{rpad .Name $.NamePadding}} {{.Short}}{{if gt (len .Aliases) 0}} (aliases: {{.Aliases}}){{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`)

	// Container lifecycle
	addToGroup("container", buildCmd, startCmd, stopCmd, restartCmd, enterCmd,
		runCmd, removeCmd, newCmd, listCmd, selectCmd, renameCmd, psCmd)

	// Development workflow
	addToGroup("workflow", branchCmd, prCmd, catchupCmd, resetCmd, extractCmd,
		importCmd, copyCmd, runAgentCmd)

	// Discourse-specific
	addToGroup("discourse", mailCmd, exposeCmd)

	// Tools & configuration
	addToGroup("tools", configCmd, imageCmd, dataCmd, pullCmd, updateCmd,
		tuiCmd, serveCmd, versionCmd)

	setupUpdateChecks()
	setupUpgradeCommand()
}

func addToGroup(group string, cmds ...*cobra.Command) {
	for _, cmd := range cmds {
		cmd.GroupID = group
		rootCmd.AddCommand(cmd)
	}
}

func exitIfErr(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
