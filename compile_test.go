package entcel

import (
	"context"
	"errors"
	"testing"

	"entgo.io/ent/dialect/sql"
	"github.com/google/cel-go/common/operators"
	"github.com/google/cel-go/common/overloads"
	"github.com/stretchr/testify/require"
)

func compileSQL(t *testing.T, env *Env, filter string) (string, []any) {
	t.Helper()

	predicate, err := env.Compile(context.Background(), filter)
	require.NoError(t, err)

	selector := sql.Select().From(sql.Table("users"))
	predicate(selector)
	return selector.Query()
}

func TestCompileComparisons(t *testing.T) {
	env, err := NewEnv(Schema{
		"active": Bool(Column("active")),
		"id":     Int(Column("id")),
		"name":   String(Column("name")),
	})
	require.NoError(t, err)

	tests := []struct {
		name     string
		filter   string
		contains string
		wantArgs []any
	}{
		{
			name:     "equals",
			filter:   `name == "leo"`,
			contains: "`users`.`name` = ?",
			wantArgs: []any{"leo"},
		},
		{
			name:     "not equals",
			filter:   `name != "leo"`,
			contains: "`users`.`name` <> ?",
			wantArgs: []any{"leo"},
		},
		{
			name:     "less than",
			filter:   `id < 10`,
			contains: "`users`.`id` < ?",
			wantArgs: []any{int64(10)},
		},
		{
			name:     "less than or equal",
			filter:   `id <= 10`,
			contains: "`users`.`id` <= ?",
			wantArgs: []any{int64(10)},
		},
		{
			name:     "greater than",
			filter:   `id > 10`,
			contains: "`users`.`id` > ?",
			wantArgs: []any{int64(10)},
		},
		{
			name:     "greater than or equal",
			filter:   `id >= 10`,
			contains: "`users`.`id` >= ?",
			wantArgs: []any{int64(10)},
		},
		{
			name:     "bool equals",
			filter:   `active == true`,
			contains: "`users`.`active` = ?",
			wantArgs: []any{true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, args := compileSQL(t, env, tt.filter)

			require.Contains(t, query, tt.contains)
			require.Equal(t, tt.wantArgs, args)
		})
	}
}

func TestCompileLogicalPredicates(t *testing.T) {
	env, err := NewEnv(Schema{
		"active": Bool(Column("active")),
		"id":     Int(Column("id")),
		"name":   String(Column("name")),
	})
	require.NoError(t, err)

	query, args := compileSQL(t, env, `active == true && (name == "leo" || id > 10)`)
	require.Contains(t, query, " AND ")
	require.Contains(t, query, " OR ")
	require.Equal(t, []any{true, "leo", int64(10)}, args)

	query, args = compileSQL(t, env, `!(name == "leo")`)
	require.Contains(t, query, "NOT")
	require.Equal(t, []any{"leo"}, args)
}

func TestCompileBooleanLiteralPredicates(t *testing.T) {
	env, err := NewEnv(Schema{})
	require.NoError(t, err)

	query, args := compileSQL(t, env, `true`)
	require.Equal(t, "SELECT * FROM `users`", query)
	require.Empty(t, args)

	query, args = compileSQL(t, env, `false`)
	require.Contains(t, query, "FALSE")
	require.Empty(t, args)
}

func TestCompilePredicateHookErrorFailsClosed(t *testing.T) {
	env, err := NewEnv(Schema{
		"custom": String(Predicate(func(context.Context, Operator, any, *sql.Selector) error {
			return errors.New("hook failed")
		})),
	})
	require.NoError(t, err)

	query, args := compileSQL(t, env, `custom == "x"`)

	require.Contains(t, query, "FALSE")
	require.Empty(t, args)
}

func TestCompileEmptyColumnResolverFailsClosed(t *testing.T) {
	env, err := NewEnv(Schema{
		"custom": String(ColumnExpr(func(*sql.Selector) string {
			return ""
		})),
	})
	require.NoError(t, err)

	tests := []string{
		`custom == "x"`,
		`custom in ["x"]`,
		`custom.contains("x")`,
		`!(custom == "x")`,
	}

	for _, filter := range tests {
		t.Run(filter, func(t *testing.T) {
			query, args := compileSQL(t, env, filter)

			require.Contains(t, query, "FALSE")
			require.NotContains(t, query, "NOT")
			require.Empty(t, args)
		})
	}
}

func TestCompileNegatedPredicateHookErrorFailsClosed(t *testing.T) {
	env, err := NewEnv(Schema{
		"custom": String(Predicate(func(context.Context, Operator, any, *sql.Selector) error {
			return errors.New("hook failed")
		})),
	})
	require.NoError(t, err)

	query, args := compileSQL(t, env, `!(custom == "x")`)

	require.Contains(t, query, "FALSE")
	require.NotContains(t, query, "NOT")
	require.Empty(t, args)
}

func TestCompileNullComparisons(t *testing.T) {
	env, err := NewEnv(Schema{
		"deleted_at": Time(Column("deleted_at")),
	})
	require.NoError(t, err)

	query, args := compileSQL(t, env, `deleted_at == null`)
	require.Contains(t, query, "`users`.`deleted_at` IS NULL")
	require.Empty(t, args)

	query, args = compileSQL(t, env, `deleted_at != null`)
	require.Contains(t, query, "`users`.`deleted_at` IS NOT NULL")
	require.Empty(t, args)
}

func TestCompileInList(t *testing.T) {
	env, err := NewEnv(Schema{
		"id": Int(Column("id")),
	})
	require.NoError(t, err)

	query, args := compileSQL(t, env, `id in [1, 2, 3]`)

	require.Contains(t, query, "`users`.`id` IN (?, ?, ?)")
	require.Equal(t, []any{int64(1), int64(2), int64(3)}, args)
}

func TestCompileStringMemberFunctions(t *testing.T) {
	env, err := NewEnv(Schema{
		"name": String(Column("name")),
	})
	require.NoError(t, err)

	tests := []struct {
		name     string
		filter   string
		contains string
		wantArgs []any
	}{
		{
			name:     "contains",
			filter:   `name.contains("eo")`,
			contains: "`users`.`name` LIKE ?",
			wantArgs: []any{"%eo%"},
		},
		{
			name:     "starts with",
			filter:   `name.startsWith("le")`,
			contains: "`users`.`name` LIKE ?",
			wantArgs: []any{"le%"},
		},
		{
			name:     "ends with",
			filter:   `name.endsWith("o")`,
			contains: "`users`.`name` LIKE ?",
			wantArgs: []any{"%o"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, args := compileSQL(t, env, tt.filter)

			require.Contains(t, query, tt.contains)
			require.Equal(t, tt.wantArgs, args)
		})
	}
}

func TestCompileConverter(t *testing.T) {
	env, err := NewEnv(Schema{
		"status": String(Column("status"), Convert(func(value any) (any, error) {
			if value == "open" {
				return 1, nil
			}
			return nil, errors.New("unknown status")
		})),
	})
	require.NoError(t, err)

	query, args := compileSQL(t, env, `status == "open"`)
	require.Contains(t, query, "`users`.`status` = ?")
	require.Equal(t, []any{1}, args)

	_, err = env.Compile(context.Background(), `status == "closed"`)
	require.ErrorIs(t, err, ErrConvertValue)
}

func TestCompileInExpandsConvertedSlices(t *testing.T) {
	env, err := NewEnv(Schema{
		"tag": String(Column("tag"), Convert(func(value any) (any, error) {
			if value == "hot" {
				return []string{"urgent", "priority"}, nil
			}
			return value, nil
		})),
	})
	require.NoError(t, err)

	query, args := compileSQL(t, env, `tag in ["hot", "cold"]`)

	require.Contains(t, query, "`users`.`tag` IN (?, ?, ?)")
	require.Equal(t, []any{"urgent", "priority", "cold"}, args)
}

func TestCompileStringPredicateHook(t *testing.T) {
	var gotOperator Operator
	var gotValue any
	env, err := NewEnv(Schema{
		"name": String(Predicate(func(_ context.Context, operator Operator, value any, selector *sql.Selector) error {
			gotOperator = operator
			gotValue = value
			selector.Where(sql.P(func(builder *sql.Builder) {
				builder.WriteString("LOWER(").Ident(selector.C("name")).WriteString(") LIKE LOWER(").Arg("%" + value.(string) + "%").WriteByte(')')
			}))
			return nil
		})),
	})
	require.NoError(t, err)

	query, args := compileSQL(t, env, `name.contains("leo")`)

	require.Equal(t, OperatorContains, gotOperator)
	require.Equal(t, "leo", gotValue)
	require.Contains(t, query, "LOWER(`users`.`name`) LIKE LOWER(?)")
	require.Equal(t, []any{"%leo%"}, args)
}

func TestCompilePredicateHookErrorsForInAndStringFailClosed(t *testing.T) {
	env, err := NewEnv(Schema{
		"custom": String(Predicate(func(context.Context, Operator, any, *sql.Selector) error {
			return errors.New("hook failed")
		})),
	})
	require.NoError(t, err)

	query, args := compileSQL(t, env, `custom in ["x"]`)
	require.Contains(t, query, "FALSE")
	require.Empty(t, args)

	query, args = compileSQL(t, env, `custom.contains("x")`)
	require.Contains(t, query, "FALSE")
	require.Empty(t, args)

	query, args = compileSQL(t, env, `!(custom.contains("x"))`)
	require.Contains(t, query, "FALSE")
	require.NotContains(t, query, "NOT")
	require.Empty(t, args)
}

func TestStringOverloadOperators(t *testing.T) {
	tests := []struct {
		name string
		want Operator
	}{
		{name: overloads.ContainsString, want: OperatorContains},
		{name: overloads.StartsWithString, want: OperatorStartsWith},
		{name: overloads.EndsWithString, want: OperatorEndsWith},
		{name: operators.In, want: OperatorIn},
	}

	for _, tt := range tests {
		got, ok := operatorFromCEL(tt.name)
		require.True(t, ok)
		require.Equal(t, tt.want, got)
	}
}
