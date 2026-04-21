package factories

import "github.com/inikalaev/database-seed-cli/pkg/seedapi"

type firstName struct{}

func (firstName) Name() string   { return "first_name" }
func (firstName) Tags() []string { return []string{"given_name", "fname"} }

// Match gates name-based auto-matching on text types. Without this, a column
// literally named "first_name" on an integer column still scored StrongMatch
// and produced quoted strings into an int field.
func (firstName) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	return textNameMatch(ctx.Column, "first_name", firstName{}.Tags())
}

func (firstName) Generate(ctx seedapi.GenContext) any {
	pool := []string{
		// English
		"James", "Emma", "Oliver", "Sophia", "Liam", "Ava", "Noah", "Isabella",
		// Spanish / Latin American
		"Santiago", "Valentina", "Mateo", "Camila", "Sebastián", "Sofía",
		// French
		"Lucas", "Léa", "Hugo", "Chloé", "Théo", "Manon",
		// German
		"Leon", "Mia", "Paul", "Hannah", "Felix", "Laura",
		// Indian
		"Aarav", "Priya", "Arjun", "Ananya", "Rohan", "Kavya",
		// Arabic
		"Omar", "Fatima", "Ali", "Layla", "Yusuf", "Noor",
		// Chinese
		"Wei", "Fang", "Jian", "Xia", "Ming", "Ying",
		// Japanese
		"Haruto", "Yui", "Sota", "Hina", "Ren", "Sakura",
		// East African
		"Amara", "Kofi", "Zara", "Kwame", "Nia", "Seun",
		// Russian / Eastern European
		"Ivan", "Anna", "Dmitry", "Elena", "Nikita", "Natasha",
	}
	return pool[ctx.Rng.IntN(len(pool))]
}
