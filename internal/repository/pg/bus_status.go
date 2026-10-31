package pg

import (
	"context"
	"corpord-api/internal/logger"
	"corpord-api/model"
	"corpord-api/pkg/dbx"
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
	query, args, err := b.qb.Sq.Select("id", "name").From(TableBusStatuses).OrderBy("id").ToSql()
	if err != nil {
		return nil, err
	}
	result := make([]model.BusStatus, 0)
	if err := b.qb.DB.SelectContext(ctx, &result, query, args...); err != nil {
		return nil, err
	}
	return result, nil
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

	err = execDomainMutation(ctx, b.qb, query, args...)
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

	err = execDomainMutation(ctx, b.qb, query, args...)
	return err
}
