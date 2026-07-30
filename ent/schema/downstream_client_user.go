package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type DownstreamClientUser struct{ ent.Schema }

func (DownstreamClientUser) Fields() []ent.Field {
	return []ent.Field{
		field.String("client_user_id").NotEmpty().Unique(),
		field.String("display_name").Default(""),
		field.Bool("seen_in_comments").Default(false),
		field.Bool("seen_in_wishes").Default(false),
		field.Bool("blocked").Default(false),
		field.Text("block_reason").Default(""),
		field.Int("blocked_by").Optional().Nillable(),
		field.Time("blocked_at").Optional().Nillable(),
		field.Time("first_seen_at").Default(time.Now),
		field.Time("last_seen_at").Default(time.Now),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (DownstreamClientUser) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("blocked", "last_seen_at"),
		index.Fields("last_seen_at"),
	}
}
