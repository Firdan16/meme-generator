package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// LockUser serializes business operations for this user until the transaction finishes.
func LockUser(ctx context.Context, tx pgx.Tx, userID string) error {
	var id string
	if err := tx.QueryRow(ctx, `SELECT id FROM users WHERE id = $1 FOR UPDATE`, userID).Scan(&id); err != nil {
		return fmt.Errorf("lock user: %w", err)
	}
	return nil
}
