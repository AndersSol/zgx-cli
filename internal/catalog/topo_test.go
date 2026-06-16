package catalog

import (
	"strings"
	"testing"
)

func TestInstallOrderRespectsDeps(t *testing.T) {
	apps := []App{
		{ID: "d", Dependencies: []string{"c"}},
		{ID: "c", Dependencies: []string{"a", "b"}},
		{ID: "b", Dependencies: []string{"a"}},
		{ID: "a"},
	}

	order, err := InstallOrder(apps)
	if err != nil {
		t.Fatalf("InstallOrder() returned error: %v", err)
	}

	assertDepsBeforeApps(t, order)
}

func TestInstallOrderDeterministic(t *testing.T) {
	apps := []App{
		{ID: "d", Dependencies: []string{"c"}},
		{ID: "c", Dependencies: []string{"a", "b"}},
		{ID: "b", Dependencies: []string{"a"}},
		{ID: "a"},
	}

	first, err := InstallOrder(apps)
	if err != nil {
		t.Fatalf("first InstallOrder() returned error: %v", err)
	}
	second, err := InstallOrder(apps)
	if err != nil {
		t.Fatalf("second InstallOrder() returned error: %v", err)
	}

	if got, want := appIDs(first), appIDs(second); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("InstallOrder() is not deterministic: %v != %v", got, want)
	}
}

func TestInstallOrderCycleIsLoudError(t *testing.T) {
	apps := []App{
		{ID: "a", Dependencies: []string{"b"}},
		{ID: "b", Dependencies: []string{"a"}},
	}

	_, err := InstallOrder(apps)
	if err == nil {
		t.Fatal("InstallOrder() returned nil error for cycle")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("InstallOrder() error = %q, want message mentioning cycle", err.Error())
	}
}

func TestInstallOrderDepOutsideSetSkipped(t *testing.T) {
	apps := []App{
		{ID: "x", Dependencies: []string{"ghost"}},
	}

	order, err := InstallOrder(apps)
	if err != nil {
		t.Fatalf("InstallOrder() returned error: %v", err)
	}
	if got, want := appIDs(order), []string{"x"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("InstallOrder() = %v, want %v", got, want)
	}
}

func TestInstallOrderRealCatalog(t *testing.T) {
	cats, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	order, err := InstallOrder(AllApps(cats))
	if err != nil {
		t.Fatalf("InstallOrder(AllApps(...)) returned error: %v", err)
	}

	pos := positionsByID(order)
	assertBefore(t, pos, "miniforge", "zgx-python-env")
	assertBefore(t, pos, "zgx-python-env", "jupyter-lab")
	assertBefore(t, pos, "curl", "ollama")
}

func assertDepsBeforeApps(t *testing.T, apps []App) {
	t.Helper()

	pos := positionsByID(apps)
	for _, app := range apps {
		appPos, ok := pos[app.ID]
		if !ok {
			t.Fatalf("missing position for %q", app.ID)
		}
		for _, depID := range app.Dependencies {
			depPos, ok := pos[depID]
			if !ok {
				continue
			}
			if depPos >= appPos {
				t.Fatalf("%s is at position %d, must come before %s at position %d", depID, depPos, app.ID, appPos)
			}
		}
	}
}

func assertBefore(t *testing.T, pos map[string]int, before string, after string) {
	t.Helper()

	beforePos, ok := pos[before]
	if !ok {
		t.Fatalf("missing position for %q", before)
	}
	afterPos, ok := pos[after]
	if !ok {
		t.Fatalf("missing position for %q", after)
	}
	if beforePos >= afterPos {
		t.Fatalf("%s is at position %d, must come before %s at position %d", before, beforePos, after, afterPos)
	}
}

func positionsByID(apps []App) map[string]int {
	pos := make(map[string]int, len(apps))
	for i, app := range apps {
		pos[app.ID] = i
	}

	return pos
}

func appIDs(apps []App) []string {
	ids := make([]string, 0, len(apps))
	for _, app := range apps {
		ids = append(ids, app.ID)
	}

	return ids
}
