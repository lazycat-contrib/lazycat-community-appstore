package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type AppVersion struct {
	ent.Schema
}

func (AppVersion) Fields() []ent.Field {
	return []ent.Field{
		field.Int("app_id"),
		field.Int("uploader_id"),
		field.String("version").NotEmpty(),
		field.Text("changelog").Default(""),
		field.Enum("status").Values("PENDING", "APPROVED", "REJECTED").Default("PENDING"),
		field.Enum("source_type").Values("LOCAL", "WEBDAV", "S3", "GITHUB").Default("LOCAL"),
		field.String("download_url").Default(""),
		field.String("storage_key").Default("primary"),
		field.String("storage_path").Default(""),
		field.Int64("file_size").Default(0),
		field.String("sha256").Default(""),
		field.Time("published_at").Optional().Nillable(),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (AppVersion) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("app_id", "version").Unique(),
		index.Fields("app_id", "status"),
		index.Fields("app_id", "status", "published_at", "created_at"),
		index.Fields("uploader_id"),
	}
}
