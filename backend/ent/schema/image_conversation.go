package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ImageConversation holds the schema definition for studio conversation threads.
type ImageConversation struct {
	ent.Schema
}

func (ImageConversation) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "image_conversations"},
	}
}

func (ImageConversation) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
		mixins.SoftDeleteMixin{},
	}
}

func (ImageConversation) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.String("title").
			MaxLen(255).
			Default(""),
	}
}

func (ImageConversation) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("deleted_at"),
	}
}
