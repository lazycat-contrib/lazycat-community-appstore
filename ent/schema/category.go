package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Category struct {
	ent.Schema
}

func (Category) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").NotEmpty(),
		field.Text("name_i18n").Default("{}"),
		field.String("slug").NotEmpty().Unique(),
		field.Int("parent_id").Optional().Nillable(),
		field.Int("sort_order").Default(0),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (Category) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("parent_id"),
	}
}
