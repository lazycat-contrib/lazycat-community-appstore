package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Favorite struct {
	ent.Schema
}

func (Favorite) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id"),
		field.Enum("target_type").Values("APP", "SUBMITTER"),
		field.Int("target_id"),
		field.Time("created_at").Default(time.Now),
	}
}

func (Favorite) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "target_type", "target_id").Unique(),
		index.Fields("target_type", "target_id"),
	}
}
