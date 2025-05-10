package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/fstr52/final-calculator/internal/db/postgresql"
	"github.com/fstr52/final-calculator/internal/logger"
	"github.com/fstr52/final-calculator/internal/user"
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

func (r *storage) Create(ctx context.Context, user user.User) (string, error) {
	q := `
		INSERT INTO public.users (username, password_hash)
		VALUES ($1, $2)
		RETURNING id
	`
	if err := r.client.QueryRow(ctx, q, user.Username, user.PasswordHash).Scan(&user.ID); err != nil {
		var pgErr *pgconn.PgError
		if errors.Is(err, pgErr) {
			pgErr = err.(*pgconn.PgError)
			r.logger.Error("SQL Error",
				"error", pgErr.Message,
				"detail", pgErr.Detail,
				"where", pgErr.Where,
				"code", pgErr.Code,
				"SQLState", pgErr.SQLState())
			return "", fmt.Errorf("SQL error: %v", err)
		} else {
			r.logger.Error("Create error",
				"error", err)
			return "", err
		}
	}

	return user.ID, nil
}

func (r *storage) FindAll(ctx context.Context) (u []user.User, err error) {
	q := `
		SELECT
			id,
			name,
			password
		FROM public.users
	`
	rows, err := r.client.Query(ctx, q)
	if err != nil {
		return nil, err
	}

	users := make([]user.User, 0)

	for rows.Next() {
		var usr user.User

		err = rows.Scan(&usr.ID, &usr.Username, &usr.PasswordHash)
		if err != nil {
			return nil, err
		}

		users = append(users, usr)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

func (r *storage) FindOne(ctx context.Context, id string) (user.User, error) {
	q := `
		SELECT
			id,
			username,
			password_hash
		FROM public.users
		WHERE id = $1
	`

	var usr user.User

	err := r.client.QueryRow(ctx, q, id).Scan(&usr.ID, &usr.Username, &usr.PasswordHash)
	if err != nil {
		return user.User{}, err
	}

	return usr, nil
}

func (r *storage) FindByUsername(ctx context.Context, username string) (user.User, error) {
	q := `
		SELECT
			id,
			username,
			password_hash
		FROM public.users
		WHERE username = $1
	`

	var usr user.User

	err := r.client.QueryRow(ctx, q, username).Scan(&usr.ID, &usr.Username, &usr.PasswordHash)
	if err != nil {
		return user.User{}, err
	}

	return usr, nil
}

func (r *storage) Update(ctx context.Context, user user.User) error {
	q := `
		UPDATE public.users
		SET username = $1,
			password_hash = $2
		WHERE id = $3
	`

	result, err := r.client.Exec(ctx, q, user.Username, user.PasswordHash, user.ID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

func (r *storage) Delete(ctx context.Context, id string) error {
	q := `
		DELETE
		FROM public.users
		WHERE id = $1
	`

	result, err := r.client.Exec(ctx, q, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}
