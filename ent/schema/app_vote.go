package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type AppVote struct {
	ent.Schema
}

func (AppVote) Fields() []ent.Field {
	return []ent.Field{
		field.Int("app_id"),
		field.Int("user_id"),
		field.Int("value").Default(1),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (AppVote) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("app_id", "user_id").Unique(),
		index.Fields("app_id", "updated_at"),
		index.Fields("user_id", "updated_at"),
	}
}
