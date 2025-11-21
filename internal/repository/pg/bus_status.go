package pg

import (
	"context"
	"corpord-api/internal/logger"
	"corpord-api/model"
	"corpord-api/pkg/dbx"
	"database/sql"
	sq "github.com/Masterminds/squirrel"
)

type BusStatus interface {
	All(ctx context.Context) ([]model.BusStatus, error)
	ByID(ctx context.Context, id int) (model.BusStatus, error)
	Create(ctx context.Context, status model.BusStatus) error
	Update(ctx context.Context, status model.BusStatus) (model.BusStatus, error)
	Delete(ctx context.Context, id int) error
}

type busStatus struct {
	logger *logger.Logger
	qb     *dbx.QueryBuilder
}

func NewBusStatus(logger *logger.Logger, qb *dbx.QueryBuilder) BusStatus {
	return &busStatus{
		logger: logger,
		qb:     qb,
	}
}

func (b *busStatus) All(ctx context.Context) ([]model.BusStatus, error) {
	query, args, err := b.qb.Sq.Select("*").
		From(TableBusStatuses).
		ToSql()
	if err != nil {
		b.logger.Error("Failed to build select bus category query", "error", err)
		return nil, err
	}

	rows, err := b.qb.DB.QueryContext(ctx, query, args...)
	if err != nil {
		b.logger.Error("Failed to select bus category", "error", err)
		return nil, err
	}

	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	var busCategories []model.BusStatus
	for rows.Next() {
		var busStatus model.BusStatus
		if err := rows.Scan(&busStatus.ID, &busStatus.Name); err != nil {
			b.logger.Error("Failed to scan bus category", "error", err)
			return nil, err
		}
		busCategories = append(busCategories, busStatus)
	}

	return busCategories, nil
}

func (b *busStatus) ByID(ctx context.Context, id int) (model.BusStatus, error) {
	query, args, err := b.qb.Sq.Select("*").
		From(TableBusStatuses).
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		b.logger.Error("Failed to build select bus category query", "error", err)
		return model.BusStatus{}, err
	}

	var busStatus model.BusStatus
	err = b.qb.DB.GetContext(ctx, &busStatus, query, args...)
	if err != nil {
		b.logger.Error("Failed to select bus category", "error", err)
		return model.BusStatus{}, err
	}

	return busStatus, nil
}

func (b *busStatus) Create(ctx context.Context, status model.BusStatus) error {
	query, args, err := b.qb.Sq.Insert(TableBusStatuses).
		Columns("name").
		Values(status.Name).
		ToSql()
	if err != nil {
		b.logger.Error("Failed to build insert bus category query", "error", err)
		return err
	}

	_, err = b.qb.DB.ExecContext(ctx, query, args...)
	if err != nil {
		b.logger.Error("Failed to insert bus category", "error", err)
		return err
	}

	return nil
}

func (b *busStatus) Update(ctx context.Context, status model.BusStatus) (model.BusStatus, error) {
	query, args, err := b.qb.Sq.Update(TableBusStatuses).
		Set("name", status.Name).
		Where(sq.Eq{"id": status.ID}).
		ToSql()
	if err != nil {
		b.logger.Error("Failed to build update bus category query", "error", err)
		return model.BusStatus{}, err
	}

	_, err = b.qb.DB.ExecContext(ctx, query, args...)
	if err != nil {
		b.logger.Error("Failed to update bus category", "error", err)
		return model.BusStatus{}, err
	}

	return status, nil
}

func (b *busStatus) Delete(ctx context.Context, id int) error {
	query, args, err := b.qb.Sq.Delete(TableBusStatuses).
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return err
	}

	_, err = b.qb.DB.ExecContext(ctx, query, args...)
	return err
}
