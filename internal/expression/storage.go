package expression

import "context"

type Storage interface {
	Create(ctx context.Context, expr Expression) (string, error)
	FindAll(ctx context.Context) ([]Expression, error)
	FindOne(ctx context.Context, id string) (Expression, error)
	Update(ctx context.Context, expr Expression) error
	Delete(ctx context.Context, id string) error
}
