// Package factories defines the builtin library of fill strategies.
//
// Layout: one factory per file. Shared helpers (name/type predicates) live in
// helpers.go. Registration order in All() is the inference tie-break order:
// earlier entries win when two factories return the same MatchScore.
package factories

import "github.com/inikalaev/database-seed-cli/pkg/seedapi"

// All returns every builtin factory in priority-friendly order. FK wins over
// everything; name-driven matches come before type-driven fallbacks.
func All() []seedapi.Factory {
	return []seedapi.Factory{
		fkRef{},
		firstName{}, lastName{}, fullName{}, patronymicMech{},
		emailMech{}, phoneMech{}, urlMech{}, imageURLMech{},
		companyMech{}, cityMech{}, countryMech{}, hostnameMech{},
		slugMech{}, colorMech{}, titleMech{}, genderMech{},
		tokenMech{}, ipAddressMech{}, filenameMech{}, mimeTypeMech{},
		usernameMech{}, currencyMech{}, languageCodeMech{},
		enumValue{}, enumValueStr{}, pkSerial{}, uuidMech{},
		positionMech{}, versionIntMech{}, levelMech{}, yearMech{}, priorityMech{},
		percentageMech{}, durationMech{}, fileSizeMech{}, counterMech{}, amountMech{},
		portMech{}, statusCodeMech{},
		checksumMech{}, versionStrMech{},
		latitudeMech{}, longitudeMech{},
		intMech{}, decimalMech{}, boolMech{}, byteaMech{}, arrayMech{}, timestampMech{}, dateMech{},
		timestampStrMech{},
		pointMech{}, timeOfDayMech{}, pgIntervalMech{}, tstzrangeMech{},
		hstoreMech{}, localizedJSON{}, textMech{}, jsonAny{},
	}
}
