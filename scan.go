package elm

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"
	"unicode"
)

func reflectRealTypeOf(v any) reflect.Type {
	t := reflect.TypeOf(v)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}

func reflectRealValueOf(v any) reflect.Value {
	t := reflect.ValueOf(v)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}

var (
	structCache   = make(map[reflect.Type]*tableInfo)
	structCacheMu sync.RWMutex
)

type tableInfo struct {
	Type            reflect.Type
	TableName       string
	ModelName       string
	Fields          []tableFieldInfo
	FieldsByColumn  map[string]*tableFieldInfo
	FieldsByName    map[string]*tableFieldInfo
	Relations       []tableRelationInfo
	RelationsByName map[string]*tableRelationInfo
}

type tableFieldInfo struct {
	Name      string
	Column    string
	Type      reflect.Type
	Nullable  bool
	IsPrimary bool
}

type tableRelationInfo struct {
	TableType         reflect.Type
	ModelRelationName string
	Nullable          bool
}

func getStructOrNil(v any) *tableInfo {
	t := reflectRealTypeOf(v)
	if t.Kind() != reflect.Struct {
		panic(fmt.Sprintf("elm: %T is not a struct", v))
	}
	structCacheMu.RLock()
	info, ok := structCache[t]
	structCacheMu.RUnlock()
	if ok {
		return info
	}
	return nil
}

func scanStruct(v any, cache bool) *tableInfo {
	t := reflectRealTypeOf(v)
	if t.Kind() != reflect.Struct {
		panic(fmt.Sprintf("elm: %T is not a struct", v))
	}

	structCacheMu.RLock()
	info, ok := structCache[t]
	structCacheMu.RUnlock()
	if ok {
		return info
	}

	info = &tableInfo{
		Type:            t,
		ModelName:       t.Name(),
		TableName:       toTableName(t.Name()),
		FieldsByColumn:  make(map[string]*tableFieldInfo),
		FieldsByName:    make(map[string]*tableFieldInfo),
		RelationsByName: make(map[string]*tableRelationInfo),
	}

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)

		if !f.IsExported() {
			continue
		}

		isRelation := (f.Type.Kind() == reflect.Struct && !isScalarStruct(f.Type)) ||
			(f.Type.Kind() == reflect.Pointer && f.Type.Elem().Kind() == reflect.Struct && !isScalarStruct(f.Type.Elem()))
		if isRelation {
			relType := f.Type
			nullable := false
			if relType.Kind() == reflect.Pointer {
				relType = relType.Elem()
				nullable = true
			}

			relation := tableRelationInfo{
				TableType:         scanStruct(reflect.New(relType).Interface(), cache).Type,
				ModelRelationName: f.Name,
				Nullable:          nullable,
			}
			info.Relations = append(info.Relations, relation)
			info.RelationsByName[f.Name] = &info.Relations[len(info.Relations)-1]
			continue
		}

		col := toSnakeCase(f.Name)

		fi := tableFieldInfo{
			Name:      f.Name,
			Column:    col,
			Type:      f.Type,
			Nullable:  f.Type.Kind() == reflect.Pointer,
			IsPrimary: f.Name == "ID",
		}

		info.Fields = append(info.Fields, fi)
		info.FieldsByColumn[col] = &info.Fields[len(info.Fields)-1]
		info.FieldsByName[f.Name] = &info.Fields[len(info.Fields)-1]
	}

	if cache {
		structCacheMu.Lock()
		if existing, ok := structCache[t]; ok {
			structCacheMu.Unlock()
			return existing
		}
		structCache[t] = info
		structCacheMu.Unlock()
	}

	return info
}

func scanTemporaryStruct(v any) tableInfo {
	return *scanStruct(v, false)
}

func scanAndCacheStruct(v any) *tableInfo {
	return scanStruct(v, true)
}

func toSnakeCase(s string) string {
	var b strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		if unicode.IsUpper(r) && i > 0 {
			prev := runes[i-1]
			next := rune(0)
			if i+1 < len(runes) {
				next = runes[i+1]
			}
			if unicode.IsLower(prev) || (next != 0 && unicode.IsLower(next)) {
				b.WriteRune('_')
			}
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

func toTableName(name string) string {
	snake := toSnakeCase(name)
	if strings.HasSuffix(snake, "s") {
		return snake + "es"
	}
	return snake + "s"
}

func isScalarStruct(t reflect.Type) bool {
	scalarTypes := []reflect.Type{
		reflect.TypeOf(time.Time{}),
		reflect.TypeOf(sql.NullString{}),
		reflect.TypeOf(sql.NullFloat64{}),
		reflect.TypeOf(sql.NullBool{}),
		reflect.TypeOf(sql.NullTime{}),
		reflect.TypeOf(sql.NullByte{}),
		reflect.TypeOf(sql.NullInt16{}),
		reflect.TypeOf(sql.NullInt32{}),
		reflect.TypeOf(sql.NullInt64{}),
	}
	for _, st := range scalarTypes {
		if t == st {
			return true
		}
	}
	return false
}

type nullProxy struct {
	field reflect.Value
}

func (p *nullProxy) Scan(src any) error {
	if src == nil {
		p.field.Set(reflect.Zero(p.field.Type()))
		return nil
	}

	target := p.field

	if target.Kind() == reflect.Pointer {
		target.Set(reflect.New(target.Type().Elem()))
		target = target.Elem()
	}

	return convertAssign(target.Addr().Interface(), src)
}

func convertAssign(dest, src any) error {
	if s, ok := dest.(sql.Scanner); ok {
		return s.Scan(src)
	}
	rv := reflect.ValueOf(dest).Elem()
	sv := reflect.ValueOf(src)

	if sv.IsValid() && sv.Type().AssignableTo(rv.Type()) {
		rv.Set(sv)
		return nil
	}
	if sv.IsValid() && sv.Type().ConvertibleTo(rv.Type()) {
		rv.Set(sv.Convert(rv.Type()))
		return nil
	}
	// Handle integer → bool conversion
	if sv.IsValid() && rv.Kind() == reflect.Bool {
		switch sv.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			rv.SetBool(sv.Int() != 0)
			return nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			rv.SetBool(sv.Uint() != 0)
			return nil
		}
	}
	return fmt.Errorf("elm: cannot assign %T → %T", src, dest)
}
