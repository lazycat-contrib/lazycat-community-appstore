package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type ClientSyncSetting struct {
	ent.Schema
}

func (ClientSyncSetting) Fields() []ent.Field {
	return []ent.Field{
		field.String("user_id").NotEmpty(),
		field.Bool("auto_sync_enabled").Default(false),
		field.Int("auto_sync_interval_minutes").Default(60),
		field.Bool("sync_on_startup").Default(false),
		field.Time("last_auto_sync_at").Optional().Nillable(),
		field.Enum("last_auto_sync_status").Values("success", "partial", "failed").Optional().Nillable(),
		field.Text("last_auto_sync_error").Optional().Nillable(),
		field.Time("created_at").Default(time.Now),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (ClientSyncSetting) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id").Unique(),
		index.Fields("auto_sync_enabled", "last_auto_sync_at"),
		index.Fields("sync_on_startup"),
	}
}
