package dnsreg

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/AndersSol/zgx-cli/internal/install"
)

func TestServiceFileXML(t *testing.T) {
	got := ServiceFileXML("abcd1234")
	want := `<service-group>
  <name>abcd1234</name>
  <service>
    <type>_hpzgx._tcp</type>
    <port>22</port>
  </service>
</service-group>`
	if got != want {
		t.Fatalf("ServiceFileXML() = %q, want %q", got, want)
	}

	got = ServiceFileXML(`a<b&c>"'`)
	if !strings.Contains(got, "<name>a&lt;b&amp;c&gt;&quot;&apos;</name>") {
		t.Fatalf("ServiceFileXML() does not escape XML characters: %q", got)
	}
}

func TestCreateServiceFileCommand(t *testing.T) {
	xml := "<service-group><name>it&apos;s</name></service-group>"
	got := CreateServiceFileCommand(xml)

	if !strings.HasPrefix(got, "sudo -S bash -c ") {
		t.Fatalf("CreateServiceFileCommand() = %q, want sudo prefix", got)
	}
	if !strings.Contains(got, "tee /etc/avahi/services/hpzgx.service") {
		t.Fatalf("CreateServiceFileCommand() missing service file path: %q", got)
	}
	if !strings.Contains(got, "'\\''") {
		t.Fatalf("CreateServiceFileCommand() does not single-quote-escape apostrophe: %q", got)
	}
}

func TestDeviceIdentifierCommand(t *testing.T) {
	want := "ip route show default | awk '/default/ { print $5 }' | head -1 | xargs -I {} cat /sys/class/net/{}/address | tr -d ':' | sha256sum | cut -c1-8"
	if got := DeviceIdentifierCommand(); got != want {
		t.Fatalf("DeviceIdentifierCommand() = %q, want %q", got, want)
	}
}

func TestRegisterFlow(t *testing.T) {
	runner := &fakeRunner{
		results: map[string]install.CommandResult{
			DeviceIdentifierCommand(): {ExitCode: 0, Stdout: "abcd1234\n"},
			RestartAvahiCommand():     {ExitCode: 0},
		},
	}

	result, err := Register(context.Background(), runner, "pw")
	if err != nil {
		t.Fatalf("Register() returned error: %v", err)
	}
	if result.Identifier != "abcd1234" || !result.ServiceFileWritten || !result.AvahiRestarted {
		t.Fatalf("Register() result = %#v", result)
	}

	createCommand := runner.commandContaining("tee /etc/avahi/services/hpzgx.service")
	if createCommand == "" {
		t.Fatalf("Register() did not run create command; commands=%v", runner.commands)
	}
	if !strings.Contains(createCommand, "<name>abcd1234</name>") {
		t.Fatalf("create command missing XML with identifier: %q", createCommand)
	}
}

func TestRegisterRestartFailureNonFatal(t *testing.T) {
	runner := &fakeRunner{
		results: map[string]install.CommandResult{
			DeviceIdentifierCommand(): {ExitCode: 0, Stdout: "abcd1234\n"},
			RestartAvahiCommand():     {ExitCode: 1, Stderr: "nope"},
		},
	}

	result, err := Register(context.Background(), runner, "pw")
	if err != nil {
		t.Fatalf("Register() returned error on restart failure, want non-fatal: %v", err)
	}
	if result.AvahiRestarted {
		t.Fatalf("Register() AvahiRestarted = true on restart failure")
	}
	if result.Note == "" {
		t.Fatalf("Register() Note is empty on restart failure: %#v", result)
	}
}

func TestRegisterEmptyIdIsLoudError(t *testing.T) {
	runner := &fakeRunner{
		results: map[string]install.CommandResult{
			DeviceIdentifierCommand(): {ExitCode: 0, Stdout: " \n"},
		},
	}

	_, err := Register(context.Background(), runner, "pw")
	if err == nil {
		t.Fatal("Register() returned nil on empty device id")
	}
	if len(runner.commands) != 1 {
		t.Fatalf("Register() ran create/restart after empty id: %v", runner.commands)
	}
}

func TestRegisterRejectsMalformedIdentifier(t *testing.T) {
	for _, stdout := range []string{"evil\n<inject>", "GGGG"} {
		t.Run(stdout, func(t *testing.T) {
			runner := &fakeRunner{
				results: map[string]install.CommandResult{
					DeviceIdentifierCommand(): {ExitCode: 0, Stdout: stdout},
				},
			}

			_, err := Register(context.Background(), runner, "pw")
			if err == nil {
				t.Fatal("Register() returned nil on malformed device id")
			}
			if runner.commandContaining("tee /etc/avahi/services/hpzgx.service") != "" {
				t.Fatalf("Register() ran service file command after malformed id: %v", runner.commands)
			}
			if runner.commandContaining("systemctl restart avahi-daemon") != "" {
				t.Fatalf("Register() ran restart after malformed id: %v", runner.commands)
			}
		})
	}
}

type fakeRunner struct {
	results  map[string]install.CommandResult
	errors   map[string]error
	commands []string
}

func (r *fakeRunner) Run(_ context.Context, command, _ string, _ time.Duration, _ int) (install.CommandResult, error) {
	r.commands = append(r.commands, command)
	if err := r.errors[command]; err != nil {
		return install.CommandResult{}, err
	}
	if result, ok := r.results[command]; ok {
		return result, nil
	}
	if strings.Contains(command, "tee /etc/avahi/services/hpzgx.service") {
		return install.CommandResult{ExitCode: 0}, nil
	}
	return install.CommandResult{}, errors.New("unexpected command: " + command)
}

func (r *fakeRunner) commandContaining(needle string) string {
	for _, command := range r.commands {
		if strings.Contains(command, needle) {
			return command
		}
	}
	return ""
}
