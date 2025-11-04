package pg

import (
	"context"
	"corpord-api/internal/logger"
	"corpord-api/model"
	"corpord-api/pkg/dbx"
	sq "github.com/Masterminds/squirrel"
)

type DriverStatus interface {
	All(ctx context.Context) []model.DriverStatus
	ById(ctx context.Context, id int) (model.DriverStatus, error)
	Create(ctx context.Context, status *model.DriverStatus) error
	Update(ctx context.Context, status *model.DriverStatus) error
	Delete(ctx context.Context, id int) error
}

type driverStatus struct {
	logger *logger.Logger
	qb     *dbx.QueryBuilder
}

func NewDriverStatus(logger *logger.Logger, qb *dbx.QueryBuilder) DriverStatus {
	return &driverStatus{
		logger: logger,
		qb:     qb,
	}
}

func (d *driverStatus) All(ctx context.Context) []model.DriverStatus {
	var results []model.DriverStatus
	query, args, err := d.qb.Sq.Select("id, name").From(TableDriverStatus).ToSql()
	if err != nil {
		d.logger.Error(err)
		return []model.DriverStatus{}
	}
	err = d.qb.DB.SelectContext(ctx, &results, query, args...)
	if err != nil {
		d.logger.Error(err)
		return []model.DriverStatus{}
	}
	return results
}

func (d *driverStatus) ById(ctx context.Context, id int) (model.DriverStatus, error) {
	query, args, err := d.qb.Sq.Select("id, name").
		From(TableDriverStatus).
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		d.logger.Error(err)
		return model.DriverStatus{}, err
	}
	var status model.DriverStatus
	err = d.qb.DB.GetContext(ctx, &status, query, args...)
	if err != nil {
		d.logger.Error(err)
		return model.DriverStatus{}, err
	}
	return status, nil
}

func (d *driverStatus) Create(ctx context.Context, status *model.DriverStatus) error {
	query, args, err := d.qb.Sq.Insert(TableDriverStatus).
		Columns("name").
		Values(status.Name).
		ToSql()
	if err != nil {
		d.logger.Error(err)
		return err
	}
	_, err = d.qb.DB.ExecContext(ctx, query, args...)
	if err != nil {
		d.logger.Error(err)
		return err
	}
	return nil
}

func (d *driverStatus) Update(ctx context.Context, status *model.DriverStatus) error {
	query, args, err := d.qb.Sq.Update(TableDriverStatus).
		Set("name", status.Name).
		Where(sq.Eq{"id": status.ID}).
		ToSql()
	if err != nil {
		d.logger.Error(err)
		return err
	}
	_, err = d.qb.DB.ExecContext(ctx, query, args...)
	if err != nil {
		d.logger.Error(err)
		return err
	}
	return nil
}

func (d *driverStatus) Delete(ctx context.Context, id int) error {
	query, args, err := d.qb.Sq.Delete(TableDriverStatus).Where(sq.Eq{"id": id}).ToSql()
	if err != nil {
		d.logger.Error(err)
		return err
	}
	_, err = d.qb.DB.ExecContext(ctx, query, args...)
	if err != nil {
		d.logger.Error(err)
	}
	return err
}
