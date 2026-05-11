# Contributing

Thanks for helping improve entcel.

## Development

Run the full checks before sending changes:

```sh
go test ./...
go vet ./...
```

## Design Boundaries

- entcel compiles a small, declared CEL subset into entgo SQL predicates.
- entcel does not execute queries or own repository access.
- entcel only exposes fields declared in `Schema`.
- entcel should keep SQL construction inside entgo's SQL builder APIs.
- Predicate builders should be deterministic, selector-local, and free of remote calls or repository lookups.
- Unsupported CEL expressions should return clear errors instead of falling back to runtime evaluation or query rewrites.
