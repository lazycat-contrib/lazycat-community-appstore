package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type CollaboratorInvite struct {
	ent.Schema
}

func (CollaboratorInvite) Fields() []ent.Field {
	return []ent.Field{
		field.Int("app_id"),
		field.Int("inviter_id"),
		field.String("email").Optional().Nillable(),
		field.String("token").NotEmpty().Unique().Sensitive(),
		field.String("token_prefix").NotEmpty(),
		field.Int("accepted_by").Optional().Nillable(),
		field.Time("accepted_at").Optional().Nillable(),
		field.Time("expires_at"),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (CollaboratorInvite) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("app_id"),
		index.Fields("inviter_id"),
		index.Fields("accepted_by"),
		index.Fields("expires_at"),
	}
}
