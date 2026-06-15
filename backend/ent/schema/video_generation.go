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

// VideoGeneration holds the schema definition for in-app (JWT) Veo video
// generation records.
//
// Unlike ImageGeneration (synchronous, conversation-threaded), Veo is an async
// long task: submit -> operation handle -> poll -> done. The MVP is flat (no
// conversations); each row is one submitted job tracked through read-time
// polling. operation_name + account_id are the handle the status endpoint uses
// to re-poll the bound upstream account and to proxy-stream the produced video.
type VideoGeneration struct {
	ent.Schema
}

func (VideoGeneration) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "video_generations"},
	}
}

func (VideoGeneration) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
		mixins.SoftDeleteMixin{},
	}
}

func (VideoGeneration) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.Int64("group_id"),
		field.String("prompt").
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("model").
			MaxLen(100).
			Default(""),
		// operation_name is the upstream Veo LRO handle (e.g.
		// "models/veo-3.x/operations/abc"). Empty until submit returns.
		field.String("operation_name").
			MaxLen(512).
			Default(""),
		// account_id is the Gemini account bound at submit time; the status
		// endpoint re-polls and the file proxy streams against this same account.
		field.Int64("account_id").
			Default(0),
		field.String("status").
			MaxLen(20).
			Default("pending"),
		// sample_count is how many video samples the completed operation produced.
		field.Int("sample_count").
			Default(0),
		// duration_seconds is the total billed video duration parsed on completion.
		field.Float("duration_seconds").
			Default(0),
		field.Float("cost").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}).
			Default(0),
		// billed guards against double-charging across repeated status polls of a
		// completed operation.
		field.Bool("billed").
			Default(false),
		field.String("error").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		// error_code is a stable machine-readable failure classifier the frontend
		// maps to a localized message; raw detail stays in `error`.
		field.String("error_code").
			MaxLen(40).
			Optional().
			Nillable(),
	}
}

func (VideoGeneration) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("deleted_at"),
		// Supports the stale-pending/processing sweep: WHERE status = ? AND created_at < cutoff.
		index.Fields("status", "created_at"),
	}
}
