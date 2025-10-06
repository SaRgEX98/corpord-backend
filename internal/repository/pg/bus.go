package pg

import (
	"context"
	"corpord-api/internal/logger"
	"corpord-api/model"
	"corpord-api/pkg/dbx"
	"time"

	sq "github.com/Masterminds/squirrel"
)

type BusRepository interface {
	CreateBus(ctx context.Context, bus model.Bus) error
	GetBus(ctx context.Context, id int) (*model.Bus, error)
	GetAllBuses(ctx context.Context) ([]model.Bus, error)
	UpdateBus(ctx context.Context, bus *model.BusUpdate) error
	DeleteBus(ctx context.Context, id int) error
}

type busRepository struct {
	qb     *dbx.QueryBuilder
	logger *logger.Logger
}

func NewBusRepository(logger *logger.Logger, qb *dbx.QueryBuilder) BusRepository {
	return &busRepository{
		logger: logger,
		qb:     qb,
	}
}

func (b *busRepository) CreateBus(ctx context.Context, bus model.Bus) error {
	query, args, err := b.qb.Sq.Insert("bus").
		Columns("license_plate", "brand", "capacity", "category_id", "status_id", "created_at", "updated_at").
		Values(
			bus.LicensePlate,
			bus.Brand,
			bus.Capacity,
			bus.CategoryID,
			bus.StatusID,
			bus.CreatedAt,
			bus.UpdatedAt,
		).
		ToSql()

	if err != nil {
		b.logger.Error("Failed to build create bus query", "error", err)
		return err
	}

	_, err = b.qb.DB.ExecContext(ctx, query, args...)
	if err != nil {
		b.logger.Error("Failed to create bus", "error", err, "license_plate", bus.LicensePlate)
		return ErrAlreadyExists
	}

	return nil
}

func (b *busRepository) GetBus(ctx context.Context, id int) (*model.Bus, error) {
	query, args, err := b.qb.Sq.Select("buses.*").
		From("bus").
		Where(sq.Eq{"buses.id": id}).
		ToSql()

	if err != nil {
		b.logger.Error("Failed to build get bus query", "error", err)
		return nil, err
	}

	var bus model.Bus
	err = b.qb.DB.GetContext(ctx, &bus, query, args...)
	if err != nil {
		b.logger.Error("Failed to get bus", "error", err, "id", id)
		return nil, ErrNotFound
	}

	return &bus, nil
}

func (b *busRepository) GetAllBuses(ctx context.Context) ([]model.Bus, error) {
	query, args, err := b.qb.Sq.Select("bus.*").
		From("bus").
		ToSql()

	if err != nil {
		b.logger.Error("Failed to build get all buses query", "error", err)
		return nil, err
	}

	var buses []model.Bus
	err = b.qb.DB.SelectContext(ctx, &buses, query, args...)
	if err != nil {
		b.logger.Error("Failed to get all buses", "error", err)
		return nil, ErrNotFound
	}

	return buses, nil
}

func (b *busRepository) UpdateBus(ctx context.Context, bus *model.BusUpdate) error {
	query, args, err := b.qb.Sq.Update("bus").
		Set("license_plate", bus.LicensePlate).
		Set("brand", bus.Brand).
		Set("capacity", bus.Capacity).
		Set("category_id", bus.CategoryID).
		Set("status_id", bus.StatusID).
		Set("updated_at", time.Now()).
		Where(sq.Eq{"bus.id": bus.ID}).
		ToSql()

	if err != nil {
		b.logger.Error("Failed to build update bus query", "error", err)
		return err
	}

	_, err = b.qb.DB.ExecContext(ctx, query, args...)
	if err != nil {
		b.logger.Error("Failed to update bus", "error", err, "id", bus.ID)
		return ErrNotFound
	}

	return nil
}

func (b *busRepository) DeleteBus(ctx context.Context, id int) error {
	query, args, err := b.qb.Sq.Delete("bus").
		Where(sq.Eq{"bus.id": id}).
		ToSql()

	if err != nil {
		b.logger.Error("Failed to build delete bus query", "error", err)
		return err
	}

	_, err = b.qb.DB.ExecContext(ctx, query, args...)
	if err != nil {
		b.logger.Error("Failed to delete bus", "error", err, "id", id)
		return ErrNotFound
	}

	return nil
}
