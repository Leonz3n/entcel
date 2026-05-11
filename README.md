# entcel

Compile CEL filters into entgo SQL predicates.

entcel lets HTTP and API filters use a small, declared CEL subset while your
application keeps control over which fields are queryable and how they map to
entgo SQL selectors.

## Quick Start

```go
schema := entcel.Schema{
	"id":   entcel.Int(entcel.Column("id")),
	"name": entcel.String(entcel.Column("name")),
}

env, err := entcel.NewEnv(schema)
if err != nil {
	return err
}

predicate, err := env.Compile(ctx, `name.contains("leo") && id in [1, 2, 3]`)
if err != nil {
	return err
}

selector := sql.Select().From(sql.Table("users"))
predicate(selector)

query, args := selector.Query()
```

## Supported CEL Subset

| Feature | CEL examples | SQL behavior |
| --- | --- | --- |
| Comparisons | `id == 1`, `age >= 18`, `"leo" != name` | Column comparisons with literal values |
| Boolean logic | `active == true && name != "bot"`, `!(deleted_at != null)` | `AND`, `OR`, and `NOT` predicate groups |
| Membership | `id in [1, 2, 3]` | `IN` over literal lists |
| Strings | `name.contains("leo")`, `name.startsWith("l")`, `name.endsWith("o")` | entgo string predicates using `LIKE` |
| Null checks | `deleted_at == null`, `deleted_at != null` | `IS NULL` and `IS NOT NULL` |
| Relation exists | `exists("packages", "status == 'shipped'")` | Correlated `EXISTS` subquery |

## Relation Exists

Declare relations with a target table, join columns, and a nested schema:

```go
schema := entcel.Schema{
	"tenant_id": entcel.Int(entcel.Column("tenant_id")),
	"packages": entcel.Relation(entcel.RelationConfig{
		Table: "packages",
		Join: entcel.JoinColumns{
			"shipment_id": "shipment_id",
		},
		Schema: entcel.Schema{
			"tracking_number": entcel.String(entcel.Column("tracking_number")),
			"status":          entcel.String(entcel.Column("status")),
		},
	}),
}

predicate, err := env.Compile(ctx,
	`tenant_id == 42 && exists("packages", "status == 'shipped'")`,
)
```

Relation filters are compiled with the relation schema, so parent fields are not
available inside the nested filter.

## Converters

Use converters when CEL literal values need to become domain or database values
before SQL is built:

```go
schema := entcel.Schema{
	"created_at": entcel.Time(
		entcel.Column("created_at"),
		entcel.Convert(entcel.ConvertTime(time.RFC3339)),
	),
}
```

## Security

Only fields declared in `Schema` are queryable. Unknown fields fail compilation,
and generated SQL arguments go through the entgo SQL builder rather than string
concatenation.

Custom `PredicateBuilder` hooks can add specialized predicates, but they should
be deterministic and limited to the current selector. Avoid remote calls,
repository lookups, or request-dependent side effects inside predicate builders.

## Non-Goals

- Generic SQL backend support.
- Query execution.
- Cross-service field resolution.
- Expression rewrite fallback for unsupported CEL.
- Arbitrary CEL runtime evaluation.
