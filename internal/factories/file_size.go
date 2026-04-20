package factories

import (
	"strings"

	"github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"
)

type fileSizeMech struct{}

func (fileSizeMech) Name() string   { return "file_size" }
func (fileSizeMech) Tags() []string { return []string{"file_size", "filesize"} }

func (fileSizeMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if !isInt(ctx.Column) && strings.ToLower(ctx.Column.DataType) != "bigint" {
		return seedapi.NoMatch
	}
	if nameMatches(ctx.Column, `file.?size|byte.?size`) {
		return seedapi.NameMatch
	}
	return seedapi.NoMatch
}

// Generate returns a file size in bytes in [1 KB, 50 MB].
func (fileSizeMech) Generate(ctx seedapi.GenContext) any {
	lo := ctx.Params.Int("min", 1_024)
	hi := ctx.Params.Int("max", 50*1_024*1_024)
	return inclusiveIntN(ctx.Rng, lo, hi)
}
