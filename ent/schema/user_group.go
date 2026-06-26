package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type UserGroup struct {
	ent.Schema
}

func (UserGroup) Fields() []ent.Field {
	return []ent.Field{
		field.Int("owner_id"),
		field.String("name").NotEmpty(),
		field.String("slug").NotEmpty(),
		field.Text("description").Default(""),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (UserGroup) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("owner_id", "slug").Unique(),
	}
}
