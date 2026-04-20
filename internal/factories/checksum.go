package factories

import (
	"fmt"

	"github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"
)

type checksumMech struct{}

func (checksumMech) Name() string   { return "checksum" }
func (checksumMech) Tags() []string { return []string{"checksum", "digest", "hash", "fingerprint"} }

func (checksumMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	return textNameMatch(ctx.Column, "checksum", checksumMech{}.Tags())
}

func (checksumMech) Generate(ctx seedapi.GenContext) any {
	var b [20]byte // SHA-1 length
	for i := range b {
		b[i] = byte(ctx.Rng.IntN(256))
	}
	return fmt.Sprintf("%x", b[:])
}
