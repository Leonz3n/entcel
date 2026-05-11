package entcel

import (
	"context"

	"entgo.io/ent/dialect/sql"
	"github.com/google/cel-go/common/types"
)

// Schema declares the CEL-visible fields and relations that may be compiled.
type Schema map[string]Field

// ColumnResolver resolves a field to an entgo SQL column expression for a selector.
type ColumnResolver func(*sql.Selector) string

// Converter converts a CEL literal into the value used by the SQL predicate.
type Converter func(any) (any, error)

// PredicateBuilder adds a custom SQL predicate for a field and operator.
//
// Predicate builders should be deterministic and limited to the current
// selector, with no remote calls or repository lookups.
type PredicateBuilder func(context.Context, Operator, any, *sql.Selector) error

// Field describes a queryable CEL field or relation.
type Field struct {
	celType   *types.Type
	column    ColumnResolver
	convert   Converter
	predicate PredicateBuilder
	relation  *RelationConfig
}

// FieldOption configures a Field constructor.
type FieldOption func(*Field)

// Bool declares a boolean field.
func Bool(options ...FieldOption) Field {
	return newField(types.BoolType, options...)
}

// Bytes declares a bytes field.
func Bytes(options ...FieldOption) Field {
	return newField(types.BytesType, options...)
}

// Double declares a double-precision numeric field.
func Double(options ...FieldOption) Field {
	return newField(types.DoubleType, options...)
}

// Int declares an integer field.
func Int(options ...FieldOption) Field {
	return newField(types.IntType, options...)
}

// String declares a string field.
func String(options ...FieldOption) Field {
	return newField(types.StringType, options...)
}

// Time declares a timestamp field.
func Time(options ...FieldOption) Field {
	return newScalarField(types.TimestampType, options...)
}

// Any declares a dynamically typed field.
func Any(options ...FieldOption) Field {
	return newField(types.DynType, options...)
}

// Column maps a field to a selector-qualified SQL column name.
func Column(name string) FieldOption {
	return ColumnExpr(func(selector *sql.Selector) string {
		return selector.C(name)
	})
}

// ColumnExpr maps a field to a custom SQL column expression.
func ColumnExpr(resolver ColumnResolver) FieldOption {
	return func(field *Field) {
		field.column = resolver
	}
}

// Convert configures a field literal converter.
func Convert(converter Converter) FieldOption {
	return func(field *Field) {
		field.convert = converter
	}
}

// Predicate configures a custom SQL predicate builder for a field.
func Predicate(builder PredicateBuilder) FieldOption {
	return func(field *Field) {
		field.predicate = builder
	}
}

func newField(celType *types.Type, options ...FieldOption) Field {
	field := Field{
		celType: celType,
	}
	for _, option := range options {
		option(&field)
	}
	return field
}

func newScalarField(celType *types.Type, options ...FieldOption) Field {
	return newField(celType, options...)
}
