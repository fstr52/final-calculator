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

func NewStorage(client postgresql.Client) *storage {
	return &storage{
		client: client,
	}
}

func (s *storage) Create(ctx context.Context, expr expression.Expression) (string, error) {
	q := `
		INSERT INTO public.expressions (user_id, expression, status)
		VALUES ($1, $2, $3)
		RETURNING id
	`

	if err := s.client.QueryRow(ctx, q, expr.UserID, expr.Expression, expr.Status).Scan(&expr.ID); err != nil {
		var pgErr *pgconn.PgError
		if errors.Is(err, pgErr) {
			pgErr = err.(*pgconn.PgError)
			s.logger.Error("SQL Error",
				"error", pgErr.Message,
				"detail", pgErr.Detail,
				"where", pgErr.Where,
				"code", pgErr.Code,
				"SQLState", pgErr.SQLState())
			return "", nil
		} else {
			s.logger.Error("Create error",
				"error", err)
			return "", err
		}
	}

	return expr.ID, nil
}

func (s *storage) FindAll(ctx context.Context) (e []expression.Expression, err error) {
	q := `
		SELECT
			id,
			user_id,
			expression,
			status,
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

		err = rows.Scan(&expr.ID, &expr.UserID, &expr.Expression, &expr.Status, &expr.Error, &expr.Result, &expr.CreatedAt)
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

	exprs := make([]expression.Expression, 0)

	for rows.Next() {
		var expr expression.Expression

		err = rows.Scan(&expr.ID, &expr.UserID, &expr.Expression, &expr.Status, &expr.Error, &expr.Result, &expr.CreatedAt)
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
			error,
			result,
			created_at
		FROM public.expressions
		WHERE id = $1
	`

	var expr expression.Expression

	err := s.client.QueryRow(ctx, q).Scan(&expr)
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
			error = $4,
			result = %5,
		WHERE id = $6
	`

	result, err := s.client.Exec(ctx, q, expr.UserID, expr.Expression, expr.Status, expr.Error, expr.Result, expr.ID)
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
