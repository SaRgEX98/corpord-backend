package pg

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// Коды ошибок PostgreSQL
const (
	// Class 23 - Integrity Constraint Violation
	ErrorCodeIntegrityConstraintViolation = "23"
	ErrorCodeForeignKeyViolation          = "23503"
	ErrorCodeUniqueViolation              = "23505"
	// Class 42 - Syntax Error or Access Rule Violation
	ErrorCodeUndefinedTable = "42P01"
)

var (
	// ErrNotFound возвращается, когда запрашиваемый ресурс не найден.
	ErrNotFound = errors.New("resource not found")

	// ErrAlreadyExists возвращается при попытке создать сущность с уже существующими данными.
	ErrAlreadyExists = errors.New("already exists")

	// ErrForeignKeyViolation возвращается при нарушении ограничения внешнего ключа.
	ErrForeignKeyViolation = errors.New("foreign key violation")

	// ErrUniqueViolation возвращается при нарушении ограничения уникальности.
	ErrUniqueViolation = errors.New("unique constraint violation")

	// ErrNoRowsAffected возвращается, когда операция не затронула ни одной строки.
	ErrNoRowsAffected = errors.New("no rows affected")
)

// IsPgError проверяет, является ли ошибка ошибкой PostgreSQL с указанным кодом.
// Коды ошибок PostgreSQL: https://www.postgresql.org/docs/current/errcodes-appendix.html
// IsPgError проверяет, является ли ошибка ошибкой PostgreSQL с указанным кодом.
// Коды ошибок PostgreSQL: https://www.postgresql.org/docs/current/errcodes-appendix.html
func IsPgError(err error, code string) bool {
	if err == nil {
		return false
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}

	return pgErr.Code == code
}
