package seedgens

import (
	"fmt"
	"regexp"

	"github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"
)

type SKU struct{}

func (SKU) Name() string   { return "sku" }
func (SKU) Tags() []string { return []string{"article", "product_code"} }

// Match implements seedapi.Matcher for regex-based column matching.
// Without this, the registry auto-matches via Name() and Tags() above.
func (SKU) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if ok, _ := regexp.MatchString(`(?i)^sku$|article|product_code`, ctx.Column.Name); ok {
		return seedapi.StrongMatch
	}
	return seedapi.NoMatch
}

func (SKU) Generate(ctx seedapi.GenContext) any {
	return fmt.Sprintf("SKU-%06d", ctx.Row+1)
}

func init() { seedapi.Register(SKU{}) }
