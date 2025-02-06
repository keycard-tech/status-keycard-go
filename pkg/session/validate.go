package session

import (
	goerrors "errors"

	"github.com/go-playground/validator/v10"
	"github.com/tyler-smith/go-bip39"
)

var (
	validate = validator.New()
)

func init() {
	// Register the custom validation function
	err := validate.RegisterValidation("mnemonic", isMnemonic)
	if err != nil {
		panic(err)
	}
}

func validateRequest(v interface{}) error {
	err := validate.Struct(v)
	if err != nil {
		errs := err.(validator.ValidationErrors)
		return goerrors.Join(errs)
	}
	return nil
}

// Custom validation function to check if a string is a list of space-separated words
func isMnemonic(fl validator.FieldLevel) bool {
	mnemonic := fl.Field().String()
	return bip39.IsMnemonicValid(mnemonic)
}
