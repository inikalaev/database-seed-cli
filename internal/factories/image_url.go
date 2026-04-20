package factories

import (
	"fmt"

	"github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"
)

type imageURLMech struct{}

func (imageURLMech) Name() string { return "image_url" }
func (imageURLMech) Tags() []string {
	return []string{"image", "logo", "avatar", "icon", "photo", "logotype", "preview"}
}

func (imageURLMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	return textNameMatch(ctx.Column, "image_url", imageURLMech{}.Tags())
}

func (imageURLMech) Generate(ctx seedapi.GenContext) any {
	w := ctx.Params.Int("width", 200)
	h := ctx.Params.Int("height", 200)
	return fmt.Sprintf("https://picsum.photos/seed/%d/%d/%d", ctx.Row, w, h)
}
