package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

// Greet holds the schema definition for the Greet entity.
type Greet struct{ ent.Schema }

// Fields of the Greet.
func (Greet) Fields() []ent.Field {
	return []ent.Field{
		field.String("content").NotEmpty(),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

// Edges of the Greet.
func (Greet) Edges() []ent.Edge {
	return nil
}
