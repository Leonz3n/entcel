package entcel

import (
	"context"
	"regexp"

	"entgo.io/ent/dialect/sql"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/decls"
	"github.com/google/cel-go/common/types"
)

var undeclaredReferenceRE = regexp.MustCompile("undeclared reference to '([^']+)'")

type PredicateFunc func(*sql.Selector)

type Env struct {
	cel              *cel.Env
	schema           Schema
	allowEmptyFilter bool
}

type EnvOption func(*Env)

func AllowEmptyFilter() EnvOption {
	return func(env *Env) {
		env.allowEmptyFilter = true
	}
}

func NewEnv(schema Schema, options ...EnvOption) (*Env, error) {
	declarations := make([]*decls.VariableDecl, 0, len(schema))
	for name, field := range schema {
		if field.relation != nil {
			continue
		}
		declarations = append(declarations, decls.NewVariable(name, field.celType))
	}

	exists, err := decls.NewFunction("exists",
		decls.Overload("exists_string_string", []*types.Type{types.StringType, types.StringType}, types.BoolType),
	)
	if err != nil {
		return nil, err
	}

	celEnv, err := cel.NewEnv(
		cel.VariableDecls(declarations...),
		cel.FunctionDecls(exists),
	)
	if err != nil {
		return nil, err
	}

	env := &Env{
		cel:    celEnv,
		schema: schema,
	}
	for _, option := range options {
		option(env)
	}
	return env, nil
}

func (env *Env) Compile(ctx context.Context, filter string) (PredicateFunc, error) {
	compiler := &compiler{
		env:    env,
		ctx:    ctx,
		filter: filter,
	}
	predicate, err := compiler.compileFilter(filter)
	if err != nil {
		return nil, err
	}
	return func(selector *sql.Selector) {
		if err := predicate(selector); err != nil {
			selector.Where(sql.False())
		}
	}, nil
}
