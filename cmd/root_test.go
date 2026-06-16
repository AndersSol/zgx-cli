package cmd

import (
	"io"
	"strings"
	"testing"
)

// cobra registers these itself; they are not part of zgx's command surface.
var cobraBuiltins = map[string]bool{"help": true, "completion": true}

// TestRootSubcommandSetExact verifies that the registered command surface is
// exactly the intended one: every expected command exists, and no unexpected
// commands are registered (except cobra's built-ins). The set is derived from
// rootCmd.Commands(), not a self-reported literal, so the test catches both
// missing and accidentally added commands (drift in both directions).
func TestRootSubcommandSetExact(t *testing.T) {
	want := map[string]bool{
		"config":   true,
		"discover": true, "connect": true,
		"list": true, "install": true, "verify": true, "uninstall": true,
		"health": true, "dns-register": true,
		"pair": true, "unpair": true, "pair-details": true,
	}

	have := make(map[string]bool)
	for _, c := range rootCmd.Commands() {
		name := c.Name()
		if cobraBuiltins[name] {
			continue
		}
		have[name] = true
		if !want[name] {
			t.Errorf("unexpected subcommand registered: %q", name)
		}
	}

	for name := range want {
		if !have[name] {
			t.Errorf("root missing subcommand %q", name)
		}
	}
}

// TestStubsReturnError still defends the honest-exit invariant from the stub
// phase: now via a real command error path that MUST return an error (exit != 0).
func TestStubsReturnError(t *testing.T) {
	rootCmd.SetArgs([]string{"pair-details"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("command error path returned nil; should return an error for exit != 0")
	}
	if !strings.Contains(err.Error(), "accepts 1 arg") {
		t.Errorf("unexpected error message from missing arg: %v", err)
	}
}
