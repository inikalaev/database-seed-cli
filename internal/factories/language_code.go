package factories

import "github.com/inikalaev/database-seed-cli/pkg/seedapi"

type languageCodeMech struct{}

func (languageCodeMech) Name() string   { return "language_code" }
func (languageCodeMech) Tags() []string { return []string{"language", "locale", "lang"} }

func (languageCodeMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	return textNameMatch(ctx.Column, "language_code", languageCodeMech{}.Tags())
}

var languageCodes = []string{
	"en", "ru", "zh", "de", "fr", "es", "pt", "ar", "hi", "ja", "ko", "tr", "it", "pl",
}

func (languageCodeMech) Generate(ctx seedapi.GenContext) any {
	return languageCodes[ctx.Rng.IntN(len(languageCodes))]
}
