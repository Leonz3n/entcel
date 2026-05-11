package entcel

import (
	"errors"
	"fmt"
)

var (
	ErrEmptyFilter           = errors.New("empty filter")
	ErrUnknownField          = errors.New("unknown field")
	ErrUnsupportedOperator   = errors.New("unsupported operator")
	ErrUnsupportedExpression = errors.New("unsupported expression")
	ErrInvalidLiteral        = errors.New("invalid literal")
	ErrUnknownRelation       = errors.New("unknown relation")
	ErrInvalidRelationFilter = errors.New("invalid relation filter")
	ErrConvertValue          = errors.New("convert value")
)

type Error struct {
	Kind     error
	Filter   string
	Field    string
	Operator Operator
	Message  string
	Err      error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Kind != nil {
		return e.Kind.Error()
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return "entcel error"
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	if e.Err != nil {
		return errors.Join(e.Kind, e.Err)
	}
	return e.Kind
}

func wrapError(kind error, filter string, field string, operator Operator, message string, err error) error {
	return &Error{
		Kind:     kind,
		Filter:   filter,
		Field:    field,
		Operator: operator,
		Message:  message,
		Err:      err,
	}
}

func newError(kind error, filter string, field string, operator Operator, format string, args ...any) error {
	message := ""
	if format != "" {
		message = fmt.Sprintf(format, args...)
	}
	return wrapError(kind, filter, field, operator, message, nil)
}
