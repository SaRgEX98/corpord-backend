package pg

import (
	"corpord-api/internal/logger"
	"corpord-api/model"
	"corpord-api/pkg/dbx"
	sq "github.com/Masterminds/squirrel"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

type Stop interface {
	All(ctx context.Context) ([]*model.Stop, error)
	ByID(ctx context.Context, id int) (*model.Stop, error)
	Create(ctx context.Context, stop *model.Stop) error
	Update(ctx context.Context, stop *model.StopUpdate) error
	Delete(ctx context.Context, id int) error
}

type stop struct {
	logger *logger.Logger
	qb     *dbx.QueryBuilder
}

func NewStop(logger *logger.Logger, qb *dbx.QueryBuilder) Stop {
	return &stop{
		logger: logger,
		qb:     qb,
	}
}

func (s *stop) All(ctx context.Context) ([]*model.Stop, error) {
	var stops []*model.Stop
	query, args, err := s.qb.Sq.Select(
		"name",
		"address",
		"latitude",
		"longitude",
		"created_at",
		"updated_at").
		From(TableStop).
		ToSql()
	if err != nil {
		s.logger.Error("failed to build query from database", zap.Error(err))
		return nil, err
	}
	err = s.qb.DB.SelectContext(ctx, &stops, query, args...)
	if err != nil {
		s.logger.Error("failed to execute query from database", zap.Error(err))
		return nil, err
	}
	return stops, nil
}

func (s *stop) ByID(ctx context.Context, id int) (*model.Stop, error) {
	var result model.Stop
	query, args, err := s.qb.Sq.Select(
		"name",
		"address",
		"latitude",
		"longitude",
		"created_at",
		"updated_at").
		From(TableStop).
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		s.logger.Error("failed to build query from database", zap.Error(err))
		return nil, err
	}
	err = s.qb.DB.GetContext(ctx, &result, query, args...)
	if err != nil {
		s.logger.Error("failed to execute query from database", zap.Error(err))
		return nil, err
	}
	return &result, nil
}

func (s *stop) Create(ctx context.Context, stop *model.Stop) error {
	query, args, err := s.qb.Sq.Insert(TableStop).Columns(
		"name",
		"address",
		"latitude",
		"longitude").
		Values(
			stop.Name,
			stop.Address,
			stop.Latitude,
			stop.Longitude).
		ToSql()
	if err != nil {
		s.logger.Error("failed to build query from database", zap.Error(err))
		return err
	}
	_, err = s.qb.DB.ExecContext(ctx, query, args...)
	if err != nil {
		s.logger.Error("failed to execute query from database", zap.Error(err))
		return err
	}
	return nil
}

func (s *stop) Update(ctx context.Context, stop *model.StopUpdate) error {
	query, args, err := s.qb.Sq.Update(TableStop).
		SetMap(stop.ToMap()).
		Where(sq.Eq{"id": stop.ID}).
		ToSql()
	if err != nil {
		s.logger.Error("failed to build query from database", zap.Error(err))
		return err
	}
	_, err = s.qb.DB.ExecContext(ctx, query, args...)
	if err != nil {
		s.logger.Error("failed to execute query from database", zap.Error(err))
		return err
	}
	return nil
}

func (s *stop) Delete(ctx context.Context, id int) error {
	query, args, err := s.qb.Sq.Delete(TableStop).Where(sq.Eq{"id": id}).ToSql()
	if err != nil {
		s.logger.Error("failed to build query from database", zap.Error(err))
		return err
	}
	_, err = s.qb.DB.ExecContext(ctx, query, args...)
	if err != nil {
		s.logger.Error("failed to execute query from database", zap.Error(err))
		return err
	}
	return nil
}
