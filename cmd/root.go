// Package cmd implements the zgx CLI, a thin front end over the engine in
// internal/. Front ends (CLI, TUI, future macOS app) keep no logic themselves.
//
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var version = "dev"

// rootCmd is the base command run as `zgx`.
var rootCmd = &cobra.Command{
	Use:     "zgx",
	Short:   "Configure HP ZGX nano over SSH",
	Version: version,
	Long: `zgx discovers, connects to, and configures HP ZGX nano devices over SSH.

Portable CLI ported from HPInc/ZGX-Toolkit (X11/MIT).`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command and returns the error to main, which owns the
// exit code. This keeps command execution testable (no os.Exit here).
func Execute() error {
	return rootCmd.Execute()
}

// stubCmd builds an unimplemented subcommand. It returns an error so the exit
// code is honest (unimplemented is not success) until logic is added.
func stubCmd(use, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return fmt.Errorf("%s: not implemented yet", cmd.Name())
		},
	}
}
