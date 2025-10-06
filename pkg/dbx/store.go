package dbx

import (
	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
)

type QueryBuilder struct {
	DB *sqlx.DB
	Sq sq.StatementBuilderType
}

func NewQueryBuilder(db *sqlx.DB) *QueryBuilder {
	return &QueryBuilder{
		DB: db,
		Sq: sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
	}
}
