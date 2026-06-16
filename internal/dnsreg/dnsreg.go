// Package dnsreg registers ZGX devices in Avahi for stable mDNS discovery.
package dnsreg

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/AndersSol/zgx/internal/install"
)

const (
	ServiceType     = "_hpzgx._tcp"
	ServicePort     = 22
	ServiceFilePath = "/etc/avahi/services/hpzgx.service"

	commandTimeout = 30 * time.Second
)

var deviceIdentifierPattern = regexp.MustCompile(`^[0-9a-f]{8}$`)

// DeviceIdentifierCommand returns the fixed remote command that gets the default
// NIC's MAC and hashes it to an 8-character ID.
func DeviceIdentifierCommand() string {
	return "ip route show default | awk '/default/ { print $5 }' | head -1 | xargs -I {} cat /sys/class/net/{}/address | tr -d ':' | sha256sum | cut -c1-8"
}

// ServiceFileXML builds Avahi service-group XML for identifier (XML-escaped).
func ServiceFileXML(identifier string) string {
	return fmt.Sprintf(`<service-group>
  <name>%s</name>
  <service>
    <type>%s</type>
    <port>%d</port>
  </service>
</service-group>`, escapeXML(identifier), ServiceType, ServicePort)
}

// CreateServiceFileCommand builds the sudo command that writes the Avahi service file.
func CreateServiceFileCommand(xml string) string {
	inner := "echo " + singleQuote(xml) + " | tee " + ServiceFilePath + " > /dev/null"
	return "sudo -S bash -c " + singleQuote(inner)
}

// RestartAvahiCommand returns the fixed restart command.
func RestartAvahiCommand() string {
	return "sudo -S systemctl restart avahi-daemon"
}

type Result struct {
	Identifier         string
	ServiceFileWritten bool
	AvahiRestarted     bool
	Note               string
}

// Register runs the full dns-register flow over runner.
func Register(ctx context.Context, runner install.Runner, sudoPassword string) (Result, error) {
	if runner == nil {
		return Result{}, fmt.Errorf("dns-register: Runner missing")
	}

	idResult, err := runner.Run(ctx, DeviceIdentifierCommand(), "", commandTimeout, 0)
	if err != nil {
		return Result{}, fmt.Errorf("dns-register: get device id: %w", err)
	}
	if idResult.ExitCode != 0 {
		return Result{}, fmt.Errorf("dns-register: get device id failed with exit %d: %s", idResult.ExitCode, strings.TrimSpace(idResult.Stderr))
	}
	identifier := strings.TrimSpace(idResult.Stdout)
	if identifier == "" {
		return Result{}, fmt.Errorf("dns-register: empty device id from command %q", DeviceIdentifierCommand())
	}
	if !deviceIdentifierPattern.MatchString(identifier) {
		return Result{}, fmt.Errorf("dns-register: unexpected device id format: %q", identifier)
	}

	result := Result{Identifier: identifier}
	xml := ServiceFileXML(identifier)
	createResult, err := runner.Run(ctx, CreateServiceFileCommand(xml), sudoPassword, commandTimeout, 0)
	if err != nil {
		return Result{}, fmt.Errorf("dns-register: write service file: %w", err)
	}
	if createResult.ExitCode != 0 {
		return Result{}, fmt.Errorf("dns-register: write service file failed with exit %d: %s", createResult.ExitCode, strings.TrimSpace(createResult.Stderr))
	}
	result.ServiceFileWritten = true

	restartResult, err := runner.Run(ctx, RestartAvahiCommand(), sudoPassword, commandTimeout, 0)
	if err != nil || restartResult.ExitCode != 0 {
		result.AvahiRestarted = false
		result.Note = "Avahi could not be restarted; the service file is written and will be activated on the next reboot."
		return result, nil
	}

	result.AvahiRestarted = true
	return result, nil
}

func escapeXML(s string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return replacer.Replace(s)
}

func singleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
