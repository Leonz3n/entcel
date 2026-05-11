package main

import (
	"context"
	"fmt"
	"log"

	"entgo.io/ent/dialect/sql"
	"github.com/leonz3n/entcel"
)

func main() {
	env, err := entcel.NewEnv(entcel.Schema{
		"id":   entcel.Int(entcel.Column("id")),
		"name": entcel.String(entcel.Column("name")),
	})
	if err != nil {
		log.Fatal(err)
	}

	predicate, err := env.Compile(context.Background(), `name.contains("leo") && id in [1, 2, 3]`)
	if err != nil {
		log.Fatal(err)
	}

	selector := sql.Select().From(sql.Table("users"))
	predicate(selector)

	query, args := selector.Query()
	fmt.Println(query)
	fmt.Println(args)
}
