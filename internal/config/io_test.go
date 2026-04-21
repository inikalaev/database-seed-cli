package config

import (
	"os"
	"path/filepath"
	"testing"
)

func minimalConfig() *Config {
	rc := 10
	return &Config{
		Version:  CurrentVersion,
		Database: DatabaseSection{Dialect: "postgres"},
		Tables: map[string]*Table{
			"public.users": {
				RowCount:   &rc,
				PrimaryKey: []string{"id"},
				Columns: map[string]*ColumnSpec{
					"id":    {Factory: "uuid"},
					"email": {Factory: "email", Unresolved: false},
					"score": {Factory: "integer", Params: map[string]any{"min": 0, "max": 100}},
					"meta":  {Values: map[string]*ColumnSpec{"plan": {Factory: "string"}}},
				},
			},
		},
	}
}

func TestMarshalUnmarshalRoundTrip(t *testing.T) {
	orig := minimalConfig()
	data, err := Marshal(orig)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Version != orig.Version {
		t.Errorf("version = %d, want %d", got.Version, orig.Version)
	}
	tbl, ok := got.Tables["public.users"]
	if !ok {
		t.Fatal("public.users missing after round-trip")
	}
	if *tbl.RowCount != 10 {
		t.Errorf("row_count = %d, want 10", *tbl.RowCount)
	}
	if tbl.Columns["email"].Factory != "email" {
		t.Errorf("email factory = %q, want 'email'", tbl.Columns["email"].Factory)
	}
	if tbl.Columns["score"].Params["min"] == nil {
		t.Error("score params.min must survive round-trip")
	}
	if tbl.Columns["meta"].Values["plan"].Factory != "string" {
		t.Errorf("meta.plan factory = %q, want 'string'", tbl.Columns["meta"].Values["plan"].Factory)
	}
}

func TestSaveLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "seed.yaml")

	orig := minimalConfig()
	if err := Save(path, orig); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not created: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Version != orig.Version {
		t.Errorf("version = %d, want %d", got.Version, orig.Version)
	}
	if _, ok := got.Tables["public.users"]; !ok {
		t.Fatal("public.users missing after Save/Load")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	if _, err := Load("/does/not/exist/seed.yaml"); err == nil {
		t.Fatal("Load must error on missing file")
	}
}

func TestUnmarshal_VersionNewer(t *testing.T) {
	yaml := "version: 9999\ndatabase:\n  dialect: postgres\ntables: {}\n"
	if _, err := Unmarshal([]byte(yaml)); err == nil {
		t.Fatal("must error on version newer than CLI supports")
	}
}

func TestUnmarshal_VersionAbsent_WarnCaptured(t *testing.T) {
	var warned bool
	yaml := "database:\n  dialect: postgres\ntables: {}\n"
	_, err := Unmarshal([]byte(yaml), WithLogger(func(msg string) {
		warned = true
		_ = msg
	}))
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !warned {
		t.Fatal("expected logger to be called for missing version field")
	}
}

func TestQualifiedKey(t *testing.T) {
	if got := QualifiedKey("public", "users"); got != "public.users" {
		t.Errorf("got %q, want 'public.users'", got)
	}
	if got := QualifiedKey("", "orders"); got != "public.orders" {
		t.Errorf("got %q, want 'public.orders'", got)
	}
}
