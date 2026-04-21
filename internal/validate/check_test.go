package validate

import (
	"testing"

	"github.com/inikalaev/database-seed-cli/internal/config"
	"github.com/inikalaev/database-seed-cli/internal/factories"
	"github.com/inikalaev/database-seed-cli/internal/registry"
	"github.com/inikalaev/database-seed-cli/pkg/seedapi"
)

func fullReg() *registry.Registry {
	return registry.New(factories.All())
}

func emptyReg() *registry.Registry {
	return registry.New(nil)
}

func oneTable(key string, t *config.Table) *config.Config {
	// relations.Build requires Schema+Name to be set on each table (they are
	// computed by config.Unmarshal but not set when building manually).
	t.Schema = "public"
	t.Name = "t"
	return &config.Config{
		Version:  1,
		Database: config.DatabaseSection{Dialect: "postgres"},
		Tables:   map[string]*config.Table{key: t},
	}
}

func col(factory string) *config.ColumnSpec {
	return &config.ColumnSpec{Factory: factory}
}

func rowCount(n int) *int { return &n }

// checkKind collects all issue kinds from Check and returns a set.
func checkKinds(t *testing.T, cfg *config.Config, reg *registry.Registry) map[Kind]int {
	t.Helper()
	issues, err := Check(cfg, reg)
	if err != nil {
		t.Fatalf("Check error: %v", err)
	}
	out := map[Kind]int{}
	for _, i := range issues {
		out[i.Kind]++
	}
	return out
}

// --- Column-level checks ---

func TestCheck_Unresolved(t *testing.T) {
	cfg := oneTable("public.t", &config.Table{
		RowCount: rowCount(1),
		Columns:  map[string]*config.ColumnSpec{"c": {Unresolved: true, Factory: "string"}},
	})
	kinds := checkKinds(t, cfg, fullReg())
	if kinds[KindUnresolved] != 1 {
		t.Fatalf("unresolved count = %d, want 1", kinds[KindUnresolved])
	}
}

func TestCheck_NoFactory(t *testing.T) {
	cfg := oneTable("public.t", &config.Table{
		RowCount: rowCount(1),
		Columns:  map[string]*config.ColumnSpec{"c": {Factory: ""}},
	})
	kinds := checkKinds(t, cfg, emptyReg())
	if kinds[KindNoFactory] != 1 {
		t.Fatalf("no-factory count = %d, want 1", kinds[KindNoFactory])
	}
}

func TestCheck_UnknownFactory(t *testing.T) {
	cfg := oneTable("public.t", &config.Table{
		RowCount: rowCount(1),
		Columns:  map[string]*config.ColumnSpec{"c": {Factory: "ghost"}},
	})
	kinds := checkKinds(t, cfg, emptyReg())
	if kinds[KindUnknownFactory] != 1 {
		t.Fatalf("unknown-factory count = %d, want 1", kinds[KindUnknownFactory])
	}
}

func TestCheck_FKRefMissingTarget(t *testing.T) {
	cfg := oneTable("public.t", &config.Table{
		RowCount: rowCount(1),
		Columns:  map[string]*config.ColumnSpec{"c": {Factory: seedapi.FactoryFKRef}},
	})
	kinds := checkKinds(t, cfg, fullReg())
	if kinds[KindFKRefMissingTarget] != 1 {
		t.Fatalf("fkref-missing-target count = %d, want 1", kinds[KindFKRefMissingTarget])
	}
}

func TestCheck_FKRefTargetNotFound(t *testing.T) {
	cfg := oneTable("public.t", &config.Table{
		RowCount: rowCount(1),
		Columns: map[string]*config.ColumnSpec{
			"c": {Factory: seedapi.FactoryFKRef, Params: map[string]any{"target": "public.ghost.id"}},
		},
	})
	kinds := checkKinds(t, cfg, fullReg())
	if kinds[KindFKRefTargetNotFound] != 1 {
		t.Fatalf("fkref-target-not-found count = %d, want 1", kinds[KindFKRefTargetNotFound])
	}
}

func TestCheck_FKRefEmptyPool(t *testing.T) {
	zero := 0
	cfg := &config.Config{
		Version:  1,
		Database: config.DatabaseSection{Dialect: "postgres"},
		Tables: map[string]*config.Table{
			"public.parent": {
				Schema:     "public",
				Name:       "parent",
				RowCount:   &zero,
				PrimaryKey: []string{"id"},
				Columns:    map[string]*config.ColumnSpec{"id": {Factory: "uuid"}},
			},
			"public.child": {
				Schema:   "public",
				Name:     "child",
				RowCount: rowCount(5),
				Columns: map[string]*config.ColumnSpec{
					"parent_id": {
						Factory:  seedapi.FactoryFKRef,
						Params:   map[string]any{"target": "public.parent.id"},
						Nullable: false,
					},
				},
			},
		},
	}
	kinds := checkKinds(t, cfg, fullReg())
	if kinds[KindFKRefEmptyPool] != 1 {
		t.Fatalf("fkref-empty-pool count = %d, want 1", kinds[KindFKRefEmptyPool])
	}
}

// --- Table-level checks ---

func TestCheck_RowCountPerMissing(t *testing.T) {
	cfg := oneTable("public.t", &config.Table{
		RowCount:    rowCount(1),
		RowCountPer: map[string][2]int{"ghost": {1, 3}},
		Columns:     map[string]*config.ColumnSpec{"c": {Factory: "string"}},
	})
	kinds := checkKinds(t, cfg, fullReg())
	if kinds[KindRowCountPerMissing] != 1 {
		t.Fatalf("row-count-per-missing count = %d, want 1", kinds[KindRowCountPerMissing])
	}
}

func TestCheck_CompositeUnique(t *testing.T) {
	cfg := oneTable("public.t", &config.Table{
		RowCount:   rowCount(1),
		UniqueKeys: [][]string{{"a", "b"}},
		Columns: map[string]*config.ColumnSpec{
			"a": {Factory: "string"},
			"b": {Factory: "string"},
		},
	})
	kinds := checkKinds(t, cfg, fullReg())
	if kinds[KindCompositeUnique] != 1 {
		t.Fatalf("composite-unique count = %d, want 1", kinds[KindCompositeUnique])
	}
}

func TestCheck_UniqueUnsafeFactory(t *testing.T) {
	cfg := oneTable("public.t", &config.Table{
		RowCount:   rowCount(1),
		UniqueKeys: [][]string{{"name"}},
		// city does not implement UniqueGenerator → should warn.
		Columns: map[string]*config.ColumnSpec{"name": {Factory: "city"}},
	})
	kinds := checkKinds(t, cfg, fullReg())
	if kinds[KindUniqueUnsafeFactory] != 1 {
		t.Fatalf("unique-unsafe-factory count = %d, want 1", kinds[KindUniqueUnsafeFactory])
	}
}

func TestCheck_CheckNotApplied(t *testing.T) {
	cfg := oneTable("public.t", &config.Table{
		RowCount: rowCount(1),
		Columns:  map[string]*config.ColumnSpec{"score": {Factory: "integer"}},
		Checks: []config.CheckConstraint{
			{Name: "score_positive", Expression: "score > 0", Columns: []string{"score"}},
		},
	})
	kinds := checkKinds(t, cfg, fullReg())
	if kinds[KindCheckNotApplied] != 1 {
		t.Fatalf("check-not-applied count = %d, want 1", kinds[KindCheckNotApplied])
	}
}

func TestCheck_Exclude(t *testing.T) {
	cfg := oneTable("public.t", &config.Table{
		RowCount: rowCount(1),
		Columns:  map[string]*config.ColumnSpec{"r": {Factory: "string"}},
		Excludes: []config.ExcludeConstraint{
			{Name: "no_overlap", Definition: "USING gist (r WITH &&)"},
		},
	})
	kinds := checkKinds(t, cfg, fullReg())
	if kinds[KindExclude] != 1 {
		t.Fatalf("exclude count = %d, want 1", kinds[KindExclude])
	}
}

func TestCheck_PartialUnique(t *testing.T) {
	cfg := oneTable("public.t", &config.Table{
		RowCount: rowCount(1),
		Columns:  map[string]*config.ColumnSpec{"c": {Factory: "string"}},
		PartialUniqueKeys: []config.PartialUniqueKey{
			{Columns: []string{"c"}, Predicate: "deleted_at IS NULL"},
		},
	})
	kinds := checkKinds(t, cfg, fullReg())
	if kinds[KindPartialUnique] != 1 {
		t.Fatalf("partial-unique count = %d, want 1", kinds[KindPartialUnique])
	}
}

func TestCheck_ValueTypeMismatch(t *testing.T) {
	cfg := oneTable("public.t", &config.Table{
		RowCount: rowCount(1),
		Columns: map[string]*config.ColumnSpec{
			"n": {Value: "not-an-int", DataType: "integer"},
		},
	})
	kinds := checkKinds(t, cfg, fullReg())
	if kinds[KindValueTypeMismatch] != 1 {
		t.Fatalf("value-type-mismatch count = %d, want 1", kinds[KindValueTypeMismatch])
	}
}

func TestCheck_RemvedColumnSkipped(t *testing.T) {
	cfg := oneTable("public.t", &config.Table{
		RowCount: rowCount(1),
		Columns: map[string]*config.ColumnSpec{
			"c": {Removed: true, Factory: ""},
		},
	})
	kinds := checkKinds(t, cfg, emptyReg())
	if kinds[KindNoFactory] != 0 {
		t.Fatal("removed column must not emit no-factory issue")
	}
}

func TestCheck_RemovedTableSkipped(t *testing.T) {
	cfg := oneTable("public.t", &config.Table{
		Removed:  true,
		RowCount: rowCount(1),
		Columns:  map[string]*config.ColumnSpec{"c": {Factory: ""}},
	})
	kinds := checkKinds(t, cfg, emptyReg())
	if len(kinds) != 0 {
		t.Fatalf("removed table must emit no issues, got %v", kinds)
	}
}

// --- Aggregate helpers ---

func TestCounts(t *testing.T) {
	issues := []Issue{
		{Level: LevelErr},
		{Level: LevelErr},
		{Level: LevelWarn},
		{Level: LevelInfo},
	}
	errs, warns, infos := Counts(issues)
	if errs != 2 || warns != 1 || infos != 1 {
		t.Fatalf("Counts = %d/%d/%d, want 2/1/1", errs, warns, infos)
	}
}

func TestHasErrors(t *testing.T) {
	if HasErrors([]Issue{{Level: LevelWarn}}) {
		t.Fatal("warn-only must not HasErrors")
	}
	if !HasErrors([]Issue{{Level: LevelErr}}) {
		t.Fatal("err must HasErrors")
	}
}

func TestHasFixable(t *testing.T) {
	if HasFixable([]Issue{{Level: LevelErr}}) {
		t.Fatal("issue without Fix must not HasFixable")
	}
	if !HasFixable([]Issue{{Level: LevelErr, Fix: &FixSpec{}}}) {
		t.Fatal("issue with Fix must HasFixable")
	}
}

// --- Kind.String ---

func TestKindString_Known(t *testing.T) {
	cases := map[Kind]string{
		KindUnresolved:          "unresolved",
		KindNoFactory:           "no-factory",
		KindUnknownFactory:      "unknown-factory",
		KindFKRefMissingTarget:  "fkref-missing-target",
		KindFKRefTargetNotFound: "fkref-target-not-found",
		KindFKRefEmptyPool:      "fkref-empty-pool",
		KindMissingFactoryParam: "missing-factory-param",
		KindJsonFieldUnresolved: "json-field-unresolved",
	}
	for k, want := range cases {
		if got := k.String(); got != want {
			t.Errorf("Kind(%d).String() = %q, want %q", k, got, want)
		}
	}
}

func TestKindString_Unknown(t *testing.T) {
	if got := Kind(9999).String(); got != "unknown" {
		t.Fatalf("unknown kind = %q, want 'unknown'", got)
	}
}

// --- UniqueSafeFactory ---

func TestUniqueSafeFactory_UUID(t *testing.T) {
	reg := fullReg()
	if !UniqueSafeFactory("uuid", reg) {
		t.Fatal("uuid must be unique-safe")
	}
}

func TestUniqueSafeFactory_City(t *testing.T) {
	reg := fullReg()
	if UniqueSafeFactory("city", reg) {
		t.Fatal("city must not be unique-safe")
	}
}

func TestUniqueSafeFactory_Unknown(t *testing.T) {
	reg := fullReg()
	if UniqueSafeFactory("ghost", reg) {
		t.Fatal("unknown factory must not be unique-safe")
	}
}
