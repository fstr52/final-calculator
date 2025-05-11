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
                id UUID PRIMARY KEY,
                user_id UUID REFERENCES users(id) ON DELETE CASCADE,
                root_operation_id TEXT,
                status VARCHAR(50),
                success BOOLEAN,
                result FLOAT,
                error TEXT,
                expression TEXT NOT NULL,
                created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        );

        CREATE TABLE IF NOT EXISTS operations (
                id UUID PRIMARY KEY,
                expr_id UUID REFERENCES expressions(id) ON DELETE CASCADE,
                left_operand UUID,
                right_operand UUID,
                operator VARCHAR(10),
                dependencies UUID[],
                result FLOAT,
                error TEXT,
                status VARCHAR(20),
                created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        );
        `

	_, err := client.Exec(ctx, schema)
	if err != nil {
		return fmt.Errorf("failed to initialize schema: %w", err)
	}
	return nil
}
