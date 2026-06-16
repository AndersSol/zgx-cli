package discovery

import (
	"net"
	"reflect"
	"strings"
	"testing"
)

func TestServiceTypesAreFullyQualified(t *testing.T) {
	services := []string{sshService, hpzgxService}
	for _, service := range services {
		if !strings.HasPrefix(service, "_") {
			t.Errorf("service %q does not start with _", service)
		}
		if !strings.HasSuffix(service, "._tcp.local.") {
			t.Errorf("service %q does not end with ._tcp.local.", service)
		}
	}
}

func TestAddressStringsIncludesRoutableIPv6(t *testing.T) {
	ips := []net.IP{
		net.ParseIP("192.168.1.209"),
		net.ParseIP("fd00::1"),
		net.ParseIP("fe80::1"),
	}

	got := addressStrings(ips)
	want := []string{"192.168.1.209", "fd00::1"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("addressStrings() = %#v, want %#v", got, want)
	}
}

func TestIsZGXHostname(t *testing.T) {
	positives := []string{
		"zgx-abc123",
		"zgx-ABCDEF",
		"zgx-ABCD",
		"spark-wxyz",
		"spark-1234",
	}
	for _, hostname := range positives {
		if !IsZGXHostname(hostname) {
			t.Errorf("IsZGXHostname(%q) = false, want true", hostname)
		}
	}

	negatives := []string{
		"zgx-abc",
		"zgx-abcde",
		"zgx-toolong7",
		"spark-toolong5",
		"spark-abc",
		"foobar",
		"",
		"zgx-",
		"ZGX-abcdef",
	}
	for _, hostname := range negatives {
		if IsZGXHostname(hostname) {
			t.Errorf("IsZGXHostname(%q) = true, want false", hostname)
		}
	}
}

func TestHostnameFromHost(t *testing.T) {
	tests := map[string]string{
		"zgx-abc123.local.": "zgx-abc123",
		"zgx-abc123.local":  "zgx-abc123",
		"foo":               "foo",
	}

	for input, want := range tests {
		if got := HostnameFromHost(input); got != want {
			t.Errorf("HostnameFromHost(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestMergeZGX(t *testing.T) {
	ssh := []Device{
		{
			Name:      "ssh-zgx",
			Hostname:  "zgx-abc123",
			Addresses: []string{"10.0.0.10"},
			Port:      22,
			Protocol:  "tcp",
		},
		{
			Name:      "ssh-laptop",
			Hostname:  "laptop",
			Addresses: []string{"10.0.0.11"},
			Port:      22,
			Protocol:  "tcp",
		},
	}
	hpzgx := []Device{
		{
			Name:       "hpzgx-zgx",
			Hostname:   "zgx-abc123",
			Addresses:  []string{"1.2.3.4"},
			Port:       22,
			Protocol:   "tcp",
			TXTRecords: map[string]string{"source": "hpzgx"},
		},
		{
			Name:      "hpzgx-server",
			Hostname:  "server9",
			Addresses: []string{"10.0.0.12"},
			Port:      8022,
			Protocol:  "tcp",
		},
	}

	got := MergeZGX(ssh, hpzgx)
	want := []Device{
		{
			Name:      "hpzgx-server",
			Hostname:  "server9",
			Addresses: []string{"10.0.0.12"},
			Port:      8022,
			Protocol:  "tcp",
		},
		{
			Name:       "hpzgx-zgx",
			Hostname:   "zgx-abc123",
			Addresses:  []string{"1.2.3.4"},
			Port:       22,
			Protocol:   "tcp",
			TXTRecords: map[string]string{"source": "hpzgx"},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MergeZGX() = %#v, want %#v", got, want)
	}
}
