package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type AppTag struct {
	ent.Schema
}

func (AppTag) Fields() []ent.Field {
	return []ent.Field{
		field.Int("app_id"),
		field.Int("tag_id"),
		field.Time("created_at").Default(time.Now),
	}
}

func (AppTag) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("app_id", "tag_id").Unique(),
		index.Fields("tag_id"),
	}
}
