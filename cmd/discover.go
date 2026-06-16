package cmd

import (
	"fmt"
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
				fmt.Fprintf(out, "%s  %s:%d  (%s)\n",
					device.Hostname,
					strings.Join(device.Addresses, ","),
					device.Port,
					device.Name,
				)
			}

			return nil
		},
	}
	cmd.Flags().IntVar(&timeoutSeconds, "timeout", 5, "How long discovery should run, in seconds")
	return cmd
}
