package factories

import "github.com/inikalaev/database-seed-cli/pkg/seedapi"

type mimeTypeMech struct{}

func (mimeTypeMech) Name() string   { return "mime_type" }
func (mimeTypeMech) Tags() []string { return []string{"content_type", "mime_type", "media_type"} }

func (mimeTypeMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	return textNameMatch(ctx.Column, "mime_type", mimeTypeMech{}.Tags())
}

var mimeTypes = []string{
	"image/png", "image/jpeg", "image/gif", "image/webp",
	"application/pdf", "application/zip",
	"video/mp4", "audio/mpeg",
	"text/plain", "text/csv",
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
}

func (mimeTypeMech) Generate(ctx seedapi.GenContext) any {
	return mimeTypes[ctx.Rng.IntN(len(mimeTypes))]
}
