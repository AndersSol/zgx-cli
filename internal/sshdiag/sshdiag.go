// Package sshdiag turns opaque SSH failures into actionable operator hints.
package sshdiag

import (
	"fmt"
	"strings"
)

// isAuthMethodsExhausted reports whether err is the x/crypto/ssh failure raised
// when every offered authentication method was rejected, e.g.
//
//	ssh: unable to authenticate, attempted methods [none password], no supported methods remain
//
// That single shape covers both a wrong username and a wrong password, so callers
// must not claim which one is at fault. Host-key mismatches and key-parse errors
// carry different text and deliberately do not match.
func isAuthMethodsExhausted(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "unable to authenticate") &&
		strings.Contains(msg, "no supported methods remain")
}

// AnnotateAuthError wraps an exhausted-auth SSH failure with a hint that names the
// effective user and host and points at --user, without asserting whether the
// username or the password is wrong. Any other error (including nil) is returned
// unchanged so callers can wrap every SSH result unconditionally.
func AnnotateAuthError(err error, user, host string) error {
	if !isAuthMethodsExhausted(err) {
		return err
	}
	return fmt.Errorf("SSH authentication failed for %s@%s: the username or password was "+
		"not accepted. Devices imaged with NVIDIA DGX OS use the user you created at first "+
		"boot (retry with --user <name>); HP factory images use --user hp: %w", user, host, err)
}
