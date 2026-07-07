package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

type StorageConfig struct {
	ent.Schema
}

func (StorageConfig) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "storage_configs"},
	}
}

func (StorageConfig) Fields() []ent.Field {
	return []ent.Field{
		field.String("key").NotEmpty().Unique(),
		field.String("name").Default(""),
		field.Enum("provider").Values("LOCAL", "S3", "CLOUDFLARE_R2", "WEBDAV").Default("LOCAL"),
		field.Enum("delivery_mode").Values("SERVER", "DIRECT").Default("SERVER"),
		field.String("local_path").Default(""),
		field.String("endpoint_url").Default(""),
		field.String("bucket_name").Default(""),
		field.String("region").Default("auto"),
		field.Bool("path_style").Default(true),
		field.String("account_id").Default(""),
		field.String("root_prefix").Default(""),
		field.String("access_key_id").Default("").Sensitive(),
		field.String("secret_access_key").Default("").Sensitive(),
		field.String("webdav_username").Default(""),
		field.String("webdav_password").Default("").Sensitive(),
		field.String("public_base_url").Default(""),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}
