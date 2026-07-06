package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type ClientSourceApp struct {
	ent.Schema
}

func (ClientSourceApp) Fields() []ent.Field {
	return []ent.Field{
		field.Int("source_id"),
		field.String("external_id").Default(""),
		field.String("package_id").NotEmpty(),
		field.String("name").NotEmpty(),
		field.String("slug").NotEmpty(),
		field.String("summary").Default(""),
		field.String("category").Default(""),
		field.Text("category_i18n_json").Default("{}"),
		field.String("icon_url").Default(""),
		field.Bool("install_protected").Default(false),
		field.Text("screenshots_json").Default(""),
		field.Text("latest_version_json").Default(""),
		field.Text("versions_json").Default(""),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (ClientSourceApp) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("source", ClientSource.Type).
			Ref("apps").
			Field("source_id").
			Required().
			Unique(),
	}
}

func (ClientSourceApp) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("source_id", "package_id").Unique(),
		index.Fields("source_id", "slug"),
		index.Fields("source_id", "updated_at"),
	}
}
