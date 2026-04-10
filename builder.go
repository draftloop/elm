package elm

import (
	"fmt"
	"reflect"
	"strings"
)

type Builder struct {
	elm   *Elm
	table *tableInfo

	set []builderSet

	selects   []string
	where     []BuilderWhere
	relations []builderJoin
	orderBy   []string
	groupBy   []string
	limit     *int
	offset    *int
	err       error
}

type builderSet struct {
	column string
	value  any
}

type BuilderWhere struct {
	kind string

	field    string
	operator string
	value    any

	where []BuilderWhere
}

type builderJoin struct {
	kind  string
	table *tableInfo
	alias string
	on    string
}

func (w *BuilderWhere) Build() (string, []any) {
	if w.kind == "COND" {
		if w.operator == "IN" {
			values, _ := w.value.([]any)
			placeholders := make([]string, len(values))
			for i := range values {
				placeholders[i] = "?"
			}
			return fmt.Sprintf("%s IN (%s)", w.field, strings.Join(placeholders, ", ")), values
		}
		if w.operator == "IS NULL" || w.operator == "IS NOT NULL" {
			return fmt.Sprintf("%s %s", w.field, w.operator), nil
		}
		return fmt.Sprintf("%s %s ?", w.field, w.operator), []any{w.value}
	}
	var clauses []string
	var args []any
	for _, c := range w.where {
		clause, args2 := c.Build()
		args = append(args, args2...)
		clauses = append(clauses, clause)
	}
	return "(" + strings.Join(clauses, " "+w.kind+" ") + ")", args
}

func (b *Builder) Set(column string, value any) *Builder {
	b.set = append(b.set, builderSet{column, value})
	return b
}

func (b *Builder) SetFromStruct(value any, table *tableInfo) *Builder {
	v := reflectRealValueOf(value)

	for _, f := range table.Fields {
		if f.IsPrimary {
			continue
		}
		b.set = append(b.set, struct {
			column string
			value  any
		}{f.Column, v.FieldByName(f.Name).Interface()})
	}
	return b
}

func (b *Builder) UnsafeSelect(args ...string) *Builder {
	b.selects = append(b.selects, args...)
	return b
}

func (b *Builder) Where(args ...BuilderWhere) *Builder {
	b.where = append(b.where, args...)
	return b
}

func Eq(field string, value any) BuilderWhere {
	return BuilderWhere{kind: "COND", field: field, operator: "=", value: value}
}

func NotEq(field string, value any) BuilderWhere {
	return BuilderWhere{kind: "COND", field: field, operator: "!=", value: value}
}

func Gt(field string, value any) BuilderWhere {
	return BuilderWhere{kind: "COND", field: field, operator: ">", value: value}
}

func GtEq(field string, value any) BuilderWhere {
	return BuilderWhere{kind: "COND", field: field, operator: ">=", value: value}
}

func Lt(field string, value any) BuilderWhere {
	return BuilderWhere{kind: "COND", field: field, operator: "<", value: value}
}
func LtEq(field string, value any) BuilderWhere {
	return BuilderWhere{kind: "COND", field: field, operator: "<=", value: value}
}

func Like(field string, value any) BuilderWhere {
	return BuilderWhere{kind: "COND", field: field, operator: "LIKE", value: value}
}

func In(field string, values []any) BuilderWhere {
	return BuilderWhere{kind: "COND", field: field, operator: "IN", value: values}
}

func IsNull(field string) BuilderWhere {
	return BuilderWhere{kind: "COND", field: field, operator: "IS NULL"}
}

func IsNotNull(field string) BuilderWhere {
	return BuilderWhere{kind: "COND", field: field, operator: "IS NOT NULL"}
}

func And(and ...BuilderWhere) BuilderWhere {
	return BuilderWhere{kind: "AND", where: and}
}

func Or(or ...BuilderWhere) BuilderWhere {
	return BuilderWhere{kind: "OR", where: or}
}

func (b *Builder) InnerRelation(model any) *Builder {
	if b.err != nil {
		return b
	}
	relationTable := getStructOrNil(model)
	if relationTable == nil {
		b.err = fmt.Errorf("elm: InnerRelation: %T is not a registered struct", model)
		return b
	}
	for _, rel := range b.table.Relations {
		if rel.TableType == relationTable.Type {
			b.relations = append(b.relations, builderJoin{kind: "INNER", table: relationTable, alias: rel.ModelRelationName})
			return b
		}
	}
	b.err = fmt.Errorf("elm: InnerRelation: %T is not a declared relation on %s", model, b.table.ModelName)
	return b
}

func (b *Builder) LeftRelation(model any) *Builder {
	if b.err != nil {
		return b
	}
	relationTable := getStructOrNil(model)
	if relationTable == nil {
		b.err = fmt.Errorf("elm: LeftRelation: %T is not a registered struct", model)
		return b
	}
	for _, rel := range b.table.Relations {
		if rel.TableType == relationTable.Type {
			b.relations = append(b.relations, builderJoin{kind: "LEFT", table: relationTable, alias: rel.ModelRelationName})
			return b
		}
	}
	b.err = fmt.Errorf("elm: LeftRelation: %T is not a declared relation on %s", model, b.table.ModelName)
	return b
}

func (b *Builder) UnsafeJoin(kind string, model any, on string) *Builder {
	table := scanAndCacheStruct(model)
	b.relations = append(b.relations, builderJoin{kind: kind, table: table, alias: table.ModelName, on: on})
	return b
}

func (b *Builder) UnsafeJoinAs(kind string, model any, alias string, on string) *Builder {
	table := scanAndCacheStruct(model)
	b.relations = append(b.relations, builderJoin{kind: kind, table: table, alias: alias, on: on})
	return b
}

func (b *Builder) SelectFrom(model any, fields ...string) *Builder {
	if b.err != nil {
		return b
	}
	table := scanAndCacheStruct(model)
	for _, field := range fields {
		fi, ok := table.FieldsByName[field]
		if !ok {
			b.err = fmt.Errorf("elm: SelectFrom: field %q not found on %s", field, table.ModelName)
			return b
		}
		b.selects = append(b.selects, fmt.Sprintf("%s.%s AS %s", table.ModelName, fi.Column, table.ModelName+"__"+fi.Column))
	}
	return b
}

func (b *Builder) SelectFromAs(model any, alias string, fields ...string) *Builder {
	if b.err != nil {
		return b
	}
	table := scanAndCacheStruct(model)
	for _, field := range fields {
		fi, ok := table.FieldsByName[field]
		if !ok {
			b.err = fmt.Errorf("elm: SelectFrom: field %q not found on %s", field, alias)
			return b
		}
		b.selects = append(b.selects, fmt.Sprintf("%s.%s AS %s", alias, fi.Column, alias+"__"+fi.Column))
	}
	return b
}

func (b *Builder) SelectAllFrom(model any) *Builder {
	table := scanAndCacheStruct(model)
	for _, fi := range table.Fields {
		b.selects = append(b.selects, fmt.Sprintf("%s.%s AS %s", table.ModelName, fi.Column, table.ModelName+"__"+fi.Column))
	}
	return b
}

func (b *Builder) SelectAllFromAs(model any, alias string) *Builder {
	table := scanAndCacheStruct(model)
	for _, fi := range table.Fields {
		b.selects = append(b.selects, fmt.Sprintf("%s.%s AS %s", alias, fi.Column, alias+"__"+fi.Column))
	}
	return b
}

func (b *Builder) UnsafeOrderBy(args ...string) *Builder {
	b.orderBy = append(b.orderBy, args...)
	return b
}

func (b *Builder) UnsafeGroupBy(args ...string) *Builder {
	b.groupBy = append(b.groupBy, args...)
	return b
}

func (b *Builder) Limit(limit int) *Builder {
	b.limit = &limit
	return b
}

func (b *Builder) Offset(offset int) *Builder {
	b.offset = &offset
	return b
}

func (b *Builder) Insert(insertedID ...*int64) error {
	if b.err != nil {
		return b.err
	}
	if len(b.set) == 0 {
		return fmt.Errorf("elm: Insert requires at least one Set()")
	}

	columns := make([]string, len(b.set))
	placeholders := make([]string, len(b.set))
	args := make([]any, len(b.set))

	for i, s := range b.set {
		columns[i] = s.column
		placeholders[i] = "?"
		args[i] = s.value
	}

	query := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		b.table.TableName,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
	)

	result, err := b.elm.Exec(query, args...)
	if err != nil {
		return err
	}

	if len(insertedID) > 0 && insertedID[0] != nil {
		*insertedID[0], err = result.LastInsertId()
	}
	return err
}

func (b *Builder) Scan(dests ...any) error {
	if b.err != nil {
		return b.err
	}
	isSlice := false

	switch len(dests) {
	case 0:
		return fmt.Errorf("elm: Scan requires at least one destination")
	case 1:
		val := reflect.ValueOf(dests[0])
		if val.Kind() != reflect.Pointer {
			return fmt.Errorf("elm: Scan requires a pointer")
		}
		isSlice = val.Elem().Kind() == reflect.Slice
		if isSlice {
			elemType := val.Elem().Type().Elem()
			if elemType.Kind() == reflect.Pointer {
				return fmt.Errorf("elm: Scan does not support slice of pointers ([]*T), use []T instead")
			}
		}
	default:
		for i, dest := range dests {
			val := reflect.ValueOf(dest)
			if val.Kind() != reflect.Pointer {
				return fmt.Errorf("elm: Scan dest[%d] must be a pointer", i)
			}
			switch val.Elem().Kind() {
			case reflect.Struct:
				return fmt.Errorf("elm: dest[%d] is a struct — a struct destination must be the only destination", i)
			case reflect.Slice:
				return fmt.Errorf("elm: dest[%d] is a slice — a slice destination must be the only destination", i)
			case reflect.Pointer:
				return fmt.Errorf("elm: dest[%d] is a pointer — pointer inception are prohibited", i)
			}
		}
	}

	limitClause := ""
	if !isSlice {
		limitClause = " LIMIT 1"
	} else if b.limit != nil {
		limitClause = fmt.Sprintf(" LIMIT %d", *b.limit)
	}

	offsetClause := ""
	if b.offset != nil {
		offsetClause = fmt.Sprintf(" OFFSET %d", *b.offset)
	}

	whereClause, whereArgs := b.buildWhere()
	query := fmt.Sprintf("SELECT %s FROM %s %s%s%s%s%s%s%s",
		b.buildSelect(),
		b.table.TableName,
		b.table.ModelName,
		b.buildJoins(),
		whereClause,
		b.buildGroupBy(),
		b.buildOrderBy(),
		limitClause,
		offsetClause,
	)

	rows, err := b.elm.Query(query, whereArgs...)
	if err != nil {
		return err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return err
	}

	postScanTargets := func(dest reflect.Value, destTable *tableInfo) {
		for _, rel := range destTable.Relations {
			if rel.Nullable {
				fieldVal := dest.FieldByName(rel.ModelRelationName)
				if !fieldVal.IsNil() {
					idField := fieldVal.Elem().FieldByName("ID")
					if !idField.IsValid() || idField.IsZero() {
						fieldVal.Set(reflect.Zero(reflect.PointerTo(rel.TableType)))
					}
				}
			}
		}
	}

	if isSlice {
		slice := reflect.ValueOf(dests[0]).Elem()
		elemType := slice.Type().Elem()

		if elemType.Kind() == reflect.Struct && !isScalarStruct(elemType) {
			for rows.Next() {
				elem := reflect.New(elemType).Elem()
				tmp := scanTemporaryStruct(reflect.New(elemType).Elem().Interface())
				targets, err := b.buildScanTargets(elem, &tmp, cols)
				if err != nil {
					return err
				}
				if err := rows.Scan(targets...); err != nil {
					return err
				}
				postScanTargets(elem, &tmp)
				slice.Set(reflect.Append(slice, elem))
			}
		} else {
			for rows.Next() {
				elemPtr := reflect.New(elemType)
				if err := rows.Scan(elemPtr.Interface()); err != nil {
					return err
				}
				slice.Set(reflect.Append(slice, elemPtr.Elem()))
			}
		}
	} else {
		if !rows.Next() {
			if err := rows.Err(); err != nil {
				return err
			}
			return nil
		}

		outer := reflect.ValueOf(dests[0]) // **test.User (ou *int64, etc.)
		dest := outer.Elem()               // *test.User (ce que pointe &user3)

		for dest.Kind() == reflect.Pointer {
			if dest.IsNil() {
				newVal := reflect.New(dest.Type().Elem())
				outer.Elem().Set(newVal) // écrit dans *(&user3) = user3 ✅
				dest = newVal
			}
			dest = dest.Elem()
		}

		if dest.Kind() == reflect.Struct && !isScalarStruct(dest.Type()) {
			tmp := scanTemporaryStruct(dest.Interface())
			targets, err := b.buildScanTargets(dest, &tmp, cols)
			if err != nil {
				return err
			}
			if err := rows.Scan(targets...); err != nil {
				return err
			}
			postScanTargets(dest, &tmp)
		} else {
			targets := make([]any, len(dests))
			for i, d := range dests {
				dest := reflect.ValueOf(d).Elem()
				targets[i] = &nullProxy{field: dest}
			}
			if err := rows.Scan(targets...); err != nil {
				return err
			}
		}
	}

	return rows.Err()
}

func (b *Builder) buildScanTargets(dest reflect.Value, destTable *tableInfo, cols []string) ([]any, error) {
	colToPtr := make(map[string]any)

	for _, fi := range destTable.Fields {
		colToPtr[fi.Column] = &nullProxy{dest.FieldByName(fi.Name)}
		colToPtr[destTable.ModelName+"__"+fi.Column] = &nullProxy{dest.FieldByName(fi.Name)}
	}

	for _, rel := range destTable.Relations {
		relationField := dest.FieldByName(rel.ModelRelationName)

		var relationFieldValue reflect.Value
		if rel.Nullable {
			tmp := reflect.New(rel.TableType)
			relationField.Set(tmp)
			relationFieldValue = tmp.Elem()
		} else {
			relationFieldValue = relationField
		}

		relTable := getStructOrNil(reflect.New(rel.TableType).Elem().Interface())
		if relTable == nil {
			panic(fmt.Sprintf("elm: relation type %s is not a registered struct", rel.TableType))
		}
		for _, fi := range relTable.Fields {
			colToPtr[rel.ModelRelationName+"__"+fi.Column] = &nullProxy{relationFieldValue.FieldByName(fi.Name)}
		}
	}

	targets := make([]any, len(cols))
	for i, col := range cols {
		if ptr, ok := colToPtr[col]; ok {
			targets[i] = ptr
		} else {
			var discard any
			targets[i] = &discard
		}
	}
	return targets, nil
}

func (b *Builder) buildSelect() string {
	if len(b.selects) > 0 {
		return strings.Join(b.selects, ", ")
	}

	var parts []string
	for _, f := range b.table.Fields {
		parts = append(parts, fmt.Sprintf("%s.%s AS %s", b.table.ModelName, f.Column, b.table.ModelName+"__"+f.Column))
	}
	if len(b.relations) != 0 {
		for _, rel := range b.relations {
			for _, f := range rel.table.Fields {
				parts = append(parts, fmt.Sprintf("%s.%s AS %s", rel.alias, f.Column, rel.alias+"__"+f.Column))
			}
		}
	}

	return strings.Join(parts, ", ")
}

func (b *Builder) buildJoins() string {
	if len(b.relations) == 0 {
		return ""
	}
	var parts []string
	for _, rel := range b.relations {
		onClause := rel.on
		if onClause == "" {
			fk, ok := b.table.FieldsByName[rel.alias+"ID"]
			if !ok {
				panic(fmt.Sprintf("elm: %s has no field %sID required for join on %s", b.table.ModelName, rel.alias, rel.table.TableName))
			}
			onClause = fmt.Sprintf("%s = %s", rel.alias+".ID", b.table.ModelName+"."+fk.Column)
		}
		parts = append(parts, fmt.Sprintf(" %s JOIN %s %s ON %s", rel.kind, rel.table.TableName, rel.alias, onClause))
	}
	return strings.Join(parts, "")
}

func (b *Builder) buildWhere() (string, []any) {
	if len(b.where) == 0 {
		return "", nil
	}

	var clauses []string
	var args []any
	for _, w := range b.where {
		clause, args2 := w.Build()
		args = append(args, args2...)
		clauses = append(clauses, clause)
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func (b *Builder) buildGroupBy() string {
	if len(b.groupBy) == 0 {
		return ""
	}
	return " GROUP BY " + strings.Join(b.groupBy, ", ")
}

func (b *Builder) buildOrderBy() string {
	if len(b.orderBy) == 0 {
		return ""
	}
	return " ORDER BY " + strings.Join(b.orderBy, ", ")
}

func (b *Builder) Update(affected ...*int64) error {
	if b.err != nil {
		return b.err
	}
	if len(b.set) == 0 {
		return fmt.Errorf("elm: Update requires at least one Set()")
	}

	setClauses := make([]string, len(b.set))
	args := make([]any, len(b.set))

	for i, s := range b.set {
		setClauses[i] = fmt.Sprintf("%s = ?", s.column)
		args[i] = s.value
	}

	whereClause, whereArgs := b.buildWhere()
	args = append(args, whereArgs...)

	query := fmt.Sprintf(
		"UPDATE %s SET %s%s",
		b.table.TableName,
		strings.Join(setClauses, ", "),
		whereClause,
	)

	result, err := b.elm.Exec(query, args...)
	if err != nil {
		return err
	}

	if len(affected) > 0 && affected[0] != nil {
		*affected[0], err = result.RowsAffected()
	}
	return err
}

func (b *Builder) Delete(affected ...*int64) error {
	if b.err != nil {
		return b.err
	}
	whereClause, whereArgs := b.buildWhere()

	query := fmt.Sprintf("DELETE FROM %s%s", b.table.TableName, whereClause)

	result, err := b.elm.Exec(query, whereArgs...)
	if err != nil {
		return err
	}

	if len(affected) > 0 && affected[0] != nil {
		*affected[0], err = result.RowsAffected()
	}
	return err
}
