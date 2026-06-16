package cmd

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/AndersSol/zgx-cli/internal/discovery"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(discoverCmd())
}

func discoverCmd() *cobra.Command {
	var timeoutSeconds int

	cmd := &cobra.Command{
		Use:   "discover",
		Short: "Find ZGX devices on the network (mDNS)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			devices, err := discovery.DiscoverTimeout(time.Duration(timeoutSeconds) * time.Second)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if len(devices) == 0 {
				fmt.Fprintln(out, "No ZGX devices found.")
				return nil
			}

			for _, device := range devices {
				fmt.Fprintln(out, formatDevice(device))
			}

			return nil
		},
	}
	cmd.Flags().IntVar(&timeoutSeconds, "timeout", 5, "How long discovery should run, in seconds")
	return cmd
}

func formatDevice(d discovery.Device) string {
	port := strconv.Itoa(d.Port)
	hostports := make([]string, 0, len(d.Addresses))
	for _, addr := range d.Addresses {
		hostports = append(hostports, net.JoinHostPort(addr, port))
	}
	addrField := strings.Join(hostports, ", ")
	if addrField == "" {
		addrField = ":" + port
	}
	return fmt.Sprintf("%s  %s  (%s)", d.Hostname, addrField, d.Name)
}
