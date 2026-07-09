package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type AssetLink struct {
	ent.Schema
}

func (AssetLink) Fields() []ent.Field {
	return []ent.Field{
		field.Int("asset_id").Positive(),
		field.String("owner_type").NotEmpty(),
		field.Int("owner_id").NonNegative(),
		field.String("role").NotEmpty(),
		field.Int("sort_order").Default(0),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (AssetLink) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("asset_id"),
		index.Fields("owner_type", "owner_id"),
		index.Fields("owner_type", "owner_id", "role", "sort_order"),
		index.Fields("owner_type", "owner_id", "role", "asset_id").Unique(),
	}
}
