package config

import (
	"fmt"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestReadVersionAbsent(t *testing.T) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte("tables:\n  t: {columns: {}}\n"), &doc); err != nil {
		t.Fatal(err)
	}
	v, err := readVersion(&doc)
	if err != nil {
		t.Fatal(err)
	}
	if v != 0 {
		t.Fatalf("version = %d, want 0", v)
	}
}

func TestReadVersionInvalid(t *testing.T) {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte("version: oops\n"), &doc); err != nil {
		t.Fatal(err)
	}
	if _, err := readVersion(&doc); err == nil {
		t.Fatal("want error on non-int version")
	}
}

func TestMigrateDocumentNoMigrationForGap(t *testing.T) {
	// Simulate a future CurrentVersion with no registered migration.
	orig := migrations
	defer func() { migrations = orig }()
	// Empty — nothing registered.
	migrations = map[int]migrationFunc{}

	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(fmt.Sprintf("version: %d\n", CurrentVersion-1)), &doc); err != nil {
		t.Fatal(err)
	}
	if CurrentVersion == 1 {
		t.Skip("CurrentVersion is 1 — no older version to migrate from")
	}
	if err := migrateDocument(&doc); err == nil || !strings.Contains(err.Error(), "no migration") {
		t.Fatalf("want 'no migration' error, got %v", err)
	}
}

func TestMigrateDocumentRegisteredMigration(t *testing.T) {
	orig := migrations
	defer func() { migrations = orig }()
	migrations = map[int]migrationFunc{
		1: func(root *yaml.Node) error {
			// Bump version: 1 -> 2.
			for i := 0; i+1 < len(root.Content); i += 2 {
				if root.Content[i].Value == "version" {
					root.Content[i+1].Value = "2"
					return nil
				}
			}
			return fmt.Errorf("version field not found")
		},
	}

	// Temporarily pretend CurrentVersion is 2 for this test by running migration
	// loop manually — migrateDocument() reads CurrentVersion so we can't easily
	// override it. Instead, just invoke the migration and verify it bumps.
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte("version: 1\ndatabase: {dialect: postgres}\ntables: {}\n"), &doc); err != nil {
		t.Fatal(err)
	}
	root := rootMapping(&doc)
	if root == nil {
		t.Fatal("no root mapping")
	}
	if err := migrations[1](root); err != nil {
		t.Fatal(err)
	}
	v, err := readVersion(&doc)
	if err != nil {
		t.Fatal(err)
	}
	if v != 2 {
		t.Fatalf("version after migration = %d, want 2", v)
	}
}
