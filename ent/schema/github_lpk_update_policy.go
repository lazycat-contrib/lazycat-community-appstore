package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// GitHubLPKUpdatePolicy stores one GitHub Release LPK update schedule per app.
type GitHubLPKUpdatePolicy struct {
	ent.Schema
}

func (GitHubLPKUpdatePolicy) Annotations() []entschema.Annotation {
	return []entschema.Annotation{
		entsql.Annotation{Table: "github_lpk_update_policies"},
	}
}

func (GitHubLPKUpdatePolicy) Fields() []ent.Field {
	return []ent.Field{
		field.Int("app_id").Positive().Unique(),
		field.Bool("enabled").Default(false),
		field.Int("interval_minutes").Default(24*60).Range(60, 30*24*60),
		field.Time("last_checked_at").Optional().Nillable(),
		field.Time("last_success_at").Optional().Nillable(),
		field.Time("next_check_at").Optional().Nillable(),
		field.String("last_version").Default(""),
		field.Text("last_error").Default(""),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (GitHubLPKUpdatePolicy) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("app", App.Type).
			Ref("github_lpk_update_policy").
			Field("app_id").
			Unique().
			Required().
			Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

func (GitHubLPKUpdatePolicy) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("enabled", "next_check_at"),
	}
}
