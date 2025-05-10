package postgresql

import (
	"context"
	"fmt"
)

func InitSchema(ctx context.Context, client Client) error {
	schema := `
        CREATE EXTENSION IF NOT EXISTS "pgcrypto";

        CREATE TABLE IF NOT EXISTS users (
                id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                username VARCHAR(255) UNIQUE NOT NULL,
                password_hash TEXT NOT NULL
        );

        CREATE TABLE IF NOT EXISTS expressions (
                id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
                user_id UUID REFERENCES users(id) ON DELETE CASCADE,
                expression TEXT NOT NULL,
                status VARCHAR(50),
                error TEXT,
                result FLOAT,
                created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        );
        `

	_, err := client.Exec(ctx, schema)
	if err != nil {
		return fmt.Errorf("failed to initialize schema: %w", err)
	}
	return nil
}
