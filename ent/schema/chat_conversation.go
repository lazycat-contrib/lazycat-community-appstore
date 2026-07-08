package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type ChatConversation struct {
	ent.Schema
}

func (ChatConversation) Fields() []ent.Field {
	return []ent.Field{
		field.Int("app_id").Optional().Nillable(),
		field.String("topic").Default(""),
		field.Text("last_message_body").Default(""),
		field.String("last_message_sender_name").Default(""),
		field.Time("last_message_at").Optional().Nillable(),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (ChatConversation) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("app_id"),
		index.Fields("last_message_at"),
		index.Fields("updated_at"),
	}
}
