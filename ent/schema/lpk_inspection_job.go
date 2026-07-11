package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// LPKInspectionJob records an asynchronous external-LPK metadata inspection.
type LPKInspectionJob struct {
	ent.Schema
}

func (LPKInspectionJob) Fields() []ent.Field {
	return []ent.Field{
		field.Int("app_id"),
		field.Int("version_id").Optional().Nillable(),
		field.Int("user_id"),
		field.String("download_url").NotEmpty(),
		field.Enum("trigger").Values("API_TOKEN_FIRST_SUBMISSION", "MANUAL"),
		field.Enum("state").Values("PENDING", "RUNNING", "SUCCEEDED", "FAILED", "TIMED_OUT", "CANCELLED").Default("PENDING"),
		field.Bool("overwrite_existing_metadata").Default(false),
		field.Int("attempts").Default(0),
		field.Text("last_error").Optional().Nillable(),
		field.Time("next_attempt_at").Optional().Nillable(),
		field.Time("deadline_at").Optional().Nillable(),
		field.Time("completed_at").Optional().Nillable(),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (LPKInspectionJob) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("state", "next_attempt_at"),
		index.Fields("app_id", "state"),
		index.Fields("user_id", "created_at"),
	}
}
