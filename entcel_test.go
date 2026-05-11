package entcel

import (
	"context"
	"errors"
	"testing"

	"entgo.io/ent/dialect/sql"
	"github.com/google/cel-go/common/types"
	"github.com/stretchr/testify/require"
)

func TestCompileEmptyFilterReturnsErrorByDefault(t *testing.T) {
	env, err := NewEnv(Schema{"id": Int(Column("id"))})
	require.NoError(t, err)

	_, err = env.Compile(context.Background(), "")

	require.Error(t, err)
	require.True(t, errors.Is(err, ErrEmptyFilter))
}

func TestCompileEmptyFilterCanBeNoop(t *testing.T) {
	env, err := NewEnv(Schema{"id": Int(Column("id"))}, AllowEmptyFilter())
	require.NoError(t, err)

	predicate, err := env.Compile(context.Background(), "")
	require.NoError(t, err)

	selector := sql.Select().From(sql.Table("users"))
	predicate(selector)
	query, args := selector.Query()

	require.Equal(t, "SELECT * FROM `users`", query)
	require.Empty(t, args)
}

func TestPredicateBuilderUsesApprovedSignature(t *testing.T) {
	var builder PredicateBuilder = func(context.Context, Operator, any, *sql.Selector) error {
		return nil
	}

	field := String(Predicate(builder))

	require.NotNil(t, field.predicate)
}

func TestTimeOnlyDeclaresTimestampType(t *testing.T) {
	field := Time()

	require.Equal(t, types.TimestampType, field.celType)
	require.Nil(t, field.convert)
}
