package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Wish struct{ ent.Schema }

func (Wish) Fields() []ent.Field {
	return []ent.Field{
		field.Enum("kind").Values("SUGGESTION", "APP_REQUEST", "CUSTOMIZATION"),
		field.Enum("status").Values("OPEN", "PLANNED", "IN_PROGRESS", "COMPLETED", "REJECTED").Default("OPEN"),
		field.String("title").NotEmpty(),
		field.Text("body").NotEmpty(),
		field.String("reference_url").Default(""),
		field.String("contact_email").Default(""),
		field.String("contact_other").Default(""),
		// ClientUserID is the stable, cross-device moderation identity.
		field.String("client_user_id").NotEmpty(),
		// OwnerID is scoped to the creating LazyCat device. Existing wishes
		// intentionally default to no owner because their original device is
		// not recoverable from historical data.
		field.String("owner_id").Default(""),
		field.String("author_name").Default(""),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
		field.Time("last_activity_at").Default(time.Now),
	}
}

func (Wish) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("kind", "status", "last_activity_at"),
		index.Fields("status", "last_activity_at"),
		index.Fields("client_user_id", "created_at"),
		index.Fields("owner_id", "created_at"),
	}
}
