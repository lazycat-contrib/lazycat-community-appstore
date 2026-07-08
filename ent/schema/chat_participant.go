package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type ChatParticipant struct {
	ent.Schema
}

func (ChatParticipant) Fields() []ent.Field {
	return []ent.Field{
		field.Int("conversation_id"),
		field.Enum("actor_type").Values("USER", "CLIENT").Default("USER"),
		field.Int("user_id").Default(0),
		field.String("client_user_id").Default(""),
		field.String("display_name").Default(""),
		field.String("avatar_url").Default(""),
		field.Time("last_read_at").Optional().Nillable(),
		field.Time("hidden_at").Optional().Nillable(),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (ChatParticipant) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("conversation_id"),
		index.Fields("conversation_id", "actor_type", "user_id", "client_user_id"),
		index.Fields("actor_type", "user_id", "updated_at"),
		index.Fields("actor_type", "client_user_id", "updated_at"),
	}
}
