package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type WishStatusEvent struct{ ent.Schema }

func (WishStatusEvent) Fields() []ent.Field {
	return []ent.Field{
		field.Int("wish_id"),
		field.String("from_status").Default(""),
		field.Enum("to_status").Values("OPEN", "PLANNED", "IN_PROGRESS", "COMPLETED", "REJECTED"),
		field.Enum("actor_type").Values("CLIENT", "USER"),
		field.Int("actor_user_id").Optional().Nillable(),
		field.String("actor_client_user_id").Default(""),
		field.String("actor_name").Default(""),
		field.Text("text").NotEmpty(),
		field.Time("created_at").Default(time.Now),
	}
}

func (WishStatusEvent) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("wish_id", "created_at"),
		index.Fields("actor_user_id", "created_at"),
		index.Fields("actor_client_user_id", "created_at"),
	}
}
