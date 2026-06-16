package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/AndersSol/zgx-cli/internal/connect"
	"github.com/AndersSol/zgx-cli/internal/dnsreg"
	"github.com/AndersSol/zgx-cli/internal/install"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func init() {
	rootCmd.AddCommand(
		healthCmd(),
		dnsRegisterCmd(),
	)
}

func healthCmd() *cobra.Command {
	opts := defaultSystemCommandOptions()
	cmd := &cobra.Command{
		Use:   "health <host>",
		Short: "Check SSH connectivity to a device",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			host := args[0]
			hostKey, err := connect.KnownHostsCallback(expandHome(opts.knownHosts))
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			target := connect.Target{Host: host, User: opts.user, Port: opts.port}
			out := cmd.OutOrStdout()
			if err := connect.TestKeyAuth(ctx, target, expandHome(opts.identity), hostKey); err != nil {
				fmt.Fprintf(out, "%s: unreachable: %v\n", host, err)
				return fmt.Errorf("health: %s unreachable: %w", host, err)
			}
			fmt.Fprintf(out, "%s: healthy\n", host)
			return nil
		},
	}
	addSystemSSHFlags(cmd, &opts)
	return cmd
}

func dnsRegisterCmd() *cobra.Command {
	opts := defaultSystemCommandOptions()
	cmd := &cobra.Command{
		Use:   "dns-register <host>",
		Short: "Register the device for stable mDNS discovery",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			host := args[0]
			out := cmd.OutOrStdout()

			fmt.Fprintf(out, "Sudo password for %s@%s: ", opts.user, host)
			passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Fprintln(out)
			if err != nil {
				return fmt.Errorf("read sudo password: %w", err)
			}

			hostKey, err := connect.KnownHostsCallback(expandHome(opts.knownHosts))
			if err != nil {
				return err
			}

			runner := install.SSHRunner{
				Target:         connect.Target{Host: host, User: opts.user, Port: opts.port},
				HostKey:        hostKey,
				PrivateKeyPath: expandHome(opts.identity),
			}
			result, err := dnsreg.Register(context.Background(), runner, string(passwordBytes))
			if err != nil {
				return err
			}

			fmt.Fprintf(out, "Device ID: %s\n", result.Identifier)
			fmt.Fprintf(out, "Service file written: %t\n", result.ServiceFileWritten)
			fmt.Fprintf(out, "Avahi restarted: %t\n", result.AvahiRestarted)
			if result.Note != "" {
				fmt.Fprintf(out, "Note: %s\n", result.Note)
			}
			return nil
		},
	}
	addSystemSSHFlags(cmd, &opts)
	return cmd
}

type systemCommandOptions struct {
	user, identity, knownHosts string
	port                       int
}

func defaultSystemCommandOptions() systemCommandOptions {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return systemCommandOptions{user: "hp", port: 22}
	}
	return systemCommandOptions{
		user:       "hp",
		port:       22,
		identity:   filepath.Join(homeDir, ".ssh", "id_ed25519"),
		knownHosts: filepath.Join(homeDir, ".ssh", "known_hosts"),
	}
}

func addSystemSSHFlags(cmd *cobra.Command, opts *systemCommandOptions) {
	cmd.Flags().StringVar(&opts.user, "user", opts.user, "SSH user on the device")
	cmd.Flags().IntVar(&opts.port, "port", opts.port, "SSH port on the device")
	cmd.Flags().StringVar(&opts.identity, "identity", opts.identity, "private SSH key")
	cmd.Flags().StringVar(&opts.knownHosts, "known-hosts", opts.knownHosts, "known_hosts file")
}
