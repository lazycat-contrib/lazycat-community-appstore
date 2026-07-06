package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Comment struct {
	ent.Schema
}

func (Comment) Fields() []ent.Field {
	return []ent.Field{
		field.Int("app_id"),
		field.Int("user_id"),
		field.Int("parent_id").Optional().Nillable(),
		field.Enum("author_type").Values("USER", "CLIENT").Default("USER"),
		field.String("author_name").Default(""),
		field.String("client_user_id").Default(""),
		field.Text("body").NotEmpty(),
		field.Bool("deleted").Default(false),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (Comment) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("app_id", "deleted"),
		index.Fields("app_id", "parent_id", "deleted"),
		index.Fields("user_id"),
		index.Fields("client_user_id"),
	}
}
