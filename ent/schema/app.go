package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type App struct {
	ent.Schema
}

func (App) Fields() []ent.Field {
	return []ent.Field{
		field.Int("owner_id"),
		field.Int("category_id").Optional().Nillable(),
		field.String("name").NotEmpty(),
		field.String("slug").NotEmpty().Unique(),
		field.String("summary").Default(""),
		field.Text("description").Default(""),
		field.String("icon_url").Optional().Nillable(),
		field.Enum("status").Values("DRAFT", "PENDING", "APPROVED", "REJECTED", "UNLISTED").Default("PENDING"),
		field.Bool("allow_unreviewed_updates").Default(false),
		field.Bool("comments_enabled").Default(true),
		field.Int("download_count").Default(0),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (App) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("owner_id"),
		index.Fields("category_id"),
		index.Fields("status"),
	}
}
