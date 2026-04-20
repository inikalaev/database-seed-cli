package factories

import "github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"

type statusCodeMech struct{}

func (statusCodeMech) Name() string   { return "status_code" }
func (statusCodeMech) Tags() []string { return []string{"status_code", "http_code", "http_status"} }

func (statusCodeMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if !isInt(ctx.Column) {
		return seedapi.NoMatch
	}
	if nameMatches(ctx.Column, `(^|_)status_code(_|$)|(^|_)http_(code|status)(_|$)`) {
		return seedapi.NameMatch
	}
	return seedapi.NoMatch
}

var httpStatusCodes = []int{200, 201, 204, 301, 302, 400, 401, 403, 404, 409, 422, 500, 502, 503}

func (statusCodeMech) Generate(ctx seedapi.GenContext) any {
	return httpStatusCodes[ctx.Rng.IntN(len(httpStatusCodes))]
}
