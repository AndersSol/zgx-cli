package catalog

import "testing"

func TestLoad(t *testing.T) {
	cats, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if len(cats) == 0 {
		t.Fatal("Load() returned no categories")
	}
}

func TestAllApps(t *testing.T) {
	cats, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	apps := AllApps(cats)
	if got, want := len(apps), 17; got != want {
		t.Fatalf("len(AllApps(...)) = %d, want %d", got, want)
	}
}

func TestByIDKnownApps(t *testing.T) {
	cats, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	for _, id := range []string{"base-system", "ollama", "zgx-python-env", "poetry"} {
		if _, ok := ByID(cats, id); !ok {
			t.Errorf("ByID(..., %q) did not find app", id)
		}
	}
}

func TestCommandsArePresent(t *testing.T) {
	cats, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	for _, app := range AllApps(cats) {
		if app.InstallCommand == "" {
			t.Errorf("%s missing installCommand", app.ID)
		}
		if app.VerifyCommand == "" {
			t.Errorf("%s missing verifyCommand", app.ID)
		}
	}
}

func TestDependenciesReferToExistingApps(t *testing.T) {
	cats, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	for _, app := range AllApps(cats) {
		for _, depID := range app.Dependencies {
			if _, ok := ByID(cats, depID); !ok {
				t.Errorf("%s has unknown dependency %q", app.ID, depID)
			}
		}
	}
}

func TestUninstallCommandSemantics(t *testing.T) {
	cats, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	base, ok := ByID(cats, "base-system")
	if !ok {
		t.Fatal("base-system does not exist")
	}
	if base.UninstallCommand != nil {
		t.Fatal("base-system should have nil UninstallCommand")
	}

	ollama, ok := ByID(cats, "ollama")
	if !ok {
		t.Fatal("ollama does not exist")
	}
	if ollama.UninstallCommand == nil {
		t.Fatal("ollama should have UninstallCommand")
	}
}
