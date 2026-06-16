package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/AndersSol/zgx-cli/internal/discovery"
)

func TestConnectHost(t *testing.T) {
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
			},
			want: "192.168.1.209",
		},
		{
			name: "no addresses",
			device: discovery.Device{
				Hostname: "zgx-53d0",
			},
			want: "zgx-53d0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := connectHost(tt.device)
			if got != tt.want {
				t.Fatalf("connectHost() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSelectDevice(t *testing.T) {
	devices := []discovery.Device{
		{
			Name:      "zgx-1111 SSH",
			Hostname:  "zgx-1111",
			Addresses: []string{"192.168.1.11"},
		},
		{
			Name:      "zgx-2222 SSH",
			Hostname:  "zgx-2222",
			Addresses: []string{"192.168.1.22"},
		},
	}

	t.Run("empty returns error", func(t *testing.T) {
		var out bytes.Buffer
		_, err := selectDevice(nil, strings.NewReader(""), &out)
		if err == nil {
			t.Fatal("selectDevice() error = nil, want error")
		}
	})

	t.Run("single returns without reading input", func(t *testing.T) {
		var out bytes.Buffer
		got, err := selectDevice(devices[:1], strings.NewReader(""), &out)
		if err != nil {
			t.Fatalf("selectDevice() error = %v, want nil", err)
		}
		if got.Hostname != devices[0].Hostname {
			t.Fatalf("selectDevice() hostname = %q, want %q", got.Hostname, devices[0].Hostname)
		}
		wantOut := "Using the only device found: zgx-1111 (192.168.1.11)\n"
		if out.String() != wantOut {
			t.Fatalf("selectDevice() output = %q, want %q", out.String(), wantOut)
		}
	})

	t.Run("multi selects second device", func(t *testing.T) {
		var out bytes.Buffer
		got, err := selectDevice(devices, strings.NewReader("2\n"), &out)
		if err != nil {
			t.Fatalf("selectDevice() error = %v, want nil", err)
		}
		if got.Hostname != devices[1].Hostname {
			t.Fatalf("selectDevice() hostname = %q, want %q", got.Hostname, devices[1].Hostname)
		}
		wantOut := "  [1] zgx-1111  192.168.1.11  (zgx-1111 SSH)\n" +
			"  [2] zgx-2222  192.168.1.22  (zgx-2222 SSH)\n" +
			"Select a device [1-2]: "
		if out.String() != wantOut {
			t.Fatalf("selectDevice() output = %q, want %q", out.String(), wantOut)
		}
	})

	t.Run("multi out of range returns error", func(t *testing.T) {
		var out bytes.Buffer
		_, err := selectDevice(devices, strings.NewReader("9\n"), &out)
		if err == nil {
			t.Fatal("selectDevice() error = nil, want error")
		}
	})

	t.Run("multi non numeric returns error", func(t *testing.T) {
		var out bytes.Buffer
		_, err := selectDevice(devices, strings.NewReader("abc\n"), &out)
		if err == nil {
			t.Fatal("selectDevice() error = nil, want error")
		}
	})
}
