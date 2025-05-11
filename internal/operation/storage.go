package operation

import "context"

type Storage interface {
	Create(ctx context.Context, op Operation) error
	FindOne(ctx context.Context, id string) (Operation, error)
	Update(ctx context.Context, op Operation) error
	GetOperationsByExpression(ctx context.Context, exprID string) ([]Operation, error)
	UpdateOperationResult(ctx context.Context, id string, result float32, status string) error
}
