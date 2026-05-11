package entcel

import (
	"context"
	"sort"

	"entgo.io/ent/dialect/sql"
)

type JoinColumns map[string]string

type RelationConfig struct {
	Table  string
	Join   JoinColumns
	Schema Schema
}

func Relation(config RelationConfig) Field {
	return Field{
		relation: &config,
	}
}

func compileExists(ctx context.Context, filter string, relationName string, relation RelationConfig, nestedFilter string) (selectorPredicate, error) {
	if relation.Table == "" {
		return nil, newError(ErrInvalidRelationFilter, filter, relationName, "", "relation %q requires a table", relationName)
	}
	if len(relation.Join) == 0 {
		return nil, newError(ErrInvalidRelationFilter, filter, relationName, "", "relation %q requires at least one join column", relationName)
	}
	for parentColumn, childColumn := range relation.Join {
		if parentColumn == "" || childColumn == "" {
			return nil, newError(ErrInvalidRelationFilter, filter, relationName, "", "relation %q requires non-empty join columns", relationName)
		}
	}

	nestedEnv, err := NewEnv(relation.Schema, AllowEmptyFilter())
	if err != nil {
		return nil, wrapError(ErrInvalidRelationFilter, filter, relationName, "", "create relation environment", err)
	}

	nestedCompiler := &compiler{
		env:    nestedEnv,
		ctx:    ctx,
		filter: nestedFilter,
	}
	nestedPredicate, err := nestedCompiler.compileFilter(nestedFilter)
	if err != nil {
		return nil, wrapError(ErrInvalidRelationFilter, filter, relationName, "", "compile relation filter", err)
	}

	parentColumns := make([]string, 0, len(relation.Join))
	for parentColumn := range relation.Join {
		parentColumns = append(parentColumns, parentColumn)
	}
	sort.Strings(parentColumns)

	return func(parent *sql.Selector) error {
		child := sql.Select().From(sql.Table(relation.Table)).Limit(1)
		child.AppendSelectExpr(sql.Raw("1"))
		for _, parentColumn := range parentColumns {
			child.Where(sql.ColumnsEQ(parent.C(parentColumn), child.C(relation.Join[parentColumn])))
		}
		if err := nestedPredicate(child); err != nil {
			return err
		}
		query, args := child.Query()
		parent.Where(sql.ExprP("EXISTS("+query+")", args...))
		return nil
	}, nil
}
