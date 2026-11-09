package asset

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// DecodeJSONObject decodes a flat JSON object into the struct T.
//
// The JSON keys are matched against the "csv" struct tags so that a table
// can be exported as CSV or as JSON without changing the schema. Key
// matching is case-insensitive. Unknown keys are ignored, which keeps the
// loader forward compatible with extended exports.
//
// Numbers are decoded through json.Number so that full float precision is
// preserved and integer fields reject fractional values explicitly.
func DecodeJSONObject[T any](data []byte, path string) (T, error) {
	var out T
	elemType := reflect.TypeOf(out)
	if elemType.Kind() != reflect.Struct {
		return out, fmt.Errorf("%s: target type %s is not a struct", path, elemType)
	}
	fields := structFields(elemType)

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var raw map[string]json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return out, fmt.Errorf("%s: parse json: %w", path, err)
	}

	// Lower-cased key -> field info, for case-insensitive matching.
	byLower := make(map[string]fieldInfo, len(fields))
	for _, fi := range fields {
		byLower[strings.ToLower(fi.name)] = fi
	}

	v := reflect.New(elemType).Elem()
	for key, cellRaw := range raw {
		fi, ok := byLower[strings.ToLower(strings.TrimSpace(key))]
		if !ok {
			continue // unknown key: ignore
		}
		cell := strings.TrimSpace(string(cellRaw))
		if cell == "null" {
			continue
		}
		if fi.multi {
			var items []string
			if err := json.Unmarshal(cellRaw, &items); err != nil {
				return out, fmt.Errorf("%s: key %q: %w", path, key, err)
			}
			v.Field(fi.index).Set(reflect.ValueOf(items))
			continue
		}
		if err := setJSONField(v.Field(fi.index), fi, cellRaw, path, key); err != nil {
			return out, err
		}
	}
	return v.Interface().(T), nil
}

// setJSONField converts a JSON value into the destination field.
func setJSONField(dst reflect.Value, fi fieldInfo, raw json.RawMessage, path, key string) error {
	fail := func(format string, args ...any) error {
		return fmt.Errorf("%s: key %q: %s", path, key, fmt.Sprintf(format, args...))
	}

	switch fi.kind {
	case reflect.String:
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return fail("expected a string")
		}
		dst.SetString(s)
	case reflect.Bool:
		var b bool
		if err := json.Unmarshal(raw, &b); err != nil {
			return fail("expected a boolean")
		}
		dst.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		var n json.Number
		if err := json.Unmarshal(raw, &n); err != nil {
			return fail("expected a number")
		}
		i, err := strconv.ParseInt(n.String(), 10, 64)
		if err != nil {
			return fail("expected an integer, got %q", n.String())
		}
		dst.SetInt(i)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		var n json.Number
		if err := json.Unmarshal(raw, &n); err != nil {
			return fail("expected a number")
		}
		u, err := strconv.ParseUint(n.String(), 10, 64)
		if err != nil {
			return fail("expected an unsigned integer, got %q", n.String())
		}
		dst.SetUint(u)
	case reflect.Float32, reflect.Float64:
		var n json.Number
		if err := json.Unmarshal(raw, &n); err != nil {
			return fail("expected a number")
		}
		f, err := n.Float64()
		if err != nil {
			return fail("expected a number, got %q", n.String())
		}
		dst.SetFloat(f)
	default:
		return fail("unsupported field type %s", fi.kind)
	}
	return nil
}
