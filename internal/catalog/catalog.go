// Package catalog owns the curated app catalog without CLI or UI coupling.
package catalog

import (
	"embed"
	"encoding/json"
	"fmt"
)

// App describes one installable app in the catalog.
type App struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Icon               string   `json:"icon"`
	Description        string   `json:"description"`
	Features           []string `json:"features"`
	Category           string   `json:"category"`
	InstallCommand     string   `json:"installCommand"`
	VerifyCommand      string   `json:"verifyCommand"`
	UninstallCommand   *string  `json:"uninstallCommand,omitempty"`
	RequiresVirtualEnv bool     `json:"requiresVirtualEnv,omitempty"`
	Dependencies       []string `json:"dependencies,omitempty"`
}

// Category groups apps the same way the source does.
type Category struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Apps        []App  `json:"apps"`
}

var _ embed.FS

//go:embed catalog.json
var catalogJSON []byte

// Load reads the embedded catalog and fails loudly on invalid data.
func Load() ([]Category, error) {
	var cats []Category
	if err := json.Unmarshal(catalogJSON, &cats); err != nil {
		return nil, fmt.Errorf("catalog: parsing embedded catalog.json failed: %w", err)
	}

	return cats, nil
}

// AllApps flattens the catalog for lookup and validation.
func AllApps(cats []Category) []App {
	apps := make([]App, 0)
	for _, cat := range cats {
		apps = append(apps, cat.Apps...)
	}

	return apps
}

// ByID finds an app without assuming catalog ordering.
func ByID(cats []Category, id string) (App, bool) {
	for _, app := range AllApps(cats) {
		if app.ID == id {
			return app, true
		}
	}

	return App{}, false
}

// CategoryByID finds a category without exposing internal storage.
func CategoryByID(cats []Category, id string) (Category, bool) {
	for _, cat := range cats {
		if cat.ID == id {
			return cat, true
		}
	}

	return Category{}, false
}
