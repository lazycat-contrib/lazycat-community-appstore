package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Collaborator struct {
	ent.Schema
}

func (Collaborator) Fields() []ent.Field {
	return []ent.Field{
		field.Int("app_id"),
		field.Int("user_id"),
		field.Time("created_at").Default(time.Now),
	}
}

func (Collaborator) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("app_id", "user_id").Unique(),
		index.Fields("user_id"),
	}
}
