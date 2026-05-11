package entcel

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCompileUnknownFieldErrorIncludesContext(t *testing.T) {
	env, err := NewEnv(Schema{
		"id": Int(Column("id")),
	})
	require.NoError(t, err)

	_, err = env.Compile(context.Background(), `missing == 1`)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrUnknownField)

	var entErr *Error
	require.ErrorAs(t, err, &entErr)
	require.Equal(t, "missing", entErr.Field)
	require.Empty(t, entErr.Operator)
	require.Equal(t, `missing == 1`, entErr.Filter)
}

func TestCompileUnknownFieldDoesNotGuessOperatorFromFilterText(t *testing.T) {
	env, err := NewEnv(Schema{
		"id": Int(Column("id")),
	})
	require.NoError(t, err)

	_, err = env.Compile(context.Background(), `missing > "=="`)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrUnknownField)

	var entErr *Error
	require.ErrorAs(t, err, &entErr)
	require.Equal(t, "missing", entErr.Field)
	require.Empty(t, entErr.Operator)
	require.Equal(t, `missing > "=="`, entErr.Filter)
}

func TestCompileUnsupportedOperatorErrorIncludesContext(t *testing.T) {
	env, err := NewEnv(Schema{
		"id": Int(Column("id")),
	})
	require.NoError(t, err)

	_, err = env.Compile(context.Background(), `id + 1 == 2`)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrUnsupportedOperator)
}

func TestCompileInvalidLiteralErrorIncludesContext(t *testing.T) {
	env, err := NewEnv(Schema{
		"id": Int(Column("id")),
	})
	require.NoError(t, err)

	_, err = env.Compile(context.Background(), `id == (1 + 1)`)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidLiteral)
}
