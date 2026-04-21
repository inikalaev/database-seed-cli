package seedapi

// SetupKind selects the prompt type the CLI renders for a SetupStep.
//
// The supported kinds are deliberately few so factories can rely on the CLI
// producing a value of a known Go type:
//
//   - SetupString — free-form text; user must enter a non-empty value. Stored
//     as string (already TrimSpace'd).
//   - SetupInt    — integer; validated before accept. Stored as int.
//   - SetupFloat  — decimal; validated before accept. Stored as float64.
//   - SetupBool   — survey.Select{"true","false"}. Stored as bool.
//   - SetupList   — comma-separated entries, each parsed by Element.Kind.
//     Stored as []any where each element has the Go type produced by the
//     corresponding scalar kind. Requires a non-nil Element.
//
// For anything more complex (nested maps, multi-level lists, arbitrary JSON)
// return a SetupString and parse the raw value inside your factory's
// Generate — the CLI is intentionally scalar-only at the prompt layer.
type SetupKind string

const (
	SetupString SetupKind = "string"
	SetupInt    SetupKind = "int"
	SetupFloat  SetupKind = "float"
	SetupBool   SetupKind = "bool"
	SetupList   SetupKind = "list"
)

// SetupStep describes a single parameter the factory wants the CLI to
// collect and store under col.Params[ParamKey].
//
// Fields:
//
//   - ParamKey — key under col.Params where the accepted value is written.
//     Must be stable: the factory looks it up by the same key in Generate.
//   - Kind     — one of the SetupKind constants. Determines prompt UI and
//     the Go type written into col.Params.
//   - Element  — only meaningful when Kind == SetupList. Declares the kind
//     of each list element. Nesting is one level deep; Element.Element is
//     ignored.
//   - Prompt   — user-facing message shown as the survey prompt title.
//     Should be short and actionable, e.g. "Allowed values (comma-
//     separated):" or "Byte size:".
//   - Help     — optional help text shown when the user presses `?` in the
//     prompt. Useful for example values or units.
//   - Required — when true, the user must enter a value or switch factory;
//     the "skip" option is hidden. When false, "skip (use default)" is
//     offered and the factory must cope with the param being absent.
type SetupStep struct {
	ParamKey string
	Kind     SetupKind
	Element  *SetupStep
	Prompt   string
	Help     string
	Required bool
}

// Configurable is the optional interface a factory implements to declare
// parameters it wants the CLI to collect interactively.
//
// Two call sites invoke RequiredSetup:
//
//  1. `seed-cli validate` — walks every column, and for each factory that
//     implements Configurable it emits a `missing-factory-param` warning
//     per required step still unsatisfied.
//  2. `seed-cli fix` — after the user picks a factory (for a column or a
//     json sub-field) the CLI runs a cascade: call RequiredSetup, prompt
//     the user for each returned step, write the chosen value into
//     col.Params[step.ParamKey], and call RequiredSetup again. The loop
//     stops when an empty slice is returned.
//
// Semantics:
//
//   - RequiredSetup must be a pure function of `params`. It is never called
//     in hot paths (only in validate/fix), so lookups into params by key
//     are fine.
//   - Returning an empty slice means the factory is fully configured for
//     the given params. The cascade exits.
//   - At each prompt the user can pick "change factory instead", which
//     delegates back to the factory-picker. Previously stored params stay
//     untouched; the new factory either uses them or ignores them.
//
// Invariant — critical for correctness: after the CLI writes
// params[step.ParamKey] with an accepted value, the next call to
// RequiredSetup(params) must not return a step with the same ParamKey.
// Either remove it from the slice, or replace it with a different step
// (e.g. a follow-up parameter). Returning the same step unchanged would
// loop forever; the CLI caps iterations at a small number as a safety net,
// but that's an emergency stop — don't rely on it.
//
// See factories/enum_value_str.go for the simplest reference
// implementation, and the "Writing a Configurable factory" section of the
// top-level README for extended examples (branching on params, multi-step
// collection, list element kinds).
type Configurable interface {
	RequiredSetup(params map[string]any) []SetupStep
}
