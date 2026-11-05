package booking

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

func (s *Service) Passengers(ctx context.Context, owner, limit int, cursor string) (Page[Passenger], error) {
	out := Page[Passenger]{Items: []Passenger{}}
	after, limit, err := pagePosition(owner, "passengers", cursor, limit)
	if err != nil {
		return out, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	q := `SELECT ` + passengerColumns + ` FROM booking_passengers WHERE user_id=$1`
	args := []any{owner, limit + 1}
	if after > 0 {
		q += ` AND id<$3`
		args = append(args, after)
	}
	q += ` ORDER BY id DESC LIMIT $2`
	if err = s.DB.SelectContext(ctx, &out.Items, q, args...); err != nil {
		return out, err
	}
	if len(out.Items) > limit {
		out.Items = out.Items[:limit]
		out.NextCursor = pageCursor(owner, "passengers", out.Items[limit-1].ID)
	}
	return out, nil
}
func (s *Service) SavePassenger(ctx context.Context, owner int, id int64, name string) (Passenger, error) {
	var out Passenger
	name, err := NormalizeName(name)
	if err != nil {
		return out, err
	}
	if owner < 1 || id < 0 {
		return out, invalid()
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := s.DB.BeginTxx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	if err = lockOwner(ctx, tx, owner); err != nil {
		return out, err
	}
	if id == 0 {
		err = tx.GetContext(ctx, &out, `INSERT INTO booking_passengers(user_id,full_name) VALUES($1,$2) RETURNING `+passengerColumns, owner, name)
	} else {
		err = tx.GetContext(ctx, &out, `UPDATE booking_passengers SET full_name=$3,updated_at=clock_timestamp() WHERE id=$1 AND user_id=$2 RETURNING `+passengerColumns, id, owner, name)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return out, missing()
	}
	if err != nil {
		return out, err
	}
	return out, tx.Commit()
}
func (s *Service) DeletePassenger(ctx context.Context, owner int, id int64) error {
	if owner < 1 || id < 1 {
		return invalid()
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := s.DB.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = lockOwner(ctx, tx, owner); err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM booking_passengers WHERE id=$1 AND user_id=$2`, id, owner)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return missing()
	}
	return tx.Commit()
}
