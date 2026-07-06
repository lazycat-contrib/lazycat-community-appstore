package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type ClientInstallHistory struct {
	ent.Schema
}

func (ClientInstallHistory) Fields() []ent.Field {
	return []ent.Field{
		field.String("user_id").Default(""),
		field.Int("source_id").Optional().Nillable(),
		field.Int("source_app_id").Optional().Nillable(),
		field.String("source_name").Default(""),
		field.String("package_id").NotEmpty(),
		field.String("app_name").NotEmpty(),
		field.String("version").Default(""),
		field.Enum("result").Values("SUCCESS", "FAILED").Default("SUCCESS"),
		field.String("download_url").Default(""),
		field.String("sha256").Default(""),
		field.Text("error").Default(""),
		field.Time("created_at").Default(time.Now),
	}
}

func (ClientInstallHistory) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "created_at"),
		index.Fields("package_id", "created_at"),
		index.Fields("source_id", "created_at"),
	}
}
