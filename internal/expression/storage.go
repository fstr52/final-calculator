package expression

import "context"

type Storage interface {
	Create(ctx context.Context, expr Expression) error
	FindAll(ctx context.Context) ([]Expression, error)
	FindOne(ctx context.Context, id string) (Expression, error)
	FindAllByUser(ctx context.Context, userId string) (e []Expression, err error)
	Update(ctx context.Context, expr Expression) error
	Delete(ctx context.Context, id string) error
	GetUnfinishedExpressions(ctx context.Context) ([]*Expression, error)
}
