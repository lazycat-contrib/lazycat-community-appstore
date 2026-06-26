package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type GroupMember struct {
	ent.Schema
}

func (GroupMember) Fields() []ent.Field {
	return []ent.Field{
		field.Int("group_id"),
		field.Int("user_id"),
		field.Time("created_at").Default(time.Now),
	}
}

func (GroupMember) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("group_id", "user_id").Unique(),
		index.Fields("user_id"),
	}
}
