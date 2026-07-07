package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type MCPToken struct {
	ent.Schema
}

func (MCPToken) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id"),
		field.Enum("principal_type").Values("USER", "ADMIN").Default("USER"),
		field.String("note").Default(""),
		field.String("prefix").NotEmpty(),
		field.String("token_hash").NotEmpty().Unique().Sensitive(),
		field.Time("expires_at").Optional().Nillable(),
		field.Time("last_used_at").Optional().Nillable(),
		field.Time("created_at").Default(time.Now),
	}
}

func (MCPToken) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("principal_type"),
		index.Fields("prefix"),
		index.Fields("expires_at"),
	}
}
