package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// EditableFileTask holds asynchronous PPT/PSD export task records.
type EditableFileTask struct {
	ent.Schema
}

func (EditableFileTask) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "editable_file_tasks"},
	}
}

func (EditableFileTask) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
		mixins.SoftDeleteMixin{},
	}
}

func (EditableFileTask) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.Int64("group_id").
			Optional().
			Nillable(),
		field.Int64("api_key_id").
			Optional().
			Nillable(),
		field.Int64("image_conversation_id").
			Optional().
			Nillable(),
		field.Int64("source_generation_id").
			Optional().
			Nillable(),
		field.Int64("account_id").
			Optional().
			Nillable(),
		field.String("kind").
			MaxLen(20),
		field.String("status").
			MaxLen(20).
			Default("queued"),
		field.String("prompt").
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("model").
			MaxLen(100).
			Default(""),
		field.String("client_task_id").
			MaxLen(128).
			Default(""),
		field.String("chatgpt_conversation_id").
			MaxLen(128).
			Default(""),
		field.Float("cost").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}).
			Default(0),
		field.String("error").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Time("expires_at").
			Optional().
			Nillable(),
	}
}

func (EditableFileTask) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("status"),
		index.Fields("client_task_id"),
		index.Fields("deleted_at"),
	}
}
