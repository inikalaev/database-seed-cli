package factories

import (
	"fmt"
	"strings"

	"github.com/inikalaev/database-seed-cli/pkg/seedapi"
)

type ipAddressMech struct{}

func (ipAddressMech) Name() string   { return "ip_address" }
func (ipAddressMech) Tags() []string { return []string{"ip", "ip_address", "ipaddr"} }

func (ipAddressMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if strings.EqualFold(ctx.Column.UDTName, "inet") {
		return seedapi.TypeMatch
	}
	return textNameMatch(ctx.Column, "ip_address", ipAddressMech{}.Tags())
}

func (ipAddressMech) Generate(ctx seedapi.GenContext) any {
	return fmt.Sprintf("10.%d.%d.%d",
		ctx.Rng.IntN(256),
		ctx.Rng.IntN(256),
		1+ctx.Rng.IntN(254),
	)
}
