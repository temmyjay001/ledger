// pkg/validator/validator.go
package validator

import (
	"sync"

	"github.com/go-playground/validator/v10"
	dv "github.com/sblackstone/shopspring-decimal-validators"
)

var (
	validate *validator.Validate
	once     sync.Once
)

// GetValidator returns a singleton validator instance with all custom rules registered
func GetValidator() *validator.Validate {
	once.Do(func() {
		v := validator.New()

		// Register shopspring decimal validations
		dv.RegisterDecimalValidators(v)

		// 👉 You can register more custom validations here if needed later
		// v.RegisterValidation("custom_tag", customFunc)

		validate = v
	})
	return validate
}
