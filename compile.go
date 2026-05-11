package entcel

import (
	"context"
	"errors"
	"reflect"
	"strings"

	"entgo.io/ent/dialect/sql"
	"github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/common/operators"
	"github.com/google/cel-go/common/types"
	"google.golang.org/protobuf/types/known/structpb"
)

type compiler struct {
	env    *Env
	ctx    context.Context
	filter string
}

type selectorPredicate func(*sql.Selector) error

var errEmptyColumn = errors.New("entcel: column resolver returned empty column")

func (c *compiler) compileFilter(filter string) (selectorPredicate, error) {
	if strings.TrimSpace(filter) == "" {
		if c.env.allowEmptyFilter {
			return func(*sql.Selector) error {
				return nil
			}, nil
		}
		return nil, newError(ErrEmptyFilter, filter, "", "", "empty filter")
	}

	checked, issues := c.env.cel.Compile(filter)
	if issues != nil && issues.Err() != nil {
		if matches := undeclaredReferenceRE.FindStringSubmatch(issues.Err().Error()); len(matches) == 2 {
			return nil, newError(ErrUnknownField, filter, matches[1], "", "field is not declared in schema")
		}
		return nil, wrapError(ErrUnsupportedExpression, filter, "", "", "compile CEL expression", issues.Err())
	}
	if checked == nil || checked.NativeRep() == nil {
		return nil, newError(ErrUnsupportedExpression, filter, "", "", "empty CEL AST")
	}

	return c.compile(checked.NativeRep().Expr())
}

func (c *compiler) compile(expr ast.Expr) (selectorPredicate, error) {
	switch expr.Kind() {
	case ast.CallKind:
		return c.compileCall(expr.AsCall())
	case ast.LiteralKind:
		if value, ok := expr.AsLiteral().Value().(bool); ok {
			if value {
				return func(*sql.Selector) error {
					return nil
				}, nil
			}
			return func(selector *sql.Selector) error {
				selector.Where(sql.False())
				return nil
			}, nil
		}
		return nil, newError(ErrUnsupportedExpression, c.filter, "", "", "literal predicate is unsupported")
	case ast.IdentKind:
		return nil, newError(ErrUnsupportedExpression, c.filter, expr.AsIdent(), "", "standalone identifier predicate is unsupported")
	default:
		return nil, newError(ErrUnsupportedExpression, c.filter, "", "", "unsupported expression kind %v", expr.Kind())
	}
}

func (c *compiler) compileCall(call ast.CallExpr) (selectorPredicate, error) {
	args := call.Args()
	switch call.FunctionName() {
	case "exists":
		return c.compileExistsCall(args)
	case operators.LogicalAnd:
		if len(args) != 2 {
			return nil, newError(ErrUnsupportedExpression, c.filter, "", "", "logical AND requires two operands")
		}
		left, err := c.compile(args[0])
		if err != nil {
			return nil, err
		}
		right, err := c.compile(args[1])
		if err != nil {
			return nil, err
		}
		return andPredicates(left, right), nil
	case operators.LogicalOr:
		if len(args) != 2 {
			return nil, newError(ErrUnsupportedExpression, c.filter, "", "", "logical OR requires two operands")
		}
		left, err := c.compile(args[0])
		if err != nil {
			return nil, err
		}
		right, err := c.compile(args[1])
		if err != nil {
			return nil, err
		}
		return orPredicates(left, right), nil
	case operators.LogicalNot:
		if len(args) != 1 {
			return nil, newError(ErrUnsupportedExpression, c.filter, "", "", "logical NOT requires one operand")
		}
		predicate, err := c.compile(args[0])
		if err != nil {
			return nil, err
		}
		return notPredicate(predicate), nil
	default:
		operator, ok := operatorFromCEL(call.FunctionName())
		if !ok {
			return nil, newError(ErrUnsupportedOperator, c.filter, "", "", "unsupported operator %q", call.FunctionName())
		}
		if call.IsMemberFunction() {
			return c.compileMemberCall(operator, call)
		}
		return c.compileComparison(operator, args)
	}
}

func (c *compiler) compileExistsCall(args []ast.Expr) (selectorPredicate, error) {
	if len(args) != 2 {
		return nil, newError(ErrInvalidRelationFilter, c.filter, "", "", "exists requires relation name and nested filter")
	}
	relationName, ok := stringLiteral(args[0])
	if !ok {
		return nil, newError(ErrInvalidRelationFilter, c.filter, "", "", "exists relation name must be a string literal")
	}
	nestedFilter, ok := stringLiteral(args[1])
	if !ok {
		return nil, newError(ErrInvalidRelationFilter, c.filter, relationName, "", "exists nested filter must be a string literal")
	}
	field, ok := c.env.schema[relationName]
	if !ok || field.relation == nil {
		return nil, newError(ErrUnknownRelation, c.filter, relationName, "", "unknown relation %q", relationName)
	}
	return compileExists(c.ctx, c.filter, relationName, *field.relation, nestedFilter)
}

func (c *compiler) compileComparison(operator Operator, args []ast.Expr) (selectorPredicate, error) {
	if len(args) != 2 {
		return nil, newError(ErrUnsupportedExpression, c.filter, "", operator, "comparison requires two operands")
	}
	if operator == OperatorIn {
		return c.compileIn(args)
	}

	fieldName, literal, reverse, ok := fieldAndLiteral(args[0], args[1])
	if !ok {
		if containsArithmetic(args[0]) {
			return nil, newError(ErrUnsupportedOperator, c.filter, fieldNameFromExpr(args[0]), operator, "arithmetic expressions are unsupported")
		}
		if containsArithmetic(args[1]) && args[0].Kind() != ast.IdentKind {
			return nil, newError(ErrUnsupportedOperator, c.filter, fieldNameFromExpr(args[1]), operator, "arithmetic expressions are unsupported")
		}
		fieldName := fieldNameFromExpr(args[0])
		if fieldName == "" {
			fieldName = fieldNameFromExpr(args[1])
		}
		return nil, newError(ErrInvalidLiteral, c.filter, fieldName, operator, "comparison requires a field and a literal")
	}
	if reverse {
		operator = reverseOperator(operator)
	}

	field, ok := c.env.schema[fieldName]
	if !ok {
		return nil, newError(ErrUnknownField, c.filter, fieldName, operator, "unknown field %q", fieldName)
	}
	if field.predicate == nil && field.column == nil {
		return nil, newError(ErrUnsupportedExpression, c.filter, fieldName, operator, "field has no column resolver or predicate")
	}

	value, err := convertValue(field, literal)
	if err != nil {
		return nil, wrapError(ErrConvertValue, c.filter, fieldName, operator, "convert value", err)
	}

	return func(selector *sql.Selector) error {
		if field.predicate != nil {
			if err := field.predicate(c.ctx, operator, value, selector); err != nil {
				return err
			}
			return nil
		}
		column := ""
		if field.column != nil {
			column = field.column(selector)
		}
		if column == "" {
			return errEmptyColumn
		}
		applyComparison(selector, column, operator, value)
		return nil
	}, nil
}

func (c *compiler) compileIn(args []ast.Expr) (selectorPredicate, error) {
	fieldName, list, ok := fieldAndList(args[0], args[1])
	if !ok {
		return nil, newError(ErrUnsupportedExpression, c.filter, "", OperatorIn, "in requires a field and a literal list")
	}
	field, err := c.lookupField(fieldName, OperatorIn)
	if err != nil {
		return nil, err
	}

	values := make([]any, 0, list.Size())
	for _, element := range list.Elements() {
		if element.Kind() != ast.LiteralKind {
			return nil, newError(ErrInvalidLiteral, c.filter, fieldName, OperatorIn, "in list requires literal values")
		}
		value, err := convertValue(field, element.AsLiteral().Value())
		if err != nil {
			return nil, wrapError(ErrConvertValue, c.filter, fieldName, OperatorIn, "convert value", err)
		}
		values = appendExpanded(values, value)
	}

	return func(selector *sql.Selector) error {
		if field.predicate != nil {
			return field.predicate(c.ctx, OperatorIn, values, selector)
		}
		column := ""
		if field.column != nil {
			column = field.column(selector)
		}
		if column == "" {
			return errEmptyColumn
		}
		sql.FieldIn(column, values...)(selector)
		return nil
	}, nil
}

func (c *compiler) compileMemberCall(operator Operator, call ast.CallExpr) (selectorPredicate, error) {
	if operator != OperatorContains && operator != OperatorStartsWith && operator != OperatorEndsWith {
		return nil, newError(ErrUnsupportedOperator, c.filter, "", operator, "unsupported member operator %q", operator)
	}
	args := call.Args()
	if len(args) != 1 || call.Target().Kind() != ast.IdentKind || args[0].Kind() != ast.LiteralKind {
		return nil, newError(ErrUnsupportedExpression, c.filter, "", operator, "string function requires a field and one literal argument")
	}
	fieldName := call.Target().AsIdent()
	field, err := c.lookupField(fieldName, operator)
	if err != nil {
		return nil, err
	}
	value, err := convertValue(field, args[0].AsLiteral().Value())
	if err != nil {
		return nil, wrapError(ErrConvertValue, c.filter, fieldName, operator, "convert value", err)
	}
	text, ok := value.(string)
	if !ok {
		return nil, newError(ErrInvalidLiteral, c.filter, fieldName, operator, "string function argument must convert to string")
	}

	return func(selector *sql.Selector) error {
		if field.predicate != nil {
			return field.predicate(c.ctx, operator, text, selector)
		}
		column := ""
		if field.column != nil {
			column = field.column(selector)
		}
		if column == "" {
			return errEmptyColumn
		}
		applyStringComparison(selector, column, operator, text)
		return nil
	}, nil
}

func andPredicates(predicates ...selectorPredicate) selectorPredicate {
	return func(selector *sql.Selector) error {
		selector.CollectPredicates()
		for _, predicate := range predicates {
			if err := predicate(selector); err != nil {
				selector.UncollectedPredicates()
				return err
			}
		}
		collected := selector.CollectedPredicates()
		selector.UncollectedPredicates()
		switch len(collected) {
		case 0:
		case 1:
			selector.Where(collected[0])
		default:
			selector.Where(sql.And(collected...))
		}
		return nil
	}
}

func orPredicates(predicates ...selectorPredicate) selectorPredicate {
	return func(selector *sql.Selector) error {
		selector.CollectPredicates()
		for _, predicate := range predicates {
			if err := predicate(selector); err != nil {
				selector.UncollectedPredicates()
				return err
			}
		}
		collected := selector.CollectedPredicates()
		selector.UncollectedPredicates()
		switch len(collected) {
		case 0:
		case 1:
			selector.Where(collected[0])
		default:
			selector.Where(sql.Or(collected...))
		}
		return nil
	}
}

func notPredicate(predicate selectorPredicate) selectorPredicate {
	return func(selector *sql.Selector) error {
		selector.CollectPredicates()
		if err := predicate(selector); err != nil {
			selector.UncollectedPredicates()
			return err
		}
		collected := selector.CollectedPredicates()
		selector.UncollectedPredicates()
		switch len(collected) {
		case 0:
		case 1:
			selector.Where(sql.Not(collected[0]))
		default:
			selector.Where(sql.Not(sql.And(collected...)))
		}
		return nil
	}
}

func fieldAndLiteral(left ast.Expr, right ast.Expr) (string, any, bool, bool) {
	if left.Kind() == ast.IdentKind && right.Kind() == ast.LiteralKind {
		return left.AsIdent(), right.AsLiteral().Value(), false, true
	}
	if left.Kind() == ast.LiteralKind && right.Kind() == ast.IdentKind {
		return right.AsIdent(), left.AsLiteral().Value(), true, true
	}
	return "", nil, false, false
}

func containsArithmetic(expr ast.Expr) bool {
	if expr.Kind() != ast.CallKind {
		return false
	}
	call := expr.AsCall()
	switch call.FunctionName() {
	case operators.Add, operators.Subtract, operators.Multiply, operators.Divide, operators.Modulo, operators.Negate:
		return true
	}
	for _, arg := range call.Args() {
		if containsArithmetic(arg) {
			return true
		}
	}
	if call.IsMemberFunction() {
		return containsArithmetic(call.Target())
	}
	return false
}

func fieldNameFromExpr(expr ast.Expr) string {
	if expr.Kind() == ast.IdentKind {
		return expr.AsIdent()
	}
	if expr.Kind() != ast.CallKind {
		return ""
	}
	call := expr.AsCall()
	if call.IsMemberFunction() {
		if fieldName := fieldNameFromExpr(call.Target()); fieldName != "" {
			return fieldName
		}
	}
	for _, arg := range call.Args() {
		if fieldName := fieldNameFromExpr(arg); fieldName != "" {
			return fieldName
		}
	}
	return ""
}

func fieldAndList(left ast.Expr, right ast.Expr) (string, ast.ListExpr, bool) {
	if left.Kind() == ast.IdentKind && right.Kind() == ast.ListKind {
		return left.AsIdent(), right.AsList(), true
	}
	return "", nil, false
}

func stringLiteral(expr ast.Expr) (string, bool) {
	if expr.Kind() != ast.LiteralKind {
		return "", false
	}
	value, ok := expr.AsLiteral().Value().(string)
	if !ok {
		return "", false
	}
	return value, true
}

func reverseOperator(operator Operator) Operator {
	switch operator {
	case OperatorLT:
		return OperatorGT
	case OperatorLTE:
		return OperatorGTE
	case OperatorGT:
		return OperatorLT
	case OperatorGTE:
		return OperatorLTE
	default:
		return operator
	}
}

func convertValue(field Field, value any) (any, error) {
	if field.convert == nil {
		return value, nil
	}
	return field.convert(value)
}

func (c *compiler) lookupField(fieldName string, operator Operator) (Field, error) {
	field, ok := c.env.schema[fieldName]
	if !ok {
		return Field{}, newError(ErrUnknownField, c.filter, fieldName, operator, "unknown field %q", fieldName)
	}
	if field.predicate == nil && field.column == nil {
		return Field{}, newError(ErrUnsupportedExpression, c.filter, fieldName, operator, "field has no column resolver or predicate")
	}
	return field, nil
}

func appendExpanded(values []any, value any) []any {
	if value == nil {
		return append(values, value)
	}
	reflected := reflect.ValueOf(value)
	if reflected.Kind() != reflect.Slice && reflected.Kind() != reflect.Array {
		return append(values, value)
	}
	for i := 0; i < reflected.Len(); i++ {
		values = append(values, reflected.Index(i).Interface())
	}
	return values
}

func applyComparison(selector *sql.Selector, column string, operator Operator, value any) {
	switch operator {
	case OperatorEQ:
		if isNullValue(value) {
			sql.FieldIsNull(column)(selector)
			return
		}
		if _, ok := value.(bool); ok {
			applyFieldOp(selector, column, sql.OpEQ, value)
			return
		}
		sql.FieldEQ(column, value)(selector)
	case OperatorNEQ:
		if isNullValue(value) {
			sql.FieldNotNull(column)(selector)
			return
		}
		if _, ok := value.(bool); ok {
			applyFieldOp(selector, column, sql.OpNEQ, value)
			return
		}
		sql.FieldNEQ(column, value)(selector)
	case OperatorLT:
		sql.FieldLT(column, value)(selector)
	case OperatorLTE:
		sql.FieldLTE(column, value)(selector)
	case OperatorGT:
		sql.FieldGT(column, value)(selector)
	case OperatorGTE:
		sql.FieldGTE(column, value)(selector)
	default:
		return
	}
}

func isNullValue(value any) bool {
	switch value := value.(type) {
	case types.Null:
		return value == types.NullValue
	case structpb.NullValue:
		return value == structpb.NullValue_NULL_VALUE
	default:
		return value == nil
	}
}

func applyStringComparison(selector *sql.Selector, column string, operator Operator, value string) {
	switch operator {
	case OperatorContains:
		sql.FieldContains(column, value)(selector)
	case OperatorStartsWith:
		sql.FieldHasPrefix(column, value)(selector)
	case OperatorEndsWith:
		sql.FieldHasSuffix(column, value)(selector)
	}
}

func applyFieldOp(selector *sql.Selector, column string, operator sql.Op, value any) {
	selector.Where(sql.P(func(builder *sql.Builder) {
		builder.Ident(selector.C(column))
		builder.WriteOp(operator)
		builder.Arg(value)
	}))
}
