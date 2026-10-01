package rules

import (
	"errors"
	"fmt"

	"github.com/Olian04/webui/internal/ir"
)

var (
	ErrInvalidValueType = errors.New("invalid value type")
	ErrRequired         = errors.New("value is required")
	ErrMinimumLength    = errors.New("value is too short")
	ErrMaximumLength    = errors.New("value is too long")
	ErrMinimum          = errors.New("value is too small")
	ErrMaximum          = errors.New("value is too large")
	ErrPattern          = errors.New("value does not match pattern")
)

func Validate(rules ir.Rules, value any) error {
	switch value := value.(type) {
	case nil:
		if rules.Required {
			return fmt.Errorf("%w: %T", ErrRequired, value)
		}
		return nil
	case string:
		return validateString(rules, value)
	case int:
		return validateNumber(rules, float64(value))
	case float64:
		return validateNumber(rules, value)
	default:
		return fmt.Errorf("%w: %T", ErrInvalidValueType, value)
	}
}

func validateString(rules ir.Rules, value string) error {
	if rules.MinLen > 0 && len(value) < rules.MinLen {
		return fmt.Errorf("%w: %d", ErrMinimumLength, rules.MinLen)
	}

	if rules.MaxLen > 0 && len(value) > rules.MaxLen {
		return fmt.Errorf("%w: %d", ErrMaximumLength, rules.MaxLen)
	}

	if rules.Pattern != nil && !rules.Pattern.Expr.MatchString(value) {
		return fmt.Errorf("%w: %s", ErrPattern, rules.Pattern.Expr)
	}

	return nil
}

func validateNumber(rules ir.Rules, value float64) error {
	if rules.Min != nil && value < *rules.Min {
		return fmt.Errorf("%w: %f", ErrMinimum, *rules.Min)
	}

	if rules.Max != nil && value > *rules.Max {
		return fmt.Errorf("%w: %f", ErrMaximum, *rules.Max)
	}

	return nil
}
