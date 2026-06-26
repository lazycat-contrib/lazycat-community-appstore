package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type APIToken struct {
	ent.Schema
}

func (APIToken) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id"),
		field.String("name").NotEmpty(),
		field.String("prefix").NotEmpty(),
		field.String("token_hash").NotEmpty().Unique().Sensitive(),
		field.Time("last_used_at").Optional().Nillable(),
		field.Time("created_at").Default(time.Now),
	}
}

func (APIToken) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("prefix"),
	}
}
