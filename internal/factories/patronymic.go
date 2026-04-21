package factories

import "github.com/inikalaev/database-seed-cli/pkg/seedapi"

type patronymicMech struct{}

func (patronymicMech) Name() string   { return "patronymic" }
func (patronymicMech) Tags() []string { return []string{"patronymic", "middle_name", "otchestvo"} }

func (patronymicMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	return textNameMatch(ctx.Column, "patronymic", patronymicMech{}.Tags())
}

func (patronymicMech) Generate(ctx seedapi.GenContext) any {
	pool := []string{
		// Russian
		"Aleksandrovich", "Alekseyevich", "Vladimirovich", "Dmitrievich", "Ivanovich",
		"Mikhailovich", "Nikolayevich", "Pavlovich", "Petrovich", "Sergeyevich",
		"Aleksandrovna", "Alekseyevna", "Vladimirovna", "Dmitrievna", "Ivanovna",
		"Mikhailovna", "Nikolayevna", "Pavlovna", "Petrovna", "Sergeyevna",
		// Kazakh
		"Abenuly", "Nurlanuly", "Askaruly", "Berikuly", "Dauletkhanuly",
		"Abenkyzy", "Nurlankyzy", "Askarkyzy", "Berikkyzy", "Dauletkhankyzy",
		// Azerbaijani
		"Aliyev oglu", "Mammadov oglu", "Hasanov oglu", "Huseynov oglu",
		"Aliyeva qizi", "Mammadova qizi", "Hasanova qizi", "Huseynova qizi",
		// Georgian
		"Beridze-shvili", "Kvaratskhelia-shvili", "Chikvanaia-shvili",
		// Armenian
		"Hakobyan", "Sargsyan", "Petrosyan", "Grigoryan",
		// Uzbek
		"Karimov ogli", "Rahimov ogli", "Toshmatov ogli",
		"Karimova qizi", "Rahimova qizi", "Toshmatova qizi",
		// Tajik
		"Rahmonzoda", "Nazarov-zoda", "Karimzoda",
	}
	return pool[ctx.Rng.IntN(len(pool))]
}
