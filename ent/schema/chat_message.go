package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type ChatMessage struct {
	ent.Schema
}

func (ChatMessage) Fields() []ent.Field {
	return []ent.Field{
		field.Int("conversation_id"),
		field.Enum("sender_type").Values("USER", "CLIENT").Default("USER"),
		field.Int("sender_user_id").Default(0),
		field.String("sender_client_user_id").Default(""),
		field.String("sender_name").Default(""),
		field.String("sender_avatar_url").Default(""),
		field.Text("body").NotEmpty(),
		field.Bool("deleted").Default(false),
		field.Time("created_at").Default(time.Now),
	}
}

func (ChatMessage) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("conversation_id", "deleted", "created_at"),
		index.Fields("sender_type", "sender_user_id", "created_at"),
		index.Fields("sender_type", "sender_client_user_id", "created_at"),
	}
}
