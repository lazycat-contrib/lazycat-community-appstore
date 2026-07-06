package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type CommentNotification struct {
	ent.Schema
}

func (CommentNotification) Fields() []ent.Field {
	return []ent.Field{
		field.Int("owner_id"),
		field.Int("app_id"),
		field.Int("comment_id"),
		field.String("app_name").Default(""),
		field.String("actor_name").Default(""),
		field.Text("body").Default(""),
		field.Bool("read").Default(false),
		field.Time("created_at").Default(time.Now),
	}
}

func (CommentNotification) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("owner_id", "read", "created_at"),
		index.Fields("app_id", "created_at"),
		index.Fields("comment_id"),
	}
}
