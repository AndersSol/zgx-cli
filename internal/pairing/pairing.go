// Package pairing discovers and configures ConnectX NICs over a testable
// command runner.
package pairing

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/AndersSol/zgx-cli/internal/install"
)

const (
	NetplanPath = "/etc/netplan/40-zgx-connectx.yaml"
	LshwCommand = "lshw -class network -json"
)

const (
	lshwTimeout  = 15 * time.Second
	ipTimeout    = 10 * time.Second
	sudoTimeout  = 30 * time.Second
	noRetry      = 0
	sudoRetry    = 0
	mellanoxTerm = "mellanox"
)

var linuxDeviceNamePattern = regexp.MustCompile(`^enp[a-zA-Z0-9_-]+$`)

// NIC describes a ConnectX network interface.
type NIC struct {
	LinuxDeviceName string
	IPv4Address     string
}

// ParseConnectXNICs filters lshw network objects down to Mellanox/ConnectX NICs
// with one single enp logicalname. IPv4Address is not filled here.
func ParseConnectXNICs(lshwJSON []byte) ([]NIC, error) {
	var entries []struct {
		Product     string          `json:"product"`
		Vendor      string          `json:"vendor"`
		LogicalName json.RawMessage `json:"logicalname"`
	}
	if err := json.Unmarshal(lshwJSON, &entries); err != nil {
		return nil, fmt.Errorf("parse lshw network JSON: %w", err)
	}

	nics := make([]NIC, 0, len(entries))
	for _, entry := range entries {
		if !containsMellanox(entry.Product) && !containsMellanox(entry.Vendor) {
			continue
		}

		var logicalName string
		if err := json.Unmarshal(entry.LogicalName, &logicalName); err != nil {
			continue
		}
		if !strings.HasPrefix(logicalName, "enp") {
			continue
		}

		nics = append(nics, NIC{LinuxDeviceName: logicalName})
	}

	return nics, nil
}

// IPCommand builds the command that fetches the first IPv4 address for an interface.
func IPCommand(deviceName string) string {
	return fmt.Sprintf("ip a l %s | awk '/inet / {print $2}'", deviceName)
}

// ParseIPv4 returns the first IP line without the CIDR suffix.
func ParseIPv4(ipOutput string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(ipOutput), "\n")
	line = strings.TrimSpace(line)
	ip, _, _ := strings.Cut(line, "/")
	return ip
}

// BuildNetplan builds the netplan YAML the source writes for ConnectX interfaces.
func BuildNetplan(nics []NIC) (string, error) {
	lines := []string{
		"network:",
		"  version: 2",
		"  ethernets:",
	}
	for _, nic := range nics {
		if !linuxDeviceNamePattern.MatchString(nic.LinuxDeviceName) {
			return "", fmt.Errorf("invalid Linux device name %q", nic.LinuxDeviceName)
		}
		lines = append(lines,
			fmt.Sprintf("    %s:", nic.LinuxDeviceName),
			"      link-local: [ ipv4 ]",
		)
	}
	return strings.Join(lines, "\n"), nil
}

// WriteNetplanCommand builds the sudo command that writes and secures the netplan file.
func WriteNetplanCommand(config string) string {
	inner := fmt.Sprintf("echo '%s' > %s && chmod 600 %s", singleQuoteEscape(config), NetplanPath, NetplanPath)
	return "sudo -S sh -c " + singleQuote(inner)
}

func ApplyNetplanCommand() string {
	return "sudo -S netplan apply"
}

func RemoveNetplanCommand() string {
	return "sudo -S rm -f " + NetplanPath
}

// PairDetails fetches ConnectX NICs and their current IPv4 addresses.
func PairDetails(ctx context.Context, runner install.Runner) ([]NIC, error) {
	if runner == nil {
		return nil, fmt.Errorf("pairing: Runner missing")
	}

	result, err := runner.Run(ctx, LshwCommand, "", lshwTimeout, noRetry)
	if err != nil {
		return nil, fmt.Errorf("run lshw: %w", err)
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("lshw failed with exit %d: %s", result.ExitCode, strings.TrimSpace(result.Stderr))
	}

	nics, err := ParseConnectXNICs([]byte(result.Stdout))
	if err != nil {
		return nil, err
	}

	for i := range nics {
		if !linuxDeviceNamePattern.MatchString(nics[i].LinuxDeviceName) {
			return nil, fmt.Errorf("invalid Linux device name %q", nics[i].LinuxDeviceName)
		}
		result, err := runner.Run(ctx, IPCommand(nics[i].LinuxDeviceName), "", ipTimeout, noRetry)
		if err != nil {
			return nil, fmt.Errorf("get IP for %s: %w", nics[i].LinuxDeviceName, err)
		}
		if result.ExitCode != 0 {
			return nil, fmt.Errorf("get IP for %s failed with exit %d: %s", nics[i].LinuxDeviceName, result.ExitCode, strings.TrimSpace(result.Stderr))
		}
		nics[i].IPv4Address = ParseIPv4(result.Stdout)
	}

	return nics, nil
}

// Pair discovers ConnectX NICs, writes netplan, and applies the configuration.
func Pair(ctx context.Context, runner install.Runner, sudoPassword string) ([]NIC, error) {
	nics, err := PairDetails(ctx, runner)
	if err != nil {
		return nil, err
	}
	if len(nics) == 0 {
		return nil, fmt.Errorf("no ConnectX NICs found")
	}

	config, err := BuildNetplan(nics)
	if err != nil {
		return nil, err
	}

	if err := runSudo(ctx, runner, WriteNetplanCommand(config), sudoPassword, "write netplan"); err != nil {
		return nil, err
	}
	if err := runSudo(ctx, runner, ApplyNetplanCommand(), sudoPassword, "apply netplan"); err != nil {
		return nil, err
	}

	return nics, nil
}

// Unpair removes the ConnectX netplan file and applies netplan.
func Unpair(ctx context.Context, runner install.Runner, sudoPassword string) error {
	if runner == nil {
		return fmt.Errorf("pairing: Runner missing")
	}
	if err := runSudo(ctx, runner, RemoveNetplanCommand(), sudoPassword, "remove netplan"); err != nil {
		return err
	}
	return runSudo(ctx, runner, ApplyNetplanCommand(), sudoPassword, "apply netplan")
}

func runSudo(ctx context.Context, runner install.Runner, command, sudoPassword, label string) error {
	result, err := runner.Run(ctx, command, sudoPassword, sudoTimeout, sudoRetry)
	if err != nil {
		return fmt.Errorf("%s: %w", label, err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("%s failed with exit %d: %s", label, result.ExitCode, strings.TrimSpace(result.Stderr))
	}
	return nil
}

func containsMellanox(value string) bool {
	return strings.Contains(strings.ToLower(value), mellanoxTerm)
}

func singleQuoteEscape(value string) string {
	return strings.ReplaceAll(value, "'", "'\\''")
}

func singleQuote(value string) string {
	return "'" + singleQuoteEscape(value) + "'"
}
