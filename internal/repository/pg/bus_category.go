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
	Create(ctx context.Context, category model.BusCategory) error
	Update(ctx context.Context, category model.BusCategory) (model.BusCategory, error)
	Delete(ctx context.Context, id int) error
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
	query, args, err := b.qb.Sq.Select("id", "name").From(TableBusCategories).OrderBy("id").ToSql()
	if err != nil {
		return nil, err
	}
	result := make([]model.BusCategory, 0)
	if err := b.qb.DB.SelectContext(ctx, &result, query, args...); err != nil {
		return nil, err
	}
	return result, nil
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

func (b *busCategory) Create(ctx context.Context, category model.BusCategory) error {
	query, args, err := b.qb.Sq.Insert(TableBusCategories).
		Columns("name").
		Values(category.Name).
		ToSql()
	if err != nil {
		b.logger.Errorf("failed to create query: %v", err)
		return err
	}
	_, err = b.qb.DB.ExecContext(ctx, query, args...)
	if err != nil {
		b.logger.Errorf("failed to execute query, %v", err)
		return err
	}
	return nil
}

func (b *busCategory) Update(ctx context.Context, category model.BusCategory) (model.BusCategory, error) {
	query, args, err := b.qb.Sq.Update(TableBusCategories).
		Set("name", category.Name).
		Where(sq.Eq{"id": category.ID}).
		ToSql()
	if err != nil {
		b.logger.Errorf("failed to create query: %s \n err: %v", query, err)
		return model.BusCategory{}, err
	}

	err = execDomainMutation(ctx, b.qb, query, args...)
	if err != nil {
		b.logger.Errorf("failed to execute query: %s\nerror: %v", query, err)
		return model.BusCategory{}, err
	}

	return category, nil
}

func (b *busCategory) Delete(ctx context.Context, id int) error {
	query, args, err := b.qb.Sq.Delete(TableBusCategories).
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return err
	}

	err = execDomainMutation(ctx, b.qb, query, args...)
	return err
}
