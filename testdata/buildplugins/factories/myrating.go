package main

import "github.com/inikalaev/database-seed-cli/pkg/seedapi"

type myRatingFactory struct{}

func (myRatingFactory) Name() string   { return "my_rating" }
func (myRatingFactory) Tags() []string { return []string{"rating"} }

func (myRatingFactory) Generate(ctx seedapi.GenContext) any {
	return ctx.Rng.IntN(5) + 1
}

func init() {
	seedapi.Register(myRatingFactory{})
}
