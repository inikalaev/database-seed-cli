package factories

import "github.com/inikalaev/database-seed-cli/pkg/seedapi"

type currencyMech struct{}

func (currencyMech) Name() string   { return "currency" }
func (currencyMech) Tags() []string { return []string{"currency", "currency_code"} }

func (currencyMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	return textNameMatch(ctx.Column, "currency", currencyMech{}.Tags())
}

var currencies = []string{
	"USD", "EUR", "RUB", "GBP", "JPY", "CNY", "CHF", "CAD", "AUD", "KZT",
}

func (currencyMech) Generate(ctx seedapi.GenContext) any {
	return currencies[ctx.Rng.IntN(len(currencies))]
}
