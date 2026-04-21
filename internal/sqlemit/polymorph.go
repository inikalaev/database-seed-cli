package sqlemit

import (
	"math/rand/v2"
	"strings"
	"unicode"

	"github.com/inikalaev/database-seed-cli/internal/config"
)

// polyPlan precomputes emission metadata for a table's polymorphic pairs.
// Each pair is resolved to column indices in the active-column slice and to
// a list of ready-to-pick candidate (type name + FK-pool key) entries. A pair
// with empty candidates is still recorded so validate can flag it — the emit
// path just skips it and lets the ordinary factories run.
type polyPlan struct {
	pairs []polyPair
}

type polyPair struct {
	typeIdx    int
	idIdx      int
	candidates []polyCandidate
}

type polyCandidate struct {
	typeName   string
	fkSchema   string
	fkTable    string
	fkPkColumn string
}

// buildPolyPlan resolves every Polymorphic key on tbl into a polyPair with
// per-candidate pool coordinates. `cols` is the active-column slice order
// from activeColumns, so column indices are valid for rowVals.
func buildPolyPlan(tbl *config.Table, cols []string) *polyPlan {
	if len(tbl.Polymorphs) == 0 {
		return nil
	}
	colIdx := map[string]int{}
	for i, c := range cols {
		colIdx[c] = i
	}
	plan := &polyPlan{}
	for _, pk := range tbl.Polymorphs {
		ti, okT := colIdx[pk.TypeColumn]
		ii, okI := colIdx[pk.IdColumn]
		if !okT || !okI {
			// One of the columns was removed — leave the pair to validate.
			continue
		}
		cands := make([]polyCandidate, 0, len(pk.Candidates))
		for _, c := range pk.Candidates {
			ts, tt := splitTableKey(c.Table)
			if tt == "" {
				continue
			}
			pkCol := c.PkColumn
			if pkCol == "" {
				pkCol = "id"
			}
			typeName := c.TypeName
			if typeName == "" {
				typeName = classifyTableName(tt)
			}
			cands = append(cands, polyCandidate{
				typeName:   typeName,
				fkSchema:   ts,
				fkTable:    tt,
				fkPkColumn: pkCol,
			})
		}
		plan.pairs = append(plan.pairs, polyPair{
			typeIdx:    ti,
			idIdx:      ii,
			candidates: cands,
		})
	}
	return plan
}

func splitTableKey(qualified string) (string, string) {
	if i := strings.Index(qualified, "."); i >= 0 {
		return qualified[:i], qualified[i+1:]
	}
	return "public", qualified
}

// classifyTableName converts a snake-case plural table name to a Rails-style
// singular CamelCase class name — best-effort, conservative. Users override
// per-candidate when the app uses a non-default class name.
func classifyTableName(name string) string {
	parts := strings.Split(name, "_")
	for i, p := range parts {
		if p == "" {
			continue
		}
		r := []rune(p)
		r[0] = unicode.ToUpper(r[0])
		parts[i] = string(r)
	}
	joined := strings.Join(parts, "")
	// Trivial pluralization: drop trailing "s" if the name is longer than 1
	// char. Cases like "addresses"/"addres" are wrong on purpose — this is a
	// starting point, not a complete inflector.
	if strings.HasSuffix(joined, "s") && len(joined) > 1 {
		joined = joined[:len(joined)-1]
	}
	return joined
}

// assign picks a candidate for every polymorphic pair and writes both
// columns into rowVals. Returns the set of column indices it populated so
// the caller can skip them in the main factory loop. Pairs with empty
// candidates return no assignments — those columns fall through.
func (p *polyPlan) assign(rowVals []any, pool *memoryPool, rng *rand.Rand) map[int]bool {
	if p == nil || len(p.pairs) == 0 {
		return nil
	}
	assigned := map[int]bool{}
	for _, pair := range p.pairs {
		if len(pair.candidates) == 0 {
			continue
		}
		c := pair.candidates[rng.IntN(len(pair.candidates))]
		id, ok := pool.Pick(c.fkSchema, c.fkTable, c.fkPkColumn, rng)
		if !ok {
			// Parent pool empty — leave both columns NULL. Factory fallthrough
			// would emit random garbage that can't match an existing parent.
			rowVals[pair.typeIdx] = nil
			rowVals[pair.idIdx] = nil
		} else {
			rowVals[pair.typeIdx] = c.typeName
			rowVals[pair.idIdx] = id
		}
		assigned[pair.typeIdx] = true
		assigned[pair.idIdx] = true
	}
	return assigned
}
