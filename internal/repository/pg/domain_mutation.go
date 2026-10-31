package pg

import (
	"context"
	"corpord-api/pkg/dbx"
)

func execDomainMutation(ctx context.Context, qb *dbx.QueryBuilder, query string, args ...any) error {
	result, err := qb.DB.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}
