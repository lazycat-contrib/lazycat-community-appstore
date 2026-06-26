package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type CollaboratorRequest struct {
	ent.Schema
}

func (CollaboratorRequest) Fields() []ent.Field {
	return []ent.Field{
		field.Int("app_id"),
		field.Int("user_id"),
		field.Enum("status").Values("PENDING", "APPROVED", "REJECTED").Default("PENDING"),
		field.Text("message").Default(""),
		field.Time("reviewed_at").Optional().Nillable(),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (CollaboratorRequest) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("app_id", "user_id").Unique(),
		index.Fields("status"),
	}
}
