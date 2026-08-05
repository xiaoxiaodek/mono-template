package validator

import playvalidator "github.com/go-playground/validator/v10"

func New() *playvalidator.Validate {
	return playvalidator.New(playvalidator.WithRequiredStructEnabled())
}
