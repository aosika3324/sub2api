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

// ImageGeneration holds the schema definition for image generation records.
type ImageGeneration struct {
	ent.Schema
}

func (ImageGeneration) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "image_generations"},
	}
}

func (ImageGeneration) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
		mixins.SoftDeleteMixin{},
	}
}

func (ImageGeneration) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.Int64("conversation_id"),
		field.Int64("group_id"),
		field.String("prompt").
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("model").
			MaxLen(100).
			Default(""),
		field.String("size").
			MaxLen(30).
			Default(""),
		field.String("quality").
			MaxLen(30).
			Default(""),
		field.Int("n").
			Default(1),
		field.Int("image_count").
			Default(0),
		field.String("status").
			MaxLen(20).
			Default("pending"),
		field.Float("cost").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}).
			Default(0),
		field.JSON("storage_keys", []string{}).
			Optional().
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.JSON("input_storage_keys", []string{}).
			Optional().
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.Int("width").
			Optional().
			Nillable(),
		field.Int("height").
			Optional().
			Nillable(),
		field.String("error").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		// error_code is a stable machine-readable failure classifier
		// (no_account/no_images/content_blocked/interrupted/upstream_error/busy)
		// the frontend maps to a localized message. The raw, human-facing detail
		// stays in `error` for admin diagnostics.
		field.String("error_code").
			MaxLen(40).
			Optional().
			Nillable(),
	}
}

func (ImageGeneration) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("conversation_id"),
		index.Fields("deleted_at"),
		// Supports the stale-pending sweep: WHERE status = 'pending' AND created_at < cutoff.
		index.Fields("status", "created_at"),
	}
}
