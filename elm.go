package elm

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"time"
)

type Logger func(query string, args []any, duration time.Duration, err error)

type Config struct {
	Logger Logger
}

type Elm struct {
	sql        *sql.DB
	isPostgres bool
	logger     Logger
}

func Open(driverName string, dataSourceName string, cfg ...Config) (*Elm, error) {
	sqlDb, err := sql.Open(driverName, dataSourceName)
	if err != nil {
		return nil, err
	}
	e := &Elm{sql: sqlDb, isPostgres: isPostgresDriver(driverName)}
	if len(cfg) > 0 {
		e.logger = cfg[0].Logger
	}
	return e, nil
}

func (o *Elm) SetLogger(logger Logger) {
	o.logger = logger
}

func (o *Elm) log(query string, args []any, duration time.Duration, err error) {
	if o.logger != nil {
		o.logger(query, args, duration, err)
	}
}

func isPostgresDriver(driver string) bool {
	return driver == "postgres" || driver == "pgx" || strings.HasPrefix(driver, "pgx/")
}

func (o *Elm) prepareQuery(query string) string {
	if o.isPostgres {
		n := 0
		var sb strings.Builder
		for _, c := range query {
			if c == '?' {
				n++
				fmt.Fprintf(&sb, "$%d", n)
			} else {
				sb.WriteRune(c)
			}
		}
		query = sb.String()
	}

	return query
}

// func (o *Elm) Begin() (*Tx, error)

// func (o *Elm) BeginTx(ctx context.Context, opts *TxOptions) (*Tx, error)

func (o *Elm) Close() error {
	return o.sql.Close()
}

// func (o *Elm) Conn(ctx context.Context) (*Conn, error)

// func (o *Elm) Driver() driver.Driver

func (o *Elm) Exec(query string, args ...any) (sql.Result, error) {
	return o.ExecContext(context.Background(), query, args...)
}

func (o *Elm) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	query = o.prepareQuery(query)
	start := time.Now()
	result, err := o.sql.ExecContext(ctx, query, args...)
	o.log(query, args, time.Since(start), err)
	return result, err
}

func (o *Elm) Ping() error {
	return o.PingContext(context.Background())
}

func (o *Elm) PingContext(ctx context.Context) error {
	return o.sql.PingContext(ctx)
}

func (o *Elm) Prepare(query string) (*sql.Stmt, error) {
	return o.PrepareContext(context.Background(), query)
}

func (o *Elm) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	query = o.prepareQuery(query)
	start := time.Now()
	stmt, err := o.sql.PrepareContext(ctx, query)
	o.log(query, nil, time.Since(start), err)
	return stmt, err
}

func (o *Elm) Query(query string, args ...any) (*sql.Rows, error) {
	return o.QueryContext(context.Background(), query, args...)
}

func (o *Elm) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	query = o.prepareQuery(query)
	start := time.Now()
	rows, err := o.sql.QueryContext(ctx, query, args...)
	o.log(query, args, time.Since(start), err)
	return rows, err
}

func (o *Elm) QueryRow(query string, args ...any) *sql.Row {
	return o.QueryRowContext(context.Background(), query, args...)
}

func (o *Elm) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	query = o.prepareQuery(query)
	start := time.Now()
	row := o.sql.QueryRowContext(ctx, query, args...)
	o.log(query, args, time.Since(start), row.Err())
	return row
}

func (o *Elm) SetConnMaxIdleTime(d time.Duration) {
	o.sql.SetConnMaxIdleTime(d)
}

func (o *Elm) SetConnMaxLifetime(d time.Duration) {
	o.sql.SetConnMaxLifetime(d)
}

func (o *Elm) SetMaxIdleConns(n int) {
	o.sql.SetMaxIdleConns(n)
}

func (o *Elm) SetMaxOpenConns(n int) {
	o.sql.SetMaxOpenConns(n)
}

func (o *Elm) Stats() sql.DBStats {
	return o.sql.Stats()
}

/* type ColumnType
func (ci *ColumnType) DatabaseTypeName() string
func (ci *ColumnType) DecimalSize() (precision, scale int64, ok bool)
func (ci *ColumnType) Length() (length int64, ok bool)
func (ci *ColumnType) Name() string
func (ci *ColumnType) Nullable() (nullable, ok bool)
func (ci *ColumnType) ScanType() reflect.Type */
/* type Conn
func (c *Conn) BeginTx(ctx context.Context, opts *TxOptions) (*Tx, error)
func (c *Conn) Close() error
func (c *Conn) ExecContext(ctx context.Context, query string, args ...any) (Result, error)
func (c *Conn) PingContext(ctx context.Context) error
func (c *Conn) PrepareContext(ctx context.Context, query string) (*Stmt, error)
func (c *Conn) QueryContext(ctx context.Context, query string, args ...any) (*Rows, error)
func (c *Conn) QueryRowContext(ctx context.Context, query string, args ...any) *Row
func (c *Conn) Raw(f func(driverConn any) error) (err error) */
/* type IsolationLevel
func (i IsolationLevel) String() string */
/* type NamedArg
func Named(name string, value any) NamedArg */
/* type Null
func (n *Null[T]) Scan(value any) error
func (n Null[T]) Value() (driver.Value, error) */
/* type NullBool
func (n *NullBool) Scan(value any) error
func (n NullBool) Value() (driver.Value, error) */
/* type NullByte
func (n *NullByte) Scan(value any) error
func (n NullByte) Value() (driver.Value, error) */
/* type NullFloat64
func (n *NullFloat64) Scan(value any) error
func (n NullFloat64) Value() (driver.Value, error) */
/* type NullInt16
func (n *NullInt16) Scan(value any) error
func (n NullInt16) Value() (driver.Value, error) */
/* type NullInt32
func (n *NullInt32) Scan(value any) error
func (n NullInt32) Value() (driver.Value, error) */
/* type NullInt64
func (n *NullInt64) Scan(value any) error
func (n NullInt64) Value() (driver.Value, error) */
/* type NullString
func (ns *NullString) Scan(value any) error
func (ns NullString) Value() (driver.Value, error) */
/* type NullTime
func (n *NullTime) Scan(value any) error
func (n NullTime) Value() (driver.Value, error) */
/* type Out */
/* type RawBytes */
/* type Result */
/* type Row
func (r *Row) Err() error
func (r *Row) Scan(dest ...any) error */
/* type Rows
func (rs *Rows) Close() error
func (rs *Rows) ColumnTypes() ([]*ColumnType, error)
func (rs *Rows) Columns() ([]string, error)
func (rs *Rows) Err() error
func (rs *Rows) Next() bool
func (rs *Rows) NextResultSet() bool
func (rs *Rows) Scan(dest ...any) error */
/* type Scanner */
/* type Stmt
func (s *Stmt) Close() error
func (s *Stmt) Exec(args ...any) (Result, error)
func (s *Stmt) ExecContext(ctx context.Context, args ...any) (Result, error)
func (s *Stmt) Query(args ...any) (*Rows, error)
func (s *Stmt) QueryContext(ctx context.Context, args ...any) (*Rows, error)
func (s *Stmt) QueryRow(args ...any) *Row
func (s *Stmt) QueryRowContext(ctx context.Context, args ...any) *Row */
/* type Tx
func (tx *Tx) Commit() error
func (tx *Tx) Exec(query string, args ...any) (Result, error)
func (tx *Tx) ExecContext(ctx context.Context, query string, args ...any) (Result, error)
func (tx *Tx) Prepare(query string) (*Stmt, error)
func (tx *Tx) PrepareContext(ctx context.Context, query string) (*Stmt, error)
func (tx *Tx) Query(query string, args ...any) (*Rows, error)
func (tx *Tx) QueryContext(ctx context.Context, query string, args ...any) (*Rows, error)
func (tx *Tx) QueryRow(query string, args ...any) *Row
func (tx *Tx) QueryRowContext(ctx context.Context, query string, args ...any) *Row
func (tx *Tx) Rollback() error
func (tx *Tx) Stmt(stmt *Stmt) *Stmt
func (tx *Tx) StmtContext(ctx context.Context, stmt *Stmt) *Stmt */

func (o *Elm) Save(entity any) error {
	v := reflect.ValueOf(entity)
	if v.Kind() != reflect.Pointer {
		return fmt.Errorf("elm: Save requires a pointer, got %T", entity)
	}

	model := scanAndCacheStruct(entity)

	val := reflectRealValueOf(entity)

	idField, ok := model.FieldsByName["ID"]
	if !ok {
		return fmt.Errorf("elm: Save requires a struct with an ID field")
	}

	id := val.FieldByName(idField.Name)
	if id.IsZero() {
		var insertedID int64
		if err := o.Model(entity).SetFromStruct(entity, model).Insert(&insertedID); err != nil {
			return err
		}
		if !id.CanInt() {
			return fmt.Errorf("elm: Save: ID field must be an integer type, got %s", id.Type())
		}
		id.SetInt(insertedID)
		return nil
	}

	return o.Model(entity).SetFromStruct(entity, model).
		Where(Eq("id", id.Interface())).
		Update()
}

func (o *Elm) Model(model any) *Builder {
	return &Builder{
		elm:   o,
		table: scanAndCacheStruct(model),
	}
}

func (o *Elm) Delete(entity any) error {
	v := reflect.ValueOf(entity)
	if v.Kind() != reflect.Pointer {
		return fmt.Errorf("elm: Delete requires a pointer, got %T", entity)
	}

	model := scanAndCacheStruct(entity)

	val := reflectRealValueOf(entity)

	idField, ok := model.FieldsByName["ID"]
	if !ok {
		return fmt.Errorf("elm: Delete requires a struct with an ID field")
	}

	id := val.FieldByName(idField.Name)
	if id.IsZero() {
		return fmt.Errorf("elm: Delete requires a non-zero ID")
	}

	err := o.Model(entity).
		Where(Eq("id", id.Interface())).
		Delete()
	if err != nil {
		return err
	}

	v.Elem().Set(reflect.Zero(v.Elem().Type()))

	return nil
}
