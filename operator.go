package entcel

import (
	"github.com/google/cel-go/common/operators"
	"github.com/google/cel-go/common/overloads"
)

type Operator string

const (
	OperatorEQ         Operator = "eq"
	OperatorNEQ        Operator = "neq"
	OperatorLT         Operator = "lt"
	OperatorLTE        Operator = "lte"
	OperatorGT         Operator = "gt"
	OperatorGTE        Operator = "gte"
	OperatorIn         Operator = "in"
	OperatorContains   Operator = "contains"
	OperatorStartsWith Operator = "startsWith"
	OperatorEndsWith   Operator = "endsWith"
)

func operatorFromCEL(name string) (Operator, bool) {
	switch name {
	case operators.Equals:
		return OperatorEQ, true
	case operators.NotEquals:
		return OperatorNEQ, true
	case operators.Less:
		return OperatorLT, true
	case operators.LessEquals:
		return OperatorLTE, true
	case operators.Greater:
		return OperatorGT, true
	case operators.GreaterEquals:
		return OperatorGTE, true
	case operators.In, operators.OldIn:
		return OperatorIn, true
	case overloads.Contains, overloads.ContainsString:
		return OperatorContains, true
	case overloads.StartsWith, overloads.StartsWithString:
		return OperatorStartsWith, true
	case overloads.EndsWith, overloads.EndsWithString:
		return OperatorEndsWith, true
	default:
		return "", false
	}
}
