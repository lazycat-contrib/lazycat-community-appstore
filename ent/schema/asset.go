package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Asset struct {
	ent.Schema
}

func (Asset) Fields() []ent.Field {
	return []ent.Field{
		field.String("sha256").NotEmpty(),
		field.String("media_type").NotEmpty(),
		field.Int64("size").NonNegative(),
		field.Bytes("data").NotEmpty(),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (Asset) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("sha256").Unique(),
	}
}
