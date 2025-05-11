package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/fstr52/final-calculator/internal/db/postgresql"
	"github.com/fstr52/final-calculator/internal/expression"
	"github.com/fstr52/final-calculator/internal/logger"
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

func (s *storage) Create(ctx context.Context, expr expression.Expression) error {
	q := `
		INSERT INTO public.expressions (id, user_id, root_operation_id, status, expression)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err := s.client.Exec(ctx, q, expr.ID, expr.UserID, expr.RootOperationID, expr.Status, expr.Expression)
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

		s.logger.Error("Create expression error",
			"error", err)
		return err
	}

	return nil
}

func (s *storage) FindAll(ctx context.Context) (e []expression.Expression, err error) {
	q := `
		SELECT
			id,
			user_id,
			expression,
			status,
			success,
			error,
			result,
			created_at
		FROM public.expressions
	`
	rows, err := s.client.Query(ctx, q)
	if err != nil {
		return nil, err
	}

	exprs := make([]expression.Expression, 0)

	for rows.Next() {
		var expr expression.Expression

		err = rows.Scan(&expr.ID, &expr.UserID, &expr.Expression, &expr.Status, &expr.Success, &expr.Error, &expr.Result, &expr.CreatedAt)
		if err != nil {
			return nil, err
		}

		exprs = append(exprs, expr)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return exprs, nil
}

func (s *storage) FindAllByUser(ctx context.Context, userId string) (e []expression.Expression, err error) {
	q := `
		SELECT
			id,
			user_id,
			expression,
			status,
			success,
			error,
			result,
			created_at
		FROM public.expressions
		WHERE user_id = $1
	`

	rows, err := s.client.Query(ctx, q, userId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	exprs := make([]expression.Expression, 0)

	for rows.Next() {
		var expr expression.Expression

		err = rows.Scan(&expr.ID, &expr.UserID, &expr.Expression, &expr.Status, &expr.Success, &expr.Error, &expr.Result, &expr.CreatedAt)
		if err != nil {
			return nil, err
		}

		exprs = append(exprs, expr)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return exprs, nil
}

func (s *storage) FindOne(ctx context.Context, id string) (expression.Expression, error) {
	q := `
		SELECT
			id,
			user_id,
			expression,
			status,
			success,
			error,
			result,
			created_at
		FROM public.expressions
		WHERE id = $1
	`

	var expr expression.Expression

	err := s.client.QueryRow(ctx, q, id).Scan(&expr.ID, &expr.UserID, &expr.Expression, &expr.Status, &expr.Success, &expr.Error, &expr.Result, &expr.CreatedAt)
	if err != nil {
		return expression.Expression{}, err
	}

	return expr, nil
}

func (s *storage) Update(ctx context.Context, expr expression.Expression) error {
	q := `
		UPDATE public.expressions
		SET user_id = $1,
			expression = $2,
			status = $3,
			success = $4,
			error = $5,
			result = $6
		WHERE id = $7
	`

	result, err := s.client.Exec(ctx, q, expr.UserID, expr.Expression, expr.Status, expr.Success, expr.Error, expr.Result, expr.ID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("expression not found")
	}

	return nil
}

func (s *storage) Delete(ctx context.Context, id string) error {
	q := `
		DELETE
		FROM public.expressions
		WHERE id = $1
	`

	result, err := s.client.Exec(ctx, q, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("expression not found")
	}

	return nil
}

func (s *storage) GetUnfinishedExpressions(ctx context.Context) ([]*expression.Expression, error) {
	q := `
        SELECT
            id,
            user_id,
            root_operation_id,
            status,
            success,
            result,
            error,
            expression,
            created_at
		FROM public.expressions
		WHERE status NOT IN ('Done', 'Error')
    `

	rows, err := s.client.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("failed to query unfinished expressions: %w", err)
	}
	defer rows.Close()

	var expressions []*expression.Expression

	for rows.Next() {
		var expr expression.Expression
		err := rows.Scan(
			&expr.ID,
			&expr.UserID,
			&expr.RootOperationID,
			&expr.Status,
			&expr.Success,
			&expr.Result,
			&expr.Error,
			&expr.Expression,
			&expr.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan expression row: %w", err)
		}

		expressions = append(expressions, &expr)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return expressions, nil
}
