package inverseschema

import (
	"context"
)

func NewSchema(adapter Adapter) *Schema {
	return &Schema{adapter: adapter}
}

type Schema struct {
	adapter Adapter
	Tables  []Table
	Enums   []Enum
}

func (s *Schema) Parse() error {
	return s.ParseContext(context.Background())
}
func (s *Schema) ParseContext(ctx context.Context) error {
	var err error
	s.Tables, err = s.adapter.Tables(ctx)
	if err != nil {
		return err
	}
	s.Enums, err = s.adapter.Enums(ctx)
	if err != nil {
		return err
	}
	return nil
}
