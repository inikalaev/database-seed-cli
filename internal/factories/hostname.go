package factories

import (
	"fmt"

	"github.com/inikalaev/database-seed-cli/pkg/seedapi"
)

type hostnameMech struct{}

func (hostnameMech) Name() string         { return "hostname" }
func (hostnameMech) UniquePerRow() bool   { return true }
func (hostnameMech) Tags() []string { return []string{"hostname", "domain", "subdomain", "host"} }

func (hostnameMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	return textNameMatch(ctx.Column, "hostname", hostnameMech{}.Tags())
}

var hostnameTLDs = []string{"com", "net", "org", "io"}

func (hostnameMech) Generate(ctx seedapi.GenContext) any {
	tld := hostnameTLDs[ctx.Rng.IntN(len(hostnameTLDs))]
	return fmt.Sprintf("tenant-%d.example.%s", ctx.Row, tld)
}
