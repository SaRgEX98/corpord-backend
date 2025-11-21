package pg

import (
	"corpord-api/internal/logger"
	"corpord-api/model"
	"corpord-api/pkg/dbx"
	sq "github.com/Masterminds/squirrel"
	"golang.org/x/net/context"
)

type Driver interface {
	All(ctx context.Context) ([]model.DriverOutput, error)
	ByID(ctx context.Context, id int) (model.DriverOutput, error)
	Create(ctx context.Context, driver model.DriverInput) error
	Update(ctx context.Context, driver model.DriverInput) error
	Delete(ctx context.Context, id int) error
}

type driver struct {
	logger *logger.Logger
	qb     *dbx.QueryBuilder
}

func NewDriver(logger *logger.Logger, qb *dbx.QueryBuilder) Driver {
	return &driver{
		logger: logger,
		qb:     qb,
	}
}

func (d *driver) All(ctx context.Context) ([]model.DriverOutput, error) {
	d.logger.Debug("All Repository")
	query, args, err := d.qb.Sq.Select(
		"first_name",
		"last_name",
		"middle_name",
		"phone_number",
		"ds.name as driver_status").
		From(TableDriver).
		Join("driver_status ds ON ds.id = drivers.status").
		ToSql()
	if err != nil {
		d.logger.Error("Failed to build query", err)
		return nil, err
	}
	var drivers []model.DriverOutput
	err = d.qb.DB.SelectContext(ctx, &drivers, query, args...)
	if err != nil {
		d.logger.Error("Failed to execute query", err)
		return nil, err
	}
	return drivers, nil
}

func (d *driver) ByID(ctx context.Context, id int) (model.DriverOutput, error) {
	query, args, err := d.qb.Sq.Select(
		"first_name",
		"last_name",
		"middle_name",
		"phone_number",
		"ds.name as driver_status").
		From(TableDriver).
		Join(`driver_status ds ON ds.id = drivers.status`).
		Where(sq.Eq{"drivers.id": id}).ToSql()
	if err != nil {
		d.logger.Error("Failed to build query", err)
		return model.DriverOutput{}, err
	}
	var result model.DriverOutput
	err = d.qb.DB.GetContext(ctx, &result, query, args...)
	if err != nil {
		d.logger.Error("Failed to execute query", err)
		return model.DriverOutput{}, err
	}
	return result, nil
}

func (d *driver) Create(ctx context.Context, driver model.DriverInput) error {
	query, args, err := d.qb.Sq.Insert(TableDriver).Columns(
		"first_name",
		"last_name",
		"middle_name",
		"phone_number",
		"status").
		Values(
			driver.FirstName,
			driver.LastName,
			driver.MiddleName,
			driver.PhoneNumber,
			driver.Status).
		ToSql()
	if err != nil {
		d.logger.Error("Failed to build query", err)
		return err
	}
	_, err = d.qb.DB.ExecContext(ctx, query, args...)
	if err != nil {
		d.logger.Error("Failed to execute query", err)
		return err
	}

	return nil
}

func (d *driver) Update(ctx context.Context, driver model.DriverInput) error {
	query, args, err := d.qb.Sq.Update(TableDriver).
		Set("first_name", driver.FirstName).
		Set("last_name", driver.LastName).
		Set("middle_name", driver.MiddleName).
		Set("phone_number", driver.PhoneNumber).
		Set("status", driver.Status).
		Where(sq.Eq{"id": driver.ID}).
		ToSql()
	if err != nil {
		d.logger.Error("Failed to build query", err)
		return err
	}

	_, err = d.qb.DB.ExecContext(ctx, query, args...)
	if err != nil {
		d.logger.Error("Failed to execute query", err)
		return err
	}
	return nil
}

func (d *driver) Delete(ctx context.Context, id int) error {
	query, args, err := d.qb.Sq.Delete(TableDriver).
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		d.logger.Error("Failed to build query", err)
		return err
	}
	_, err = d.qb.DB.ExecContext(ctx, query, args...)
	if err != nil {
		d.logger.Error("Failed to execute query", err)
		return err
	}
	return nil
}
