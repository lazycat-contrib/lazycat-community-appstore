package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Announcement struct {
	ent.Schema
}

func (Announcement) Fields() []ent.Field {
	return []ent.Field{
		field.Bool("enabled").Default(true),
		field.Enum("level").Values("info", "warning", "success").Default("info"),
		field.String("title").Default(""),
		field.Text("body").Default(""),
		field.String("link_label").Default(""),
		field.String("link_url").Default(""),
		field.Time("starts_at").Optional().Nillable(),
		field.Time("ends_at").Optional().Nillable(),
		field.Int("sort_order").Default(0),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (Announcement) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("enabled", "sort_order", "updated_at"),
		index.Fields("starts_at", "ends_at"),
	}
}
