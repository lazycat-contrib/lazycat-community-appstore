package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Collection struct {
	ent.Schema
}

func (Collection) Fields() []ent.Field {
	return []ent.Field{
		field.Int("creator_id"),
		field.String("name").NotEmpty(),
		field.String("slug").NotEmpty().Unique(),
		field.Text("description").Default(""),
		field.Enum("kind").Values("MANUAL", "RECENT_UPDATED", "MOST_DOWNLOADED").Default("MANUAL"),
		field.Int("sort_order").Default(0),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (Collection) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("kind"),
		index.Fields("creator_id"),
	}
}
