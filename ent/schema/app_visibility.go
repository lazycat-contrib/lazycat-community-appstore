package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type AppVisibility struct {
	ent.Schema
}

func (AppVisibility) Fields() []ent.Field {
	return []ent.Field{
		field.Int("app_id"),
		field.Int("group_id"),
		field.Time("created_at").Default(time.Now),
	}
}

func (AppVisibility) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("app_id", "group_id").Unique(),
		index.Fields("group_id"),
	}
}
