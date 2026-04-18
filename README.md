# Elm

Zero-dependency Go ORM wrapper around database/sql with struct mapping, query builder, and PostgreSQL/SQLite support.

## Installation

```bash
go get github.com/draftloop/elm
```

## Usage

### Open a connection

```go
db, err := elm.Open("sqlite", "./app.db")
if err != nil {
    log.Fatal(err)
}
defer db.Close()
```

PostgreSQL is supported — the driver is detected from the driver name (`"postgres"`, `"pgx"`, …) and `?` placeholders are automatically rewritten to `$1`, `$2`, …

### Logger

An optional logger can be provided via `Config`. It receives the query, arguments, execution duration, and any error.

```go
db, err := elm.Open("sqlite", "./app.db", elm.Config{
    Logger: func(query string, args []any, duration time.Duration, err error) {
        if err != nil {
            log.Printf("ERR %s %v (%s): %v", query, args, duration, err)
        } else {
            log.Printf("%s %v (%s)", query, args, duration)
        }
    },
})
```

The logger can also be set after opening the connection with `db.SetLogger(...)`.

### Define a model

Fields are mapped to snake_case columns automatically. A pointer field is treated as nullable. A nested struct field is treated as a relation.

```go
type Post struct {
    ID      int64
    Message string
    UserID  *int64

    User *User
}
```

| Go field       | SQL column       |
|----------------|------------------|
| `ID`           | `id`             |
| `Username`     | `username`       |
| `ForeignKeyID` | `foreign_key_id` |

Table names are derived from the struct name in snake_case and pluralized: `User` → `users`, `Status` → `statuses`. Struct names should be singular.

### Save

`Save` inserts or updates based on whether `ID` is zero.

```go
user := &User{Username: "alice"}
err = db.Save(user) // INSERT — sets user.ID
// user.ID == 1

user.Username = "bob"
err = db.Save(user) // UPDATE WHERE id = 1
```

`Save` requires a pointer.

### Delete

`Delete` deletes the row by ID and zeroes out the struct.

```go
err = db.Delete(&user)
// user == nil
```

### Query builder

```go
// Fetch one
var user User
err = db.Model(User{}).Where(elm.Eq("id", int64(1))).Scan(&user)

// Fetch a slice
var users []User
err = db.Model(User{}).UnsafeOrderBy("id asc").Limit(10).Offset(20).Scan(&users)

// Fetch a scalar
var username string
err = db.Model(User{}).UnsafeSelect("username").Where(elm.Eq("id", int64(1))).Scan(&username)

// Fetch a nullable pointer — nil if no row found
var user *User
err = db.Model(User{}).Where(elm.Eq("id", int64(99))).Scan(&user)
// user == nil
```

### Where conditions

```go
elm.Eq("id", 1)
elm.NotEq("status", "banned")
elm.Gt("age", 18)
elm.GtEq("age", 18)
elm.Lt("score", 100)
elm.LtEq("score", 100)
elm.Like("username", "ali%")
elm.In("id", []any{1, 2, 3})
elm.NotIn("id", []any{1, 2, 3})
elm.IsNull("deleted_at")
elm.IsNotNull("deleted_at")

elm.Not(elm.Eq("status", "banned"))

elm.And(elm.Eq("active", true), elm.Gt("age", 18))
elm.Or(elm.Eq("role", "admin"), elm.Eq("role", "mod"))

elm.UnsafeWhere("created_at > ?", someTime)
```

### Joins

Relations are declared as `*T` struct fields. The join condition is derived automatically from the foreign key convention (`<RelationName>ID`).

```go
// LEFT JOIN — User field is nil if no matching row
var posts []Post
err = db.Model(Post{}).LeftRelation(User{}).Scan(&posts)

// INNER JOIN
err = db.Model(Post{}).InnerRelation(User{}).Scan(&posts)
```

### Free joins

For joins not tied to declared relations, use `UnsafeJoin`. The SQL alias defaults to the model name. All fields of each joined model are automatically added to the `SELECT` — use `SelectFrom` or `SelectAllFrom` to restrict which fields are selected.

```go
var results []UserWorkPerExcavator
err = db.Model(WorkHour{}).
    UnsafeJoin("INNER", User{}, "User.id = WorkHour.user_id").
    UnsafeJoin("INNER", Excavator{}, "Excavator.id = WorkHour.excavator_id").
    Scan(&results)
```

Use `UnsafeJoinAs` to set a custom SQL alias — required when the same model is joined more than once, or when the scan destination field name differs from the model name.

### Scan destination struct

When scanning into a struct, fields are classified as follows:

- A scalar field (`int64`, `string`, `time.Time`, …) maps to the column matching its snake_case name.
- A nested struct field (`T` or `*T`) is treated as a relation and maps to columns prefixed with `<alias>__`, where alias is the model name by default or the custom alias set via `UnsafeJoinAs` / `SelectAllFromAs` / `SelectFromAs` (e.g. `MostUsedExcavator__id`).
- A `*T` relation field is set to `nil` after the scan if the joined row has a zero ID (i.e. no match).
- A `T` relation field is never nil — use it only when the join is guaranteed to return a row.

The scan destination does not need to mirror the model exactly — it can be a flat or partial struct combining fields from multiple models:

```go
type Row struct {
    Excavator  Excavator // mapped via Excavator__*
    Hours      int64     // mapped via "hours"
    LastUserID int64     // mapped via "last_user_id"
}
```

### SelectFrom

`SelectFrom` generates column aliases for the specified fields of a model. `SelectAllFrom` does the same for all fields.

Only the listed fields will be present in the query — any struct field in the scan destination that has no matching column will keep its zero value.

```go
// Only ID and Username are selected — other User fields will be zero
err = db.Model(WorkHour{}).
    SelectFrom(User{}, "ID", "Username").
    SelectFrom(Excavator{}, "ID", "RegistrationNumber").
    UnsafeJoin("INNER", User{}, "User.id = WorkHour.user_id").
    UnsafeJoin("INNER", Excavator{}, "Excavator.id = WorkHour.excavator_id").
    Scan(&results)

// All Excavator fields are selected
err = db.Model(WorkHour{}).
    SelectAllFrom(Excavator{}).
    UnsafeJoin("INNER", Excavator{}, "Excavator.id = WorkHour.excavator_id").
    Scan(&results)
```

Use `SelectFromAs` / `SelectAllFromAs` when a custom SQL alias is needed (e.g. with `UnsafeJoinAs` or `LeftRelation` where the field name differs from the model name).

### Update / Delete via builder

```go
// Update specific columns with a WHERE condition
err = db.Model(User{}).Set("active", false).Where(elm.Eq("role", "banned")).Update()

// Delete with a WHERE condition
err = db.Model(User{}).Where(elm.Lt("created_at", cutoff)).Delete()
```

Both `Update` and `Delete` require at least one `Where()` condition. Use `UnsafeWhere("1=1")` to explicitly target all rows. Joins are not supported on `Update` and `Delete`.

### Group by

```go
err = db.Model(WorkHour{}).
    SelectFrom(Excavator{}, "ID", "RegistrationNumber").
    SelectFrom(User{}, "ID", "Username").
    UnsafeSelect("SUM(WorkHour.hours) AS hours").
    UnsafeJoin("INNER", User{}, "User.id = WorkHour.user_id").
    UnsafeJoin("INNER", Excavator{}, "Excavator.id = WorkHour.excavator_id").
    UnsafeGroupBy("Excavator.id", "User.id").
    UnsafeOrderBy("Excavator.id asc", "User.id asc").
    Scan(&results)
```

### Manual queries

```go
result, err := db.Exec("UPDATE users SET active = ? WHERE id = ?", true, 1)
rows, err := db.Query("SELECT * FROM users WHERE active = ?", true)
row := db.QueryRow("SELECT COUNT(*) FROM users")
```

## Limitations

- `UnsafeSelect`, `UnsafeOrderBy`, `UnsafeGroupBy`, `UnsafeJoin`, and `UnsafeJoinAs` accept raw strings — do not use with user input.
- `ID` field must be a non-pointer integer type (`int64`, `int`, …).
- `[]*T` slice destinations are not supported — use `[]T` instead.
- Joins are not supported on `Update` and `Delete`.
- Transactions are not yet supported.