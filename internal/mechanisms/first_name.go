package mechanisms

import "github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"

// firstName picks a given name from a small pool. Matches text columns named
// first_name / given_name / fname.
type firstName struct{}

func (firstName) Name() string   { return "first_name" }
func (firstName) Tags() []string { return []string{"person", "name"} }

func (firstName) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if !isText(ctx.Column) {
		return seedapi.NoMatch
	}
	if nameMatches(ctx.Column, `^first_?name$`, `^given_?name$`, `^fname$`) {
		return seedapi.StrongMatch
	}
	return seedapi.NoMatch
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
