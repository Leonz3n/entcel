package entcel

import (
	"context"
	"errors"
	"strings"
	"testing"

	"entgo.io/ent/dialect/sql"
	"github.com/stretchr/testify/require"
)

func compileShipmentSQL(t *testing.T, env *Env, filter string) (string, []any) {
	t.Helper()

	predicate, err := env.Compile(context.Background(), filter)
	require.NoError(t, err)

	selector := sql.Select().From(sql.Table("shipments"))
	predicate(selector)
	return selector.Query()
}

func shipmentSchema() Schema {
	return Schema{
		"tenant_id":   Int(Column("tenant_id")),
		"shipment_id": Int(Column("shipment_id")),
		"packages": Relation(RelationConfig{
			Table: "packages",
			Join: JoinColumns{
				"shipment_id": "shipment_id",
			},
			Schema: Schema{
				"tracking_number": String(Column("tracking_number")),
				"status":          String(Column("status")),
			},
		}),
	}
}

func TestCompileRelationExists(t *testing.T) {
	env, err := NewEnv(shipmentSchema())
	require.NoError(t, err)

	query, args := compileShipmentSQL(t, env, `tenant_id == 42 && exists("packages", "tracking_number == 'TN1' && status == 'shipped'")`)

	require.Contains(t, query, "EXISTS(SELECT 1 FROM `packages`")
	require.Contains(t, query, "`shipments`.`shipment_id` = `packages`.`shipment_id`")
	require.Contains(t, query, "`packages`.`tracking_number` = ?")
	require.Contains(t, query, "`packages`.`status` = ?")
	require.Contains(t, query, "LIMIT 1")
	require.Equal(t, []any{int64(42), "TN1", "shipped"}, args)
}

func TestCompileRelationExistsWithEmptyNestedFilter(t *testing.T) {
	env, err := NewEnv(shipmentSchema())
	require.NoError(t, err)

	query, args := compileShipmentSQL(t, env, `exists("packages", "")`)

	require.Contains(t, query, "EXISTS(SELECT 1 FROM `packages`")
	require.Contains(t, query, "`shipments`.`shipment_id` = `packages`.`shipment_id`")
	require.NotContains(t, query, "`packages`.`tracking_number` = ?")
	require.NotContains(t, query, "`packages`.`status` = ?")
	require.Contains(t, query, "LIMIT 1")
	require.Empty(t, args)
}

func TestCompileRelationExistsErrors(t *testing.T) {
	env, err := NewEnv(shipmentSchema())
	require.NoError(t, err)

	_, err = env.Compile(context.Background(), `exists("items", "status == 'shipped'")`)
	require.ErrorIs(t, err, ErrUnknownRelation)

	_, err = env.Compile(context.Background(), `exists("packages", "tenant_id == 1")`)
	require.ErrorIs(t, err, ErrInvalidRelationFilter)
}

func TestCompileRelationExistsRejectsEmptyJoin(t *testing.T) {
	tests := []struct {
		name string
		join JoinColumns
	}{
		{name: "nil join"},
		{name: "empty join", join: JoinColumns{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, err := NewEnv(Schema{
				"packages": Relation(RelationConfig{
					Table:  "packages",
					Join:   tt.join,
					Schema: Schema{},
				}),
			})
			require.NoError(t, err)

			_, err = env.Compile(context.Background(), `exists("packages", "")`)

			require.True(t, errors.Is(err, ErrInvalidRelationFilter))
		})
	}
}

func TestCompileRelationExistsRejectsInvalidRelationConfig(t *testing.T) {
	tests := []struct {
		name     string
		relation RelationConfig
	}{
		{
			name: "empty table",
			relation: RelationConfig{
				Table: "",
				Join: JoinColumns{
					"id": "shipment_id",
				},
				Schema: Schema{},
			},
		},
		{
			name: "empty parent join column",
			relation: RelationConfig{
				Table: "packages",
				Join: JoinColumns{
					"": "shipment_id",
				},
				Schema: Schema{},
			},
		},
		{
			name: "empty child join column",
			relation: RelationConfig{
				Table: "packages",
				Join: JoinColumns{
					"id": "",
				},
				Schema: Schema{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, err := NewEnv(Schema{
				"packages": Relation(tt.relation),
			})
			require.NoError(t, err)

			_, err = env.Compile(context.Background(), `exists("packages", "")`)

			require.ErrorIs(t, err, ErrInvalidRelationFilter)
		})
	}
}

func TestCompileRelationExistsSortsJoinColumns(t *testing.T) {
	env, err := NewEnv(Schema{
		"packages": Relation(RelationConfig{
			Table: "packages",
			Join: JoinColumns{
				"tenant_id":   "tenant_id",
				"shipment_id": "shipment_id",
			},
			Schema: Schema{},
		}),
	})
	require.NoError(t, err)

	query, args := compileShipmentSQL(t, env, `exists("packages", "")`)

	shipmentJoin := "`shipments`.`shipment_id` = `packages`.`shipment_id`"
	tenantJoin := "`shipments`.`tenant_id` = `packages`.`tenant_id`"
	require.Contains(t, query, shipmentJoin)
	require.Contains(t, query, tenantJoin)
	require.Less(t, strings.Index(query, shipmentJoin), strings.Index(query, tenantJoin))
	require.Empty(t, args)
}
