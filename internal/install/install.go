// Package install orchestrates installation, verification, and uninstallation
// over a testable remote command runner.
package install

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/AndersSol/zgx/internal/catalog"
)

const (
	defaultVerifyTimeout = 7 * time.Second
	defaultRetries       = 3
)

var sudoPattern = regexp.MustCompile(`sudo\s+`)

// CommandResult is the outcome of one remote command.
type CommandResult struct {
	ExitCode       int
	Stdout, Stderr string
}

// Runner runs a remote shell command. sudoPassword is written to stdin if the
// command starts with "sudo -S". Abstraction for testability (real = SSHRunner).
type Runner interface {
	Run(ctx context.Context, command, sudoPassword string, timeout time.Duration, retries int) (CommandResult, error)
}

// SudoCommand transforms an installCommand using the source semantics.
// It returns (command to run, usesSudo). With sudo: "sudo -S bash -c '<escaped without sudo>'".
func SudoCommand(rawCommand string) (command string, usesSudo bool) {
	usesSudo = strings.Contains(rawCommand, "sudo")
	if !usesSudo {
		return rawCommand, false
	}

	commandWithoutSudo := sudoPattern.ReplaceAllString(rawCommand, "")
	return "sudo -S bash -c " + singleQuote(commandWithoutSudo), true
}

// Report describes the aggregate outcome for an installation or uninstallation run.
type Report struct {
	Installed, AlreadyInstalled, Failed, Skipped []string
}

// Engine orchestrates install/verify/uninstall over a Runner.
type Engine struct {
	Runner Runner
}

func (e *Engine) Install(ctx context.Context, cats []catalog.Category, selectedIDs []string, sudoPassword string) (Report, error) {
	if err := e.ready(); err != nil {
		return Report{}, err
	}

	ordered, err := InstallPlan(cats, selectedIDs)
	if err != nil {
		return Report{}, err
	}

	report := Report{}
	for _, app := range ordered {
		installed, err := e.verifyApp(ctx, app)
		if err == nil && installed {
			report.AlreadyInstalled = append(report.AlreadyInstalled, app.ID)
			continue
		}

		if e.installApp(ctx, app, sudoPassword) {
			report.Installed = append(report.Installed, app.ID)
		} else {
			report.Failed = append(report.Failed, app.ID)
		}
	}

	return report, nil
}

// InstallPlan returns the apps Install will process, in order: base-system first
// (if present), then expanded dependencies + selected apps, in InstallOrder. Used
// by the cmd layer to show the full install plan.
func InstallPlan(cats []catalog.Category, selectedIDs []string) ([]catalog.App, error) {
	selected, err := resolveSelected(cats, selectedIDs)
	if err != nil {
		return nil, err
	}

	apps, err := expandDependencies(cats, selected)
	if err != nil {
		return nil, err
	}

	ordered, err := catalog.InstallOrder(apps)
	if err != nil {
		return nil, err
	}

	if base, ok := catalog.ByID(cats, "base-system"); ok {
		ordered = withoutAppID(ordered, base.ID)
		ordered = append([]catalog.App{base}, ordered...)
	}

	return ordered, nil
}

// Verify returns appID -> installed for the explicitly selected apps.
func (e *Engine) Verify(ctx context.Context, cats []catalog.Category, selectedIDs []string) (map[string]bool, error) {
	if err := e.ready(); err != nil {
		return nil, err
	}

	apps, err := resolveSelected(cats, selectedIDs)
	if err != nil {
		return nil, err
	}

	result := make(map[string]bool, len(apps))
	for _, app := range apps {
		installed, err := e.verifyApp(ctx, app)
		result[app.ID] = err == nil && installed
	}

	return result, nil
}

func (e *Engine) Uninstall(ctx context.Context, cats []catalog.Category, selectedIDs []string, sudoPassword string) (Report, error) {
	if err := e.ready(); err != nil {
		return Report{}, err
	}

	selected, err := resolveSelected(cats, selectedIDs)
	if err != nil {
		return Report{}, err
	}

	ordered, err := catalog.InstallOrder(selected)
	if err != nil {
		return Report{}, err
	}
	// Uninstall intentionally does not expand dependencies: shared deps may be in
	// use by other apps. Reverse InstallOrder applies only within the selected set.
	slices.Reverse(ordered)

	report := Report{}
	for _, app := range ordered {
		if app.UninstallCommand == nil {
			report.Skipped = append(report.Skipped, app.ID)
			continue
		}

		command, usesSudo := SudoCommand(*app.UninstallCommand)
		result, err := e.Runner.Run(ctx, command, passwordForSudo(sudoPassword, usesSudo), 0, defaultRetries)
		if err != nil || result.ExitCode != 0 {
			report.Failed = append(report.Failed, app.ID)
			continue
		}
		report.Installed = append(report.Installed, app.ID)
	}

	return report, nil
}

func (e *Engine) ready() error {
	if e == nil || e.Runner == nil {
		return fmt.Errorf("install: Runner missing")
	}
	return nil
}

func (e *Engine) installApp(ctx context.Context, app catalog.App, sudoPassword string) bool {
	command, usesSudo := SudoCommand(app.InstallCommand)
	result, err := e.Runner.Run(ctx, command, passwordForSudo(sudoPassword, usesSudo), 0, defaultRetries)
	if err != nil || result.ExitCode != 0 {
		return false
	}

	installed, err := e.verifyApp(ctx, app)
	return err == nil && installed
}

func (e *Engine) verifyApp(ctx context.Context, app catalog.App) (bool, error) {
	result, err := e.Runner.Run(ctx, app.VerifyCommand, "", defaultVerifyTimeout, defaultRetries)
	return err == nil && result.ExitCode == 0, err
}

func resolveSelected(cats []catalog.Category, selectedIDs []string) ([]catalog.App, error) {
	apps := make([]catalog.App, 0, len(selectedIDs))
	seen := make(map[string]struct{}, len(selectedIDs))

	for _, id := range selectedIDs {
		if _, ok := seen[id]; ok {
			continue
		}
		app, ok := catalog.ByID(cats, id)
		if !ok {
			return nil, fmt.Errorf("install: unknown app id %q", id)
		}
		apps = append(apps, app)
		seen[id] = struct{}{}
	}

	return apps, nil
}

func expandDependencies(cats []catalog.Category, selected []catalog.App) ([]catalog.App, error) {
	expanded := make([]catalog.App, 0, len(selected))
	seen := make(map[string]struct{}, len(selected))

	var add func(catalog.App) error
	add = func(app catalog.App) error {
		if _, ok := seen[app.ID]; ok {
			return nil
		}
		seen[app.ID] = struct{}{}
		for _, depID := range app.Dependencies {
			dep, ok := catalog.ByID(cats, depID)
			if !ok {
				return fmt.Errorf("install: %s has unknown dependency %q", app.ID, depID)
			}
			if err := add(dep); err != nil {
				return err
			}
		}
		expanded = append(expanded, app)
		return nil
	}

	for _, app := range selected {
		if err := add(app); err != nil {
			return nil, err
		}
	}

	return expanded, nil
}

func passwordForSudo(sudoPassword string, usesSudo bool) string {
	if usesSudo {
		return sudoPassword
	}
	return ""
}

func containsAppID(apps []catalog.App, id string) bool {
	for _, app := range apps {
		if app.ID == id {
			return true
		}
	}
	return false
}

func withoutAppID(apps []catalog.App, id string) []catalog.App {
	filtered := apps[:0]
	for _, app := range apps {
		if app.ID != id {
			filtered = append(filtered, app)
		}
	}
	return filtered
}

func singleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
