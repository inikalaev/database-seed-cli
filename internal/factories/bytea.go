package factories

import "github.com/inikalaev/database-seed-cli/pkg/seedapi"

// byteaMech emits random binary payloads for `bytea` columns. sqlemit formats
// []byte as PG hex-escape (`'\x…'`), which is unambiguous regardless of the
// session's standard_conforming_strings setting.
type byteaMech struct{}

func (byteaMech) Name() string   { return "bytea" }
func (byteaMech) Tags() []string { return nil }

func (byteaMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if isBytea(ctx.Column) {
		return seedapi.StrongMatch
	}
	return seedapi.NoMatch
}

func (byteaMech) Generate(ctx seedapi.GenContext) any {
	// 16 bytes is enough to exercise hex encoding without bloating fixtures;
	// users who need different sizes can override via params/custom factory.
	n := 16
	if v, ok := ctx.Params["size"].(int); ok && v > 0 {
		n = v
	}
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(ctx.Rng.UintN(256))
	}
	return b
}
