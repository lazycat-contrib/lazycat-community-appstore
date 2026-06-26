package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type AppScreenshot struct {
	ent.Schema
}

func (AppScreenshot) Fields() []ent.Field {
	return []ent.Field{
		field.Int("app_id"),
		field.Int("uploader_id"),
		field.String("image_url").NotEmpty(),
		field.String("storage_path").Default(""),
		field.String("caption").Default(""),
		field.Int("sort_order").Default(0),
		field.Time("created_at").Default(time.Now),
	}
}

func (AppScreenshot) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("app_id", "sort_order"),
		index.Fields("uploader_id"),
	}
}
