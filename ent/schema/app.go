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
		field.String("package_id").NotEmpty().Unique(),
		field.String("name").NotEmpty(),
		field.Text("name_i18n_json").Default("{}"),
		field.String("slug").NotEmpty().Unique(),
		field.String("summary").Default(""),
		field.Text("summary_i18n_json").Default("{}"),
		field.Text("description").Default(""),
		field.Text("description_i18n_json").Default("{}"),
		field.String("author").Default(""),
		field.String("homepage").Default(""),
		field.String("license").Default(""),
		field.String("min_os_version").Default(""),
		field.String("icon_url").Optional().Nillable(),
		field.Enum("status").Values("DRAFT", "PENDING", "APPROVED", "REJECTED", "UNLISTED").Default("PENDING"),
		field.Bool("allow_unreviewed_updates").Default(false),
		field.Bool("comments_enabled").Default(true),
		field.Bool("email_notifications_enabled").Default(true),
		field.String("install_password_hash").Default(""),
		field.Int("download_count").Default(0),
		field.Int("version_retention_count").Optional().Nillable(),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (App) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("owner_id"),
		index.Fields("owner_id", "updated_at"),
		index.Fields("category_id"),
		index.Fields("category_id", "status", "updated_at"),
		index.Fields("status"),
		index.Fields("status", "updated_at"),
		index.Fields("status", "download_count", "updated_at"),
	}
}
