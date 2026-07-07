package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type RegistrationInvite struct {
	ent.Schema
}

func (RegistrationInvite) Fields() []ent.Field {
	return []ent.Field{
		field.String("code").NotEmpty().Unique(),
		field.String("code_hash").NotEmpty().Unique().Sensitive(),
		field.String("code_prefix").NotEmpty(),
		field.String("note").Default(""),
		field.Int("max_uses").Default(1).Positive(),
		field.Int("remaining_uses").Default(1).NonNegative(),
		field.Int("created_by"),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (RegistrationInvite) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("code_prefix"),
		index.Fields("created_by"),
	}
}
