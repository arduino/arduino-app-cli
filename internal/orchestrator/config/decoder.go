// This file is part of arduino-app-cli.
//
// Copyright 2025 ARDUINO SA (http://www.arduino.cc/)
//
// This software is released under the GNU General Public License version 3,
// which covers the main part of arduino-app-cli.
// The terms of this license can be found at:
// https://www.gnu.org/licenses/gpl-3.0.en.html
//
// You can be released from the requirements of the above licenses by purchasing
// a commercial license. Buying such a license is mandatory if you want to
// modify or otherwise use the software for commercial activities involving the
// Arduino software without disclosing the source code of your own applications.
// To purchase a commercial license, send an email to license@arduino.cc.

package config

import (
	"encoding"
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode"
)

var textUnmarshalerType = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()
var timeDurationType = reflect.TypeOf(time.Duration(0))
var urlURLType = reflect.TypeOf(url.URL{})

const unmarshalTextMethodName = "UnmarshalText"

func decode(prefix []string, getValue Parser, config any) error {
	s := reflect.ValueOf(config).Elem()
	if s.Kind() != reflect.Struct {
		return fmt.Errorf("support only struct, %v", s.Kind())
	}

	for i := 0; i < s.NumField(); i++ {
		structField := s.Type().Field(i)
		name := structField.Name

		if unicode.IsLower(rune(name[0])) {
			continue
		}

		field := s.Field(i)
		key := append(append([]string{}, prefix...), name)
		keyStr := strings.Join(key, ".")

		// Ora 'key' è ["AppsDir"], poi ["DataDir"], ecc.
		v, err := getValue(key...)

		if err != nil || v == nil {
			// Se è una struct (es: una sottostruttura di config), scendiamo
			if field.Kind() == reflect.Struct {
				if err := decode(key, getValue, field.Addr().Interface()); err != nil {
					return err
				}
			}
			continue
		}

		if field.Addr().Type().Implements(textUnmarshalerType) {
			s, ok := v.(string)
			if !ok {
				return fmt.Errorf("config %q: expected string for unmarshaler, got %T", keyStr, v)
			}
			method := field.Addr().MethodByName(unmarshalTextMethodName)
			results := method.Call([]reflect.Value{reflect.ValueOf([]byte(s))})
			if !results[0].IsNil() {
				return fmt.Errorf("config %q: unmarshal error: %w", keyStr, results[0].Interface().(error))
			}
			continue
		}

		if field.Type() == timeDurationType {
			s, ok := v.(string)
			if !ok {
				return fmt.Errorf("config %q: expected string for duration, got %T", keyStr, v)
			}
			d, err := time.ParseDuration(s)
			if err != nil {
				return fmt.Errorf("config %q: invalid duration: %w", keyStr, err)
			}
			field.SetInt(int64(d))
			continue
		}

		if field.Type() == urlURLType {
			s, ok := v.(string)
			if !ok {
				return fmt.Errorf("config %q: expected string for URL, got %T", keyStr, v)
			}
			u, err := url.Parse(s)
			if err != nil {
				return fmt.Errorf("config %q: invalid URL: %w", keyStr, err)
			}
			field.Set(reflect.ValueOf(*u))
			continue
		}

		if field.Kind() == reflect.Struct {
			if err := decode(key, getValue, field.Addr().Interface()); err != nil {
				return err
			}
			continue
		}

		switch field.Kind() {
		case reflect.String:
			s, ok := v.(string)
			if !ok {
				return fmt.Errorf("config %q: expected string, got %T", keyStr, v)
			}
			field.SetString(s)

		case reflect.Int, reflect.Int32, reflect.Int64:
			if s, ok := v.(string); ok {
				d, err := strconv.ParseInt(s, 10, 64)
				if err != nil {
					return fmt.Errorf("config %q: %q is not a valid integer", keyStr, s)
				}
				field.SetInt(d)
			} else if reflect.ValueOf(v).Type().ConvertibleTo(field.Type()) {
				field.Set(reflect.ValueOf(v).Convert(field.Type()))
			} else {
				return fmt.Errorf("config %q: cannot assign %T to int", keyStr, v)
			}

		case reflect.Uint, reflect.Uint32, reflect.Uint64:
			if s, ok := v.(string); ok {
				d, err := strconv.ParseUint(s, 10, 64)
				if err != nil {
					return fmt.Errorf("config %q: %q is not a valid unsigned integer", keyStr, s)
				}
				field.SetUint(d)
			} else if reflect.ValueOf(v).Type().ConvertibleTo(field.Type()) {
				field.Set(reflect.ValueOf(v).Convert(field.Type()))
			} else {
				return fmt.Errorf("config %q: cannot assign %T to uint", keyStr, v)
			}

		case reflect.Bool:
			if s, ok := v.(string); ok {
				b, err := strconv.ParseBool(s)
				if err != nil {
					if structField.Tag.Get("lenient") == "true" {
						field.SetBool(false)
						continue
					}
					return fmt.Errorf("config %q: %q is not a valid boolean", keyStr, s)
				}
				field.SetBool(b)
			} else if b, ok := v.(bool); ok {
				field.SetBool(b)
			} else {
				return fmt.Errorf("config %q: cannot assign %T to bool", keyStr, v)
			}

		case reflect.Float32, reflect.Float64:
			if s, ok := v.(string); ok {
				f, err := strconv.ParseFloat(s, 64)
				if err != nil {
					return fmt.Errorf("config %q: %q is not a valid float", keyStr, s)
				}
				field.SetFloat(f)
			} else if reflect.ValueOf(v).Type().ConvertibleTo(field.Type()) {
				field.Set(reflect.ValueOf(v).Convert(field.Type()))
			} else {
				return fmt.Errorf("config %q: cannot assign %T to float", keyStr, v)
			}

		case reflect.Slice:
			if sStr, ok := v.(string); ok {
				slice, err := stringToSlice(name, field, sStr)
				if err != nil {
					return fmt.Errorf("config %q: %w", keyStr, err)
				}
				field.Set(slice)
			} else {
				rv := reflect.ValueOf(v)
				if rv.Kind() == reflect.Slice {
					field.Set(rv)
				} else {
					return fmt.Errorf("config %q: expected slice or comma-separated string, got %T", keyStr, v)
				}
			}

		default:
			return fmt.Errorf("field %s has unsupported kind %s", name, field.Kind())
		}
	}

	return nil
}

func stringToSlice(name string, field reflect.Value, val string) (reflect.Value, error) {
	slice := strings.Split(val, ",")
	sliceValue := reflect.MakeSlice(field.Type(), len(slice), len(slice))
	for i, v := range slice {
		elem := strings.TrimSpace(v)
		switch field.Type().Elem().Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			intVal, err := strconv.ParseInt(elem, 10, 64)
			if err != nil {
				return reflect.Value{}, fmt.Errorf("value %q for %q is not a int: %w", elem, name, err)
			}
			sliceValue.Index(i).Set(reflect.ValueOf(intVal).Convert(field.Type().Elem()))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			intVal, err := strconv.ParseUint(elem, 10, 64)
			if err != nil {
				return reflect.Value{}, fmt.Errorf("value %q for %q is not a uint: %w", elem, name, err)
			}
			sliceValue.Index(i).Set(reflect.ValueOf(intVal).Convert(field.Type().Elem()))
		case reflect.Float32, reflect.Float64:
			floatVal, err := strconv.ParseFloat(elem, 64)
			if err != nil {
				return reflect.Value{}, fmt.Errorf("value %q for %q is not a float: %w", elem, name, err)
			}
			sliceValue.Index(i).Set(reflect.ValueOf(floatVal).Convert(field.Type().Elem()))
		case reflect.String:
			sliceValue.Index(i).Set(reflect.ValueOf(elem).Convert(field.Type().Elem()))
		case reflect.Bool:
			boolVal, err := strconv.ParseBool(elem)
			if err != nil {
				return reflect.Value{}, fmt.Errorf("value %q for %q is not a boolean: %w", elem, name, err)
			}
			sliceValue.Index(i).Set(reflect.ValueOf(boolVal).Convert(field.Type().Elem()))
		default:
			return reflect.Value{}, fmt.Errorf("field %s has unsupported kind %s", name, field.Kind())
		}
	}
	return sliceValue, nil
}
