package governedprosekit

import (
	"reflect"
	"unicode/utf8"
)

// sourcedUTF8Valid checks the exported strings in this package's data structs,
// including named strings, nested provenance and map keys. encoding/json silently
// replaces invalid UTF-8; copying or hashing must never change an identity that way.
// This is not a general JSON validator. Unexported internals (notably time.Time's
// location) are not wire fields; JSON encoding still validates other value types.
func sourcedUTF8Valid(v any) bool {
	return sourcedUTF8ValueValid(reflect.ValueOf(v), 0)
}

func sourcedUTF8ValueValid(v reflect.Value, depth int) bool {
	// Our DTOs are acyclic and shallow. Bound recursion if a future field changes
	// that, rather than allowing a cycle to exhaust the stack before JSON runs.
	if depth > 32 {
		return false
	}
	if !v.IsValid() {
		return true
	}
	switch v.Kind() {
	case reflect.String:
		return utf8.ValidString(v.String())
	case reflect.Pointer, reflect.Interface:
		return v.IsNil() || sourcedUTF8ValueValid(v.Elem(), depth+1)
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() && !sourcedUTF8ValueValid(v.Field(i), depth+1) {
				return false
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			if !sourcedUTF8ValueValid(v.Index(i), depth+1) {
				return false
			}
		}
	case reflect.Map:
		it := v.MapRange()
		for it.Next() {
			if !sourcedUTF8ValueValid(it.Key(), depth+1) || !sourcedUTF8ValueValid(it.Value(), depth+1) {
				return false
			}
		}
	}
	return true
}
