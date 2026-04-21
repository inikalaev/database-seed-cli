package factories

import (
	"fmt"

	"github.com/inikalaev/database-seed-cli/pkg/seedapi"
)

// uuidMech emits RFC 4122 v4 UUIDs. Matches any column whose declared type is
// uuid, regardless of name.
type uuidMech struct{}

func (uuidMech) Name() string { return "uuid" }

// Tags is informational only — uuidMech implements Match() directly, so
// autoMatch never consults Tags(). UUID columns are matched by data type alone.
func (uuidMech) Tags() []string { return nil }

func (uuidMech) UniquePerRow() bool { return true }

func (uuidMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if isUUID(ctx.Column) {
		return seedapi.StrongMatch
	}
	return seedapi.NoMatch
}

func (uuidMech) Generate(ctx seedapi.GenContext) any {
	b := make([]byte, 16)
	for i := range b {
		b[i] = byte(ctx.Rng.UintN(256))
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
