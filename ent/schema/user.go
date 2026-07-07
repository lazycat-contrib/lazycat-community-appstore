package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type User struct {
	ent.Schema
}

func (User) Fields() []ent.Field {
	return []ent.Field{
		field.String("username").NotEmpty().Unique(),
		field.String("nickname").Default(""),
		field.String("avatar_url").Default(""),
		field.String("avatar_storage_key").Default("primary"),
		field.String("avatar_storage_path").Default(""),
		field.String("email").Optional().Nillable(),
		field.String("password_hash").Sensitive(),
		field.Enum("role").Values("USER", "SOFTWARE_ADMIN", "SITE_ADMIN").Default("USER"),
		field.Bool("email_verified").Default(false),
		field.Bool("disabled").Default(false),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (User) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("email").Unique(),
	}
}
