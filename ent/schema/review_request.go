package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type ReviewRequest struct {
	ent.Schema
}

func (ReviewRequest) Fields() []ent.Field {
	return []ent.Field{
		field.Enum("kind").Values("APP_SUBMISSION", "VERSION_UPLOAD", "APP_INFO_UPDATE").Default("APP_SUBMISSION"),
		field.Enum("status").Values("PENDING", "APPROVED", "REJECTED").Default("PENDING"),
		field.Int("app_id").Optional().Nillable(),
		field.Int("version_id").Optional().Nillable(),
		field.Int("requester_id"),
		field.Int("reviewer_id").Optional().Nillable(),
		field.Text("note").Default(""),
		field.Text("review_note").Default(""),
		field.Time("reviewed_at").Optional().Nillable(),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (ReviewRequest) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status"),
		index.Fields("app_id"),
		index.Fields("version_id"),
		index.Fields("requester_id"),
	}
}
