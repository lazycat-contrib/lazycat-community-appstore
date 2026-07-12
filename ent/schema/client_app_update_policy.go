package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type ClientAppUpdatePolicy struct {
	ent.Schema
}

func (ClientAppUpdatePolicy) Fields() []ent.Field {
	return []ent.Field{
		field.String("user_id").NotEmpty(),
		field.String("package_id").NotEmpty(),
		field.Bool("auto_update_enabled").Default(true),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (ClientAppUpdatePolicy) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "package_id").Unique(),
	}
}
