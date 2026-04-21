package factories

import (
	"fmt"

	"github.com/inikalaev/database-seed-cli/pkg/seedapi"
)

type filenameMech struct{}

func (filenameMech) Name() string   { return "filename" }
func (filenameMech) Tags() []string { return []string{"file_name", "filename"} }

func (filenameMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if !isText(ctx.Column) {
		return seedapi.NoMatch
	}
	if nameMatches(ctx.Column, `file.?name`) {
		return seedapi.NameMatch
	}
	return seedapi.NoMatch
}

var fileExts = []string{"pdf", "docx", "xlsx", "png", "jpg", "zip", "csv", "mp4"}

func (filenameMech) Generate(ctx seedapi.GenContext) any {
	ext := fileExts[ctx.Rng.IntN(len(fileExts))]
	return fmt.Sprintf("file_%d.%s", ctx.Row, ext)
}
