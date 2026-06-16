package cmd

import (
	"testing"

	"github.com/AndersSol/zgx-cli/internal/discovery"
)

func TestFormatDevice(t *testing.T) {
	tests := []struct {
		name   string
		device discovery.Device
		want   string
	}{
		{
			name: "IPv4 and IPv6 addresses",
			device: discovery.Device{
				Hostname:  "zgx-53d0",
				Addresses: []string{"192.168.1.209", "fd3c::1"},
				Port:      22,
				Name:      "zgx-53d0 SSH",
			},
			want: "zgx-53d0  192.168.1.209:22, [fd3c::1]:22  (zgx-53d0 SSH)",
		},
		{
			name: "no addresses",
			device: discovery.Device{
				Hostname:  "zgx-53d0",
				Addresses: nil,
				Port:      22,
				Name:      "zgx-53d0 SSH",
			},
			want: "zgx-53d0  :22  (zgx-53d0 SSH)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDevice(tt.device)
			if got != tt.want {
				t.Fatalf("formatDevice() = %q, want %q", got, tt.want)
			}
		})
	}
}
