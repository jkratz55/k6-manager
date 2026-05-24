package internal

import (
	"errors"

	validation "github.com/go-ozzo/ozzo-validation/v4"
)

type Validator struct{}

func (v Validator) Validate(i any) error {
	if validatable, ok := i.(validation.Validatable); ok {
		return validatable.Validate()
	}
	return nil
}

func MapValidationErrors(err error) map[string][]string {
	if err == nil {
		return nil
	}

	ozzoErrors, ok := errors.AsType[validation.Errors](err)
	if !ok {
		return map[string][]string{
			"error": {err.Error()},
		}
	}

	result := make(map[string][]string, len(ozzoErrors))
	for field, fieldErr := range ozzoErrors {
		if errs, ok := errors.AsType[validation.Errors](fieldErr); ok {
			nested := MapValidationErrors(errs)
			for nestedField, nestedMsgs := range nested {
				result[field+"."+nestedField] = nestedMsgs
			}
		} else {
			result[field] = []string{fieldErr.Error()}
		}
	}
	return result
}
