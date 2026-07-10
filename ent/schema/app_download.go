package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type AppDownload struct {
	ent.Schema
}

func (AppDownload) Fields() []ent.Field {
	return []ent.Field{
		field.Int("app_id"),
		field.String("version").Default(""),
		field.Time("created_at").Default(time.Now),
	}
}

func (AppDownload) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("app_id", "created_at"),
		index.Fields("created_at", "app_id"),
	}
}
