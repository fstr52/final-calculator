package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/fstr52/final-calculator/internal/db/postgresql"
	"github.com/fstr52/final-calculator/internal/logger"
	"github.com/fstr52/final-calculator/internal/operation"
	"github.com/jackc/pgconn"
)

type storage struct {
	logger logger.Logger
	client postgresql.Client
}

func NewStorage(client postgresql.Client, logger logger.Logger) *storage {
	return &storage{
		logger: logger,
		client: client,
	}
}

func (s *storage) Create(ctx context.Context, op operation.Operation) error {
	q := `
		INSERT INTO public.operations (id, expr_id, left_operand, right_operand, operator, dependencies, status, operation_time)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := s.client.Exec(ctx, q, op.ID, op.ExprID, op.Left, op.Right, op.Operator, op.Dependencies, op.Status, op.OperationTime)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			s.logger.Error("SQL Error",
				"message", pgErr.Message,
				"detail", pgErr.Detail,
				"where", pgErr.Where,
				"code", pgErr.Code,
				"SQLState", pgErr.SQLState())
			return fmt.Errorf("sql error: %w", pgErr)
		}

		s.logger.Error("Create operation error",
			"error", err)
		return err
	}

	return nil
}

func (s *storage) FindOne(ctx context.Context, id string) (operation.Operation, error) {
	q := `
		SELECT
			id,
			expr_id,
			left_operand,
			right_operand,
			operator,
			dependencies,
			result,
			error,
			status,
			operation_time
		FROM public.operations
		WHERE expr_id = $1
	`

	var op operation.Operation

	err := s.client.QueryRow(ctx, q, id).Scan(
		&op.ID,
		&op.ExprID,
		&op.Left,
		&op.Right,
		&op.Operator,
		&op.Dependencies,
		&op.Result,
		&op.Error,
		&op.Status,
		&op.OperationTime,
	)

	if err != nil {
		return operation.Operation{}, err
	}

	return op, nil
}

func (s *storage) Update(ctx context.Context, op operation.Operation) error {
	q := `
		UPDATE public.operations
		SET expr_id = $1,
			left_operand = $2,
			right_operand = $3,
			operator = $4,
			dependencies = $5,
			result = $6,
			status = $7,
			operation_time = $8
		WHERE id = $9
	`

	result, err := s.client.Exec(ctx, q,
		op.ExprID,
		op.Left,
		op.Right,
		op.Operator,
		op.Dependencies,
		op.Result,
		op.Status,
		op.OperationTime,
		op.ID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("operation not found")
	}

	return nil
}

func (s *storage) GetOperationsByExpression(ctx context.Context, exprID string) (e []operation.Operation, err error) {
	q := `
		SELECT
			id,
			expr_id,
			left_operand,
			right_operand,
			operator,
			dependencies,
			result,
			error,
			status,
			operation_time
		FROM public.operations
		WHERE expr_id = $1
	`

	rows, err := s.client.Query(ctx, q, exprID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ops := make([]operation.Operation, 0)

	for rows.Next() {
		var op operation.Operation

		err = rows.Scan(
			&op.ID,
			&op.ExprID,
			&op.Left,
			&op.Right,
			&op.Operator,
			&op.Dependencies,
			&op.Result,
			&op.Error,
			&op.Status,
			&op.OperationTime,
		)

		if err != nil {
			return nil, err
		}

		ops = append(ops, op)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return ops, nil
}

func (s *storage) UpdateOperationResult(ctx context.Context, id string, result float64, status string) error {
	q := `
        UPDATE public.operations
        SET result = $1,
            status = $2
        WHERE id = $3
    `

	cmd, err := s.client.Exec(ctx, q, result, status, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			s.logger.Error("SQL Error",
				"message", pgErr.Message,
				"detail", pgErr.Detail,
				"where", pgErr.Where,
				"code", pgErr.Code,
				"SQLState", pgErr.SQLState())
			return fmt.Errorf("sql error: %w", pgErr)
		}

		s.logger.Error("Update operation result error", "error", err)
		return err
	}

	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("operation not found: %s", id)
	}

	return nil
}
