// Package mechanisms defines the builtin library of fill strategies.
//
// Layout: one mechanism per file. Shared helpers (name/type predicates) live in
// helpers.go. Registration order in All() is the inference tie-break order:
// earlier entries win when two mechanisms return the same MatchScore.
package mechanisms

import "github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"

// All returns every builtin mechanism in priority-friendly order. FK wins over
// everything; name-driven matches come before type-driven fallbacks.
func All() []seedapi.Mechanism {
	return []seedapi.Mechanism{
		fkRef{},
		firstName{}, lastName{}, fullName{}, emailMech{}, phoneMech{},
		urlMech{}, companyMech{}, cityMech{}, countryMech{},
		enumValue{}, pkSerial{}, uuidMech{},
		intMech{}, decimalMech{}, boolMech{}, timestampMech{}, dateMech{},
		textMech{}, jsonAny{},
	}
}
