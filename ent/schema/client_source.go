package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type ClientSource struct {
	ent.Schema
}

func (ClientSource) Fields() []ent.Field {
	return []ent.Field{
		field.String("user_id").NotEmpty(),
		field.String("name").NotEmpty(),
		field.String("url").NotEmpty(),
		field.String("password").Default(""),
		field.String("default_download_mirror_id").Default(""),
		field.String("default_raw_mirror_id").Default(""),
		field.Text("group_codes_json").Default(""),
		field.Text("group_names_json").Default(""),
		field.Text("last_invalid_group_codes_json").Default(""),
		field.Text("mirrors_json").Default(""),
		field.Text("categories_json").Default(""),
		field.Text("announcements_json").Default(""),
		field.String("min_client_version").Default(""),
		field.Text("min_client_version_message").Default(""),
		field.Bool("chat_available").Default(false),
		field.Bool("chat_enabled").Default(true),
		field.Time("last_sync").Optional().Nillable(),
		field.String("last_error").Optional().Nillable(),
		field.Enum("last_error_code").Values("auth", "format", "http", "network").Optional().Nillable(),
		field.Int("last_app_count").Default(0),
		field.Int("last_installable_count").Default(0),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (ClientSource) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("apps", ClientSourceApp.Type),
	}
}

func (ClientSource) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "url").Unique(),
		index.Fields("user_id", "updated_at"),
	}
}
