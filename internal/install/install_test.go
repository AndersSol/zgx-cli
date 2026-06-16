package install

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/AndersSol/zgx-cli/internal/catalog"
)

func TestSudoCommand(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		command  string
		usesSudo bool
	}{
		{
			name:     "simple sudo",
			raw:      "sudo apt install -y x",
			command:  "sudo -S bash -c 'apt install -y x'",
			usesSudo: true,
		},
		{
			name:     "strips all sudo tokens",
			raw:      "sudo a && sudo b",
			command:  "sudo -S bash -c 'a && b'",
			usesSudo: true,
		},
		{
			name:     "no sudo",
			raw:      "curl -fsSL x | sh",
			command:  "curl -fsSL x | sh",
			usesSudo: false,
		},
		{
			name:     "escapes apostrophe",
			raw:      "sudo bash -c \"echo 'x'\"",
			command:  "sudo -S bash -c 'bash -c \"echo '\\''x'\\''\"'",
			usesSudo: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command, usesSudo := SudoCommand(tt.raw)
			if command != tt.command {
				t.Fatalf("SudoCommand(%q) command = %q, want %q", tt.raw, command, tt.command)
			}
			if usesSudo != tt.usesSudo {
				t.Fatalf("SudoCommand(%q) usesSudo = %v, want %v", tt.raw, usesSudo, tt.usesSudo)
			}
		})
	}
}

func TestInstallOrdersDepsAndBaseFirst(t *testing.T) {
	cats := mustCatalog(t)
	verify := map[string][]int{}
	for _, id := range []string{"base-system", "curl", "miniforge", "zgx-python-env", "jupyter-lab"} {
		verify[id] = []int{1, 0}
	}
	runner := newFakeRunner(cats, verify, nil)

	report, err := (&Engine{Runner: runner}).Install(context.Background(), cats, []string{"jupyter-lab"}, "pw")
	if err != nil {
		t.Fatalf("Install returned error: %v", err)
	}

	installs := runner.installIDs()
	wantOrder := []string{"base-system", "curl", "miniforge", "zgx-python-env", "jupyter-lab"}
	if !slices.Equal(installs, wantOrder) {
		t.Fatalf("install order = %v, want %v", installs, wantOrder)
	}
	for _, id := range wantOrder {
		if !slices.Contains(report.Installed, id) {
			t.Fatalf("Report.Installed missing %q: %#v", id, report)
		}
	}
}

func TestInstallPlanExpandsAndOrders(t *testing.T) {
	cats := mustCatalog(t)

	plan, err := InstallPlan(cats, []string{"jupyter-lab"})
	if err != nil {
		t.Fatalf("InstallPlan returned error: %v", err)
	}

	got := appIDs(plan)
	want := []string{"base-system", "curl", "miniforge", "zgx-python-env", "jupyter-lab"}
	for _, id := range want {
		if !slices.Contains(got, id) {
			t.Fatalf("InstallPlan missing %q: %v", id, got)
		}
	}
	if got[0] != "base-system" {
		t.Fatalf("InstallPlan first app = %q, want base-system; plan=%v", got[0], got)
	}

	pos := positions(got)
	assertBefore(t, pos, "curl", "miniforge")
	assertBefore(t, pos, "miniforge", "zgx-python-env")
	assertBefore(t, pos, "zgx-python-env", "jupyter-lab")
}

func TestInstallSkipsAlreadyInstalled(t *testing.T) {
	cats := mustCatalog(t)
	runner := newFakeRunner(cats, map[string][]int{
		"base-system": {0},
		"curl":        {0},
	}, nil)

	report, err := (&Engine{Runner: runner}).Install(context.Background(), cats, []string{"curl"}, "pw")
	if err != nil {
		t.Fatalf("Install returned error: %v", err)
	}

	if !slices.Contains(report.AlreadyInstalled, "curl") {
		t.Fatalf("curl not in AlreadyInstalled: %#v", report)
	}
	if slices.Contains(runner.installIDs(), "curl") {
		t.Fatalf("curl installCommand was run even though verify was green: %v", runner.calls)
	}
}

func TestInstallPartialFailureContinues(t *testing.T) {
	cats := mustCatalog(t)
	runner := newFakeRunner(cats, map[string][]int{
		"base-system": {0},
		"curl":        {1, 0},
		"miniforge":   {1, 1},
		"ollama":      {1, 0},
	}, map[string]int{
		"miniforge": 1,
	})

	report, err := (&Engine{Runner: runner}).Install(context.Background(), cats, []string{"miniforge", "ollama"}, "pw")
	if err != nil {
		t.Fatalf("Install returned error: %v", err)
	}

	if !slices.Contains(report.Failed, "miniforge") {
		t.Fatalf("miniforge not in Failed: %#v", report)
	}
	if !slices.Contains(report.Installed, "ollama") {
		t.Fatalf("ollama was not installed after miniforge failure: %#v", report)
	}
	if runner.installIndex("ollama") <= runner.installIndex("miniforge") {
		t.Fatalf("later app was not attempted after failure, installIDs=%v", runner.installIDs())
	}
}

func TestUnknownAppIsLoudError(t *testing.T) {
	cats := mustCatalog(t)
	runner := newFakeRunner(cats, nil, nil)

	_, err := (&Engine{Runner: runner}).Install(context.Background(), cats, []string{"does-not-exist"}, "pw")
	if err == nil {
		t.Fatal("Install with unknown app id returned nil error")
	}
	if len(runner.calls) != 0 {
		t.Fatalf("Run was called before unknown app failed: %v", runner.calls)
	}
}

func TestUninstallSkipsNonUninstallable(t *testing.T) {
	cats := mustCatalog(t)
	runner := newFakeRunner(cats, nil, nil)

	report, err := (&Engine{Runner: runner}).Uninstall(context.Background(), cats, []string{"base-system"}, "pw")
	if err != nil {
		t.Fatalf("Uninstall returned error: %v", err)
	}
	if !slices.Contains(report.Skipped, "base-system") {
		t.Fatalf("base-system not in Skipped: %#v", report)
	}
	if slices.Contains(report.Failed, "base-system") {
		t.Fatalf("base-system ended up in Failed: %#v", report)
	}
}

func TestUninstallDoesNotExpandDependencies(t *testing.T) {
	cats := mustCatalog(t)
	runner := newFakeRunner(cats, nil, nil)

	_, err := (&Engine{Runner: runner}).Uninstall(context.Background(), cats, []string{"jupyter-lab"}, "pw")
	if err != nil {
		t.Fatalf("Uninstall returned error: %v", err)
	}

	got := runner.uninstallIDs()
	want := []string{"jupyter-lab"}
	if !slices.Equal(got, want) {
		t.Fatalf("uninstall IDs = %v, want only %v", got, want)
	}
	for _, id := range []string{"miniforge", "curl", "zgx-python-env"} {
		if slices.Contains(got, id) {
			t.Fatalf("%s uninstallCommand was run while uninstalling jupyter-lab: %v", id, got)
		}
	}
}

func TestUninstallReverseOrderAmongSelected(t *testing.T) {
	cats := mustCatalog(t)
	runner := newFakeRunner(cats, nil, nil)

	_, err := (&Engine{Runner: runner}).Uninstall(context.Background(), cats, []string{"miniforge", "zgx-python-env"}, "pw")
	if err != nil {
		t.Fatalf("Uninstall returned error: %v", err)
	}

	got := runner.uninstallIDs()
	want := []string{"zgx-python-env", "miniforge"}
	if !slices.Equal(got, want) {
		t.Fatalf("uninstall order = %v, want %v", got, want)
	}
}

type fakeCall struct {
	command      string
	sudoPassword string
	timeout      time.Duration
	retries      int
}

type fakeRunner struct {
	appsByVerify    map[string]string
	appsByInstall   map[string]string
	appsByUninstall map[string]string
	verifyResults   map[string][]int
	installExit     map[string]int
	calls           []fakeCall
}

func newFakeRunner(cats []catalog.Category, verifyResults map[string][]int, installExit map[string]int) *fakeRunner {
	r := &fakeRunner{
		appsByVerify:    map[string]string{},
		appsByInstall:   map[string]string{},
		appsByUninstall: map[string]string{},
		verifyResults:   cloneVerifyResults(verifyResults),
		installExit:     installExit,
	}
	for _, app := range catalog.AllApps(cats) {
		r.appsByVerify[app.VerifyCommand] = app.ID
		command, _ := SudoCommand(app.InstallCommand)
		r.appsByInstall[command] = app.ID
		if app.UninstallCommand != nil {
			command, _ := SudoCommand(*app.UninstallCommand)
			r.appsByUninstall[command] = app.ID
		}
	}
	return r
}

func (r *fakeRunner) Run(_ context.Context, command, sudoPassword string, timeout time.Duration, retries int) (CommandResult, error) {
	r.calls = append(r.calls, fakeCall{
		command:      command,
		sudoPassword: sudoPassword,
		timeout:      timeout,
		retries:      retries,
	})

	if id, ok := r.appsByVerify[command]; ok {
		results := r.verifyResults[id]
		if len(results) == 0 {
			return CommandResult{ExitCode: 1}, nil
		}
		exit := results[0]
		r.verifyResults[id] = results[1:]
		return CommandResult{ExitCode: exit}, nil
	}

	if id, ok := r.appsByInstall[command]; ok {
		return CommandResult{ExitCode: r.installExit[id]}, nil
	}

	if id, ok := r.appsByUninstall[command]; ok {
		return CommandResult{ExitCode: r.installExit[id]}, nil
	}

	if id := r.installIDFromCommand(command); id != "" {
		return CommandResult{ExitCode: r.installExit[id]}, nil
	}

	return CommandResult{ExitCode: 0}, nil
}

func (r *fakeRunner) installIDs() []string {
	var ids []string
	for _, call := range r.calls {
		if id, ok := r.appsByInstall[call.command]; ok {
			ids = append(ids, id)
		}
	}
	return ids
}

func (r *fakeRunner) uninstallIDs() []string {
	var ids []string
	for _, call := range r.calls {
		if id, ok := r.appsByUninstall[call.command]; ok {
			ids = append(ids, id)
		}
	}
	return ids
}

func (r *fakeRunner) installIndex(id string) int {
	for i, got := range r.installIDs() {
		if got == id {
			return i
		}
	}
	return -1
}

func (r *fakeRunner) installIDFromCommand(command string) string {
	for installCommand, id := range r.appsByInstall {
		if strings.Contains(command, installCommand) {
			return id
		}
	}
	return ""
}

func cloneVerifyResults(in map[string][]int) map[string][]int {
	out := make(map[string][]int, len(in))
	for id, values := range in {
		out[id] = append([]int(nil), values...)
	}
	return out
}

func mustCatalog(t *testing.T) []catalog.Category {
	t.Helper()
	cats, err := catalog.Load()
	if err != nil {
		t.Fatalf("catalog.Load() failed: %v", err)
	}
	return cats
}

func appIDs(apps []catalog.App) []string {
	ids := make([]string, 0, len(apps))
	for _, app := range apps {
		ids = append(ids, app.ID)
	}
	return ids
}

func positions(ids []string) map[string]int {
	pos := make(map[string]int, len(ids))
	for i, id := range ids {
		pos[id] = i
	}
	return pos
}

func assertBefore(t *testing.T, pos map[string]int, before, after string) {
	t.Helper()
	if pos[before] >= pos[after] {
		t.Fatalf("%s did not come before %s: %v", before, after, pos)
	}
}
