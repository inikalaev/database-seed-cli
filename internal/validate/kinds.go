// Package validate produces a structured list of issues from a seed config.
//
// It is the single source of truth shared by the `validate` command (which
// prints issues) and the `fix` command (which iterates and mutates the config
// based on user prompts).
package validate

// Level is the severity of an issue. ERR blocks generation; WARN highlights
// likely apply-time failures; INFO is a reminder that the generator cannot
// enforce the constraint for you.
type Level int

const (
	LevelErr Level = iota
	LevelWarn
	LevelInfo
)

// Kind is a stable identifier used by `fix` to dispatch to the right flow and
// by documentation to cross-reference the issue.
type Kind int

const (
	KindUnresolved Kind = iota
	KindNoFactory
	KindUnknownFactory
	KindValueTypeMismatch
	KindFKRefMissingTarget
	KindFKRefTargetNotFound
	KindRowCountPerMissing
	KindFKRefEmptyPool
	KindFKRefInCycle
	KindUniqueUnsafeFactory
	KindCompositeUnique
	KindCompositeFK
	KindDeferrableCycle
	KindNonDeferrableCycle
	KindCheckNotApplied
	KindExclude
	KindPartialUnique
	KindJsonFieldUnresolved
	KindMissingFactoryParam
)

var kindNames = map[Kind]string{
	KindUnresolved:          "unresolved",
	KindNoFactory:           "no-factory",
	KindUnknownFactory:      "unknown-factory",
	KindValueTypeMismatch:   "value-type-mismatch",
	KindFKRefMissingTarget:  "fkref-missing-target",
	KindFKRefTargetNotFound: "fkref-target-not-found",
	KindRowCountPerMissing:  "row-count-per-missing",
	KindFKRefEmptyPool:      "fkref-empty-pool",
	KindFKRefInCycle:        "fkref-in-cycle",
	KindUniqueUnsafeFactory: "unique-unsafe-factory",
	KindCompositeUnique:     "composite-unique",
	KindCompositeFK:         "composite-fk",
	KindDeferrableCycle:     "deferrable-cycle",
	KindNonDeferrableCycle:  "non-deferrable-cycle",
	KindCheckNotApplied:     "check-not-applied",
	KindExclude:             "exclude",
	KindPartialUnique:       "partial-unique",
	KindJsonFieldUnresolved: "json-field-unresolved",
	KindMissingFactoryParam: "missing-factory-param",
}

func (k Kind) String() string {
	if s, ok := kindNames[k]; ok {
		return s
	}
	return "unknown"
}

// Issue is one finding of Check. All fields are populated so the reporter can
// render it and the fix flow can act on it.
type Issue struct {
	Level    Level
	Kind     Kind
	Location string   // "schema.table.column" for column issues, "schema.table" for table, "fk cycle" for graph
	Message  string   // short description printed as the main text
	Hint     string   // actionable one-liner, e.g. "add factory: timestamp"
	Fix      *FixSpec // nil when auto-fix is not available (composite UNIQUE, EXCLUDE, …)
}

// FixSpec carries the context a fix flow needs to locate the config entry and
// propose options. Ctx keys are documented per Kind in fixes.go.
type FixSpec struct {
	Kind   Kind
	Table  string
	Column string
	Field  string // non-empty for KindJsonFieldUnresolved: the json field name
	Ctx    map[string]any
}
