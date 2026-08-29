package entity

import (
	"context"
	"fmt"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/dbal"
)

type Service struct{ DB dbal.Database }

func (s Service) Load(ctx context.Context, e appir.Entity, id string) (dbal.Row, error) {
	rows, err := s.DB.Select(ctx, dbal.Select{Table: e.Name, Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: id}, Limit: 1})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, &dbal.Error{Code: dbal.NotFound, Message: fmt.Sprintf("%s not found", e.Label)}
	}
	return rows[0], nil
}
