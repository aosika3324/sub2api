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

// EditableFileArtifact stores files produced by editable PPT/PSD tasks.
type EditableFileArtifact struct {
	ent.Schema
}

func (EditableFileArtifact) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "editable_file_artifacts"},
	}
}

func (EditableFileArtifact) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
		mixins.SoftDeleteMixin{},
	}
}

func (EditableFileArtifact) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("task_id"),
		field.Int64("user_id"),
		field.String("kind").
			MaxLen(20),
		field.String("file_name").
			MaxLen(255),
		field.String("mime_type").
			MaxLen(120).
			Default(""),
		field.Int64("size_bytes").
			Default(0),
		field.String("storage_key").
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("source_pointer").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("sha256").
			MaxLen(64).
			Default(""),
	}
}

func (EditableFileArtifact) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("task_id"),
		index.Fields("user_id"),
		index.Fields("deleted_at"),
	}
}
