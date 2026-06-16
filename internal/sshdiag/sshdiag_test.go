package sshdiag

import (
	"errors"
	"strings"
	"testing"
)

// The exact x/crypto/ssh shape seen against a real device with the wrong user.
const authFailure = "bootstrap authorized_keys: dial hp@[fd3c::1]:22: ssh: handshake " +
	"failed: ssh: unable to authenticate, attempted methods [none password], " +
	"no supported methods remain"

func TestAnnotateAuthError_AddsHintWithEffectiveUser(t *testing.T) {
	base := errors.New(authFailure)
	got := AnnotateAuthError(base, "hp", "spark-53d0")
	if got == nil {
		t.Fatal("expected an error, got nil")
	}
	msg := got.Error()
	if !strings.Contains(msg, "--user") {
		t.Errorf("auth failure should hint at --user; got %q", msg)
	}
	if !strings.Contains(msg, "hp") || !strings.Contains(msg, "spark-53d0") {
		t.Errorf("hint should name the effective user and host; got %q", msg)
	}
	if !errors.Is(got, base) {
		t.Errorf("annotated error should wrap the original (errors.Is)")
	}
}

func TestAnnotateAuthError_NamesTheActualUserNotJustDefault(t *testing.T) {
	// A wrong password on a non-default user must not talk about `hp`.
	base := errors.New(strings.Replace(authFailure, "hp@", "ansol@", 1))
	got := AnnotateAuthError(base, "ansol", "spark-53d0")
	msg := got.Error()
	if !strings.Contains(msg, "ansol") {
		t.Errorf("hint must name the effective user 'ansol'; got %q", msg)
	}
}

func TestAnnotateAuthError_DoesNotClaimUsernameIsWrong(t *testing.T) {
	// Sharpest failure mode: wrong password looks identical. Never assert the user is wrong.
	base := errors.New(authFailure)
	msg := AnnotateAuthError(base, "hp", "spark-53d0").Error()
	lower := strings.ToLower(msg)
	for _, banned := range []string{"username is wrong", "wrong username", "incorrect username"} {
		if strings.Contains(lower, banned) {
			t.Errorf("must not assert the username is wrong (could be password); got %q", msg)
		}
	}
}

func TestAnnotateAuthError_PassesThroughHostKeyMismatch(t *testing.T) {
	base := errors.New(`bootstrap authorized_keys: dial hp@[fd3c::1]:22: ssh: handshake ` +
		`failed: unknown SSH host "[fd3c::1]:22" rejected`)
	got := AnnotateAuthError(base, "hp", "spark-53d0")
	if got.Error() != base.Error() {
		t.Errorf("host-key mismatch must pass through unchanged; got %q", got.Error())
	}
}

func TestAnnotateAuthError_PassesThroughKeyParseError(t *testing.T) {
	base := errors.New("generate or reuse key: read private key: ssh: no key found")
	got := AnnotateAuthError(base, "hp", "spark-53d0")
	if got.Error() != base.Error() {
		t.Errorf("key-parse error must pass through unchanged; got %q", got.Error())
	}
}

func TestAnnotateAuthError_NilPassesThrough(t *testing.T) {
	if got := AnnotateAuthError(nil, "hp", "spark-53d0"); got != nil {
		t.Errorf("nil error must stay nil; got %v", got)
	}
}
