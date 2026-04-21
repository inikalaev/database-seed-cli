package sqlemit

import (
	"fmt"
	"sort"
	"strings"

	"github.com/inikalaev/database-seed-cli/internal/config"
)

// uniqueTracker rejects row tuples that would collide with already-emitted
// rows on any UNIQUE constraint (composite UNIQUE, PK when not covered, etc.).
// State persists for the lifetime of one emitTable call — trackers are
// per-table, so uniqueness across the whole emit run is enforced separately
// by the FK pool and the table-scoped tracker together.
type uniqueTracker struct {
	sets []uniqueSet
}

type uniqueSet struct {
	// indices into the row values slice (parallel to `cols` in emitTable).
	indices []int
	// allLiteral is true when every column in this set is a fixed Value —
	// retries can't resolve a collision so the first dup is the only hit we'll
	// ever see. Skips the retry loop on collisions.
	allLiteral bool
	seen       map[string]bool
}

func buildUniqueTracker(tbl *config.Table, cols []string) *uniqueTracker {
	colIdx := map[string]int{}
	for i, c := range cols {
		colIdx[c] = i
	}

	// Collect all unique column groups: explicit UNIQUE + PRIMARY KEY.
	// Partial UNIQUE indexes are included as best-effort: we over-constrain
	// by ignoring the predicate (tracking every row as if it were in the
	// domain), which is safer than the default of letting apply fail on a
	// predicate hit. Worst case: extra row drops via DropInfo reporting.
	groups := make([][]string, 0, len(tbl.UniqueKeys)+len(tbl.PartialUniqueKeys)+1)
	groups = append(groups, tbl.UniqueKeys...)
	for _, p := range tbl.PartialUniqueKeys {
		groups = append(groups, p.Columns)
	}
	if len(tbl.PrimaryKey) > 0 {
		groups = append(groups, tbl.PrimaryKey)
	}

	tr := &uniqueTracker{}
	seenGroup := map[string]bool{}
	for _, g := range groups {
		if len(g) == 0 {
			continue
		}
		// Dedupe by sorted column set: PK [id] and UniqueKeys [[id]] are the
		// same tracker, PG often exposes both.
		sorted := append([]string(nil), g...)
		sort.Strings(sorted)
		dedupeKey := strings.Join(sorted, "\x00")
		if seenGroup[dedupeKey] {
			continue
		}
		seenGroup[dedupeKey] = true

		idx := make([]int, 0, len(g))
		skip := false
		for _, name := range g {
			pos, ok := colIdx[name]
			if !ok {
				// Column declared in constraint but not emitted (removed /
				// missing). Skip the whole set — we can't track partial keys.
				skip = true
				break
			}
			idx = append(idx, pos)
		}
		if skip {
			continue
		}
		allLit := true
		for _, name := range g {
			spec := tbl.Columns[name]
			if spec == nil || spec.Value == nil {
				allLit = false
				break
			}
		}
		tr.sets = append(tr.sets, uniqueSet{
			indices:    idx,
			allLiteral: allLit,
			seen:       map[string]bool{},
		})
	}
	return tr
}

// check returns (ok, canRetry). ok=true means the row is unique across all
// tracked sets. canRetry=false means at least one violated set is all-literal,
// so resampling can't help — caller should drop the row immediately.
func (tr *uniqueTracker) check(row []any) (bool, bool) {
	if tr == nil || len(tr.sets) == 0 {
		return true, true
	}
	canRetry := true
	for i := range tr.sets {
		s := &tr.sets[i]
		if tupleHasNil(row, s.indices) {
			// PG default is NULLS DISTINCT — two rows with NULL in the
			// constrained column don't collide. Skip.
			continue
		}
		key := tupleKey(row, s.indices)
		if s.seen[key] {
			if s.allLiteral {
				canRetry = false
			}
			return false, canRetry
		}
	}
	return true, canRetry
}

// commit records the row as emitted. Call only when the row will actually be
// written — dropped rows must not reserve tuple keys.
func (tr *uniqueTracker) commit(row []any) {
	if tr == nil {
		return
	}
	for i := range tr.sets {
		s := &tr.sets[i]
		if tupleHasNil(row, s.indices) {
			continue
		}
		s.seen[tupleKey(row, s.indices)] = true
	}
}

func tupleHasNil(row []any, idx []int) bool {
	for _, i := range idx {
		if row[i] == nil {
			return true
		}
	}
	return false
}

// tupleKey builds a deterministic string key for a tuple. fmt.Sprintf("%v",
// val) is stable across runs for every type sqlemit emits (strings, numbers,
// time.Time, []byte, []any), which is sufficient — we never need to parse the
// key back.
func tupleKey(row []any, idx []int) string {
	var b strings.Builder
	for n, i := range idx {
		if n > 0 {
			b.WriteByte(0x01)
		}
		fmt.Fprintf(&b, "%v", row[i])
	}
	return b.String()
}
