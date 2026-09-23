// Package validate is the repository's constructor-parameter validation
// helper (E08 review). It wraps go-playground/validator with one joined
// error format so every adapter reports every rule violation at once:
//
//	<scope>: invalid <label> (N violation(s)): Field: rule "tag" on value v; ...
//
// Usage rule (docs/development/go-conventions.md): data fields declare
// `validate` tags and go through Struct; dependency seams (interfaces)
// cannot be expressed as tags and stay explicit sentinel error checks in
// the constructor.
package validate

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

// Struct validates params and joins every tag violation into one error.
// It returns nil when params is valid. Non-struct input is an error: every
// current caller passes a Parameter Object, and silently accepting anything
// else would hide wiring bugs.
func Struct(scope, label string, params any) error {
	v := validator.New()

	if err := v.Struct(params); err != nil {
		var verrs validator.ValidationErrors
		if !errors.As(err, &verrs) {
			return fmt.Errorf("%s: validate %s: %w", scope, label, err)
		}

		parts := make([]string, 0, len(verrs))
		for _, fe := range verrs {
			parts = append(parts, fmt.Sprintf("%s: rule %q on value %v", fe.Field(), fe.Tag(), fe.Value()))
		}

		return fmt.Errorf("%s: invalid %s (%d violation(s)): %s",
			scope, label, len(parts), strings.Join(parts, "; "))
	}

	return nil
}
