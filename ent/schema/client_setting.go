package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type ClientSetting struct {
	ent.Schema
}

func (ClientSetting) Fields() []ent.Field {
	return []ent.Field{
		field.String("user_id").NotEmpty(),
		field.String("key").NotEmpty(),
		field.Text("value").Default(""),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (ClientSetting) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "key").Unique(),
	}
}
