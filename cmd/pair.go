package cmd

import (
	"context"
	"fmt"

	"github.com/AndersSol/zgx-cli/internal/connect"
	"github.com/AndersSol/zgx-cli/internal/install"
	"github.com/AndersSol/zgx-cli/internal/pairing"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(
		pairCmd(),
		unpairCmd(),
		pairDetailsCmd(),
	)
}

func pairCmd() *cobra.Command {
	opts := defaultSystemCommandOptions()
	cmd := &cobra.Command{
		Use:   "pair <host>",
		Short: "Pair two devices over ConnectX",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			password, err := readSudoPassword(out)
			if err != nil {
				return err
			}

			runner, err := buildPairRunner(args[0], opts)
			if err != nil {
				return err
			}

			nics, err := pairing.Pair(context.Background(), runner, password)
			if err != nil {
				return err
			}

			fmt.Fprintln(out, "Configured ConnectX NICs:")
			writeNICs(out, nics)
			fmt.Fprintln(out, "Netplan written and applied.")
			return nil
		},
	}
	addSystemSSHFlags(cmd, &opts)
	return cmd
}

func unpairCmd() *cobra.Command {
	opts := defaultSystemCommandOptions()
	cmd := &cobra.Command{
		Use:   "unpair <host>",
		Short: "Remove ConnectX pairing",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			password, err := readSudoPassword(out)
			if err != nil {
				return err
			}

			runner, err := buildPairRunner(args[0], opts)
			if err != nil {
				return err
			}

			if err := pairing.Unpair(context.Background(), runner, password); err != nil {
				return err
			}

			fmt.Fprintln(out, "ConnectX configuration removed.")
			return nil
		},
	}
	addSystemSSHFlags(cmd, &opts)
	return cmd
}

func pairDetailsCmd() *cobra.Command {
	opts := defaultSystemCommandOptions()
	cmd := &cobra.Command{
		Use:   "pair-details <host>",
		Short: "Show pairing details and ConnectX NICs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			runner, err := buildPairRunner(args[0], opts)
			if err != nil {
				return err
			}

			nics, err := pairing.PairDetails(context.Background(), runner)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if len(nics) == 0 {
				fmt.Fprintln(out, "No ConnectX NICs found.")
				return nil
			}
			writeNICs(out, nics)
			return nil
		},
	}
	addSystemSSHFlags(cmd, &opts)
	return cmd
}

func buildPairRunner(host string, opts systemCommandOptions) (install.SSHRunner, error) {
	hostKey, err := connect.KnownHostsCallback(expandHome(opts.knownHosts))
	if err != nil {
		return install.SSHRunner{}, err
	}
	return install.SSHRunner{
		Target:         connect.Target{Host: host, User: opts.user, Port: opts.port},
		HostKey:        hostKey,
		PrivateKeyPath: expandHome(opts.identity),
	}, nil
}

func writeNICs(out interface{ Write([]byte) (int, error) }, nics []pairing.NIC) {
	for _, nic := range nics {
		ip := nic.IPv4Address
		if ip == "" {
			ip = "(no IP)"
		}
		fmt.Fprintf(out, "%s  %s\n", nic.LinuxDeviceName, ip)
	}
}
