package pg

import (
	"context"
	"corpord-api/internal/logger"
	"corpord-api/model"
	"corpord-api/pkg/dbx"
	sq "github.com/Masterminds/squirrel"
)

type BusCategory interface {
	GetAll(ctx context.Context) ([]model.BusCategory, error)
	GetById(ctx context.Context, id int) (*model.BusCategory, error)
	Create()
	Update()
	Delete()
}

type busCategory struct {
	qb     *dbx.QueryBuilder
	logger *logger.Logger
}

func NewBusCategory(logger *logger.Logger, qb *dbx.QueryBuilder) BusCategory {
	return &busCategory{
		qb:     qb,
		logger: logger,
	}
}

func (b *busCategory) GetAll(ctx context.Context) ([]model.BusCategory, error) {
	query, args, err := b.qb.Sq.Select("*").
		From(TableBusCategories).
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

	defer rows.Close()

	var busCategories []model.BusCategory
	for rows.Next() {
		var busCategory model.BusCategory
		if err := rows.Scan(&busCategory.ID, &busCategory.Name); err != nil {
			b.logger.Error("Failed to scan bus category", "error", err)
			return nil, err
		}
		busCategories = append(busCategories, busCategory)
	}

	return busCategories, nil
}

func (b *busCategory) GetById(ctx context.Context, id int) (*model.BusCategory, error) {
	query, args, err := b.qb.Sq.Select("*").
		From(TableBusCategories).
		Where(sq.Eq{"id": id}).
		ToSql()

	if err != nil {
		b.logger.Error("Failed to build select bus category query", "error", err)
		return nil, err
	}

	var busCategory model.BusCategory
	err = b.qb.DB.GetContext(ctx, &busCategory, query, args...)
	if err != nil {
		b.logger.Error("Failed to select bus category", "error", err)
		return nil, err
	}

	return &busCategory, nil
}

func (b *busCategory) Create() {
	//TODO implement me
	panic("implement me")
}

func (b *busCategory) Update() {
	//TODO implement me
	panic("implement me")
}

func (b *busCategory) Delete() {
	//TODO implement me
	panic("implement me")
}
