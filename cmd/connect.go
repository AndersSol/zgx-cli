package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	connectpkg "github.com/AndersSol/zgx-cli/internal/connect"
	"github.com/AndersSol/zgx-cli/internal/discovery"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func init() {
	rootCmd.AddCommand(connectCmd())
}

func connectCmd() *cobra.Command {
	var (
		user       string
		port       int
		alias      string
		knownHosts string
		password   string
		discoverTimeout int
	)

	cmd := &cobra.Command{
		Use:   "connect <host>",
		Short: "Set up SSH key access to a device",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var host string
			if len(args) == 1 {
				host = args[0]
			} else {
				devices, err := discovery.DiscoverTimeout(time.Duration(discoverTimeout) * time.Second)
				if err != nil {
					return err
				}
				device, err := selectDevice(devices, os.Stdin, cmd.OutOrStdout())
				if err != nil {
					if errors.Is(err, errNoZGXDevicesFound) {
						return errors.New("no ZGX devices found; run 'zgx discover' or pass a host")
					}
					return err
				}
				host = connectHost(device)
				if alias == "" {
					alias = device.Hostname
				}
			}
			if alias == "" {
				alias = host
			}

			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("find home directory: %w", err)
			}
			sshDir := filepath.Join(homeDir, ".ssh")
			if knownHosts == "" {
				knownHosts = filepath.Join(sshDir, "known_hosts")
			}

			out := cmd.OutOrStdout()
			if password == "" {
				fmt.Fprintf(out, "Password for %s@%s: ", user, host)
				passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
				fmt.Fprintln(out)
				if err != nil {
					return fmt.Errorf("read password: %w", err)
				}
				password = string(passwordBytes)
			}

			fmt.Fprintln(out, "Generating or reusing SSH key ...")
			keyPair, err := connectpkg.GenerateKeyPair(sshDir, "id_ed25519", fmt.Sprintf("%s@%s", user, host))
			if err != nil {
				return err
			}

			hostKey, err := connectpkg.KnownHostsCallbackWithConfirm(knownHosts, func(hostname, fingerprint string) (bool, error) {
				fmt.Fprintf(out, "Unknown host %s. ED25519 fingerprint: %s. Do you trust it? Type yes: ", hostname, fingerprint)
				answer, err := bufio.NewReader(os.Stdin).ReadString('\n')
				if err != nil {
					return false, err
				}
				return strings.EqualFold(strings.TrimSpace(answer), "yes"), nil
			})
			if err != nil {
				return err
			}

			target := connectpkg.Target{Host: host, User: user, Port: port}
			fmt.Fprintf(out, "Adding public key to %s@%s ...\n", user, host)
			if err := connectpkg.Bootstrap(context.Background(), target, password, keyPair.PublicKeyLine, hostKey); err != nil {
				return err
			}

			configPath := filepath.Join(sshDir, "config")
			fmt.Fprintf(out, "Writing SSH config for alias %q ...\n", alias)
			if err := connectpkg.WriteHostConfig(configPath, alias, host, user, port, keyPair.PrivateKeyPath); err != nil {
				return err
			}

			fmt.Fprintln(out, "Testing key-based access ...")
			if err := connectpkg.TestKeyAuth(context.Background(), target, keyPair.PrivateKeyPath, hostKey); err != nil {
				return err
			}
			fmt.Fprintln(out, "Key-based access works.")
			return nil
		},
	}

	cmd.Flags().StringVar(&user, "user", "hp", "SSH user on the device")
	cmd.Flags().IntVar(&port, "port", 22, "SSH port on the device")
	cmd.Flags().StringVar(&alias, "alias", "", "Host alias in ~/.ssh/config (default: host)")
	cmd.Flags().StringVar(&knownHosts, "known-hosts", "", "known_hosts file (default: ~/.ssh/known_hosts)")
	cmd.Flags().StringVar(&password, "password", "", "SSH password (prompted hidden if empty)")
	cmd.Flags().IntVar(&discoverTimeout, "discover-timeout", 6, "Seconds to browse for devices when no host is given")

	return cmd
}

var errNoZGXDevicesFound = errors.New("no ZGX devices found")

func connectHost(d discovery.Device) string {
	if len(d.Addresses) > 0 {
		return d.Addresses[0]
	}
	return d.Hostname
}

func selectDevice(devices []discovery.Device, in io.Reader, out io.Writer) (discovery.Device, error) {
	switch len(devices) {
	case 0:
		return discovery.Device{}, errNoZGXDevicesFound
	case 1:
		fmt.Fprintf(out, "Using the only device found: %s (%s)\n", devices[0].Hostname, connectHost(devices[0]))
		return devices[0], nil
	}

	for i, device := range devices {
		fmt.Fprintf(out, "  [%d] %s  %s  (%s)\n", i+1, device.Hostname, connectHost(device), device.Name)
	}
	fmt.Fprintf(out, "Select a device [1-%d]: ", len(devices))

	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && len(line) == 0 {
		return discovery.Device{}, err
	}

	choice, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		return discovery.Device{}, fmt.Errorf("invalid device selection %q", strings.TrimSpace(line))
	}
	if choice < 1 || choice > len(devices) {
		return discovery.Device{}, fmt.Errorf("device selection %d out of range", choice)
	}
	return devices[choice-1], nil
}
