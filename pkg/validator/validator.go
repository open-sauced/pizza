// package validator provides the necessary utilities
// to validate data before invoking database queries
package validator

import (
	"net/http"
	"strings"
)

// Validator: type which contains a map of validation errors (error name : string -> error_description : string)
type Validator struct {
	Errors map[string]string
}

// New: return an instance of a validator
func New() *Validator {
	return &Validator{Errors: make(map[string]string)}
}

// Valid: returns true if there are no errors, otherwise false
func (v *Validator) Valid() bool {
	return len(v.Errors) == 0
}

// AddError: add a new error to the validator
func (v *Validator) AddError(key, message string) {
	if _, exists := v.Errors[key]; !exists {
		v.Errors[key] = message
	}
}

// CheckConstraint: Receives a constraint that evaluates to a boolean expression to validate
// false -> add error
// true -> skip
func (v *Validator) CheckConstraint(ok bool, key, message string) {
	if !ok {
		v.AddError(key, message)
	}
}

func ValidateURL(validator *Validator, url string) {
	validator.CheckConstraint(url != "", "url", "URL must be provided")
	validator.CheckConstraint(strings.HasPrefix(url, "https://github.com/"), "url", "The URL provided is not a valid repository")
	validator.CheckConstraint(checkURLValid(url), "url", "The URL provided does not exists")
}

func checkURLValid(url string) bool {
	_, err := http.Get(url)
	if err != nil {
		return false
	}
	return true
}
