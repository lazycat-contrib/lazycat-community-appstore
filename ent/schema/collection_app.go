package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type CollectionApp struct {
	ent.Schema
}

func (CollectionApp) Fields() []ent.Field {
	return []ent.Field{
		field.Int("collection_id"),
		field.Int("app_id"),
		field.Int("sort_order").Default(0),
		field.Time("created_at").Default(time.Now),
	}
}

func (CollectionApp) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("collection_id", "app_id").Unique(),
		index.Fields("app_id"),
	}
}
