package factories

import "github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"

type lastName struct{}

func (lastName) Name() string   { return "last_name" }
func (lastName) Tags() []string { return []string{"family_name", "surname", "lname"} }

func (lastName) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	return textNameMatch(ctx.Column, "last_name", lastName{}.Tags())
}

func (lastName) Generate(ctx seedapi.GenContext) any {
	pool := []string{
		// English
		"Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller", "Davis",
		// Spanish / Latin American
		"González", "Rodríguez", "López", "Martínez", "Hernández", "Pérez",
		// French
		"Martin", "Bernard", "Dubois", "Thomas", "Robert", "Richard",
		// German
		"Müller", "Schmidt", "Schneider", "Fischer", "Weber", "Meyer",
		// Indian
		"Sharma", "Patel", "Singh", "Kumar", "Gupta", "Joshi",
		// Arabic
		"Al-Farsi", "Hassan", "Ibrahim", "Khalil", "Mansour", "Qureshi",
		// Chinese
		"Wang", "Li", "Zhang", "Liu", "Chen", "Yang",
		// Japanese
		"Sato", "Suzuki", "Tanaka", "Watanabe", "Ito", "Yamamoto",
		// East African
		"Osei", "Mensah", "Diallo", "Nkosi", "Banda", "Okonkwo",
		// Russian / Eastern European
		"Ivanov", "Petrov", "Smirnov", "Kuznetsov", "Novak", "Kowalski",
	}
	return pool[ctx.Rng.IntN(len(pool))]
}
