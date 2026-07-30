package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type WishReply struct{ ent.Schema }

func (WishReply) Fields() []ent.Field {
	return []ent.Field{
		field.Int("wish_id"),
		field.Int("author_user_id"),
		field.String("author_name").Default(""),
		field.Text("body").NotEmpty(),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (WishReply) Indexes() []ent.Index {
	return []ent.Index{index.Fields("wish_id", "created_at")}
}
