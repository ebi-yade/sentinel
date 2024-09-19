package sentinel

import (
	"log/slog"
	"reflect"
)

// ReplaceAttr processes each attribute and zeroes out any fields marked with the `sentinel` tag.
func ReplaceAttr(groups []string, a slog.Attr) slog.Attr {
	if a.Value.Kind() == slog.KindAny {
		a.Value = processAny(a.Value.Any(), make(map[uintptr]bool))
	}
	return a
}

// processAny handles values, zeroing out sensitive fields.
func processAny(val interface{}, visited map[uintptr]bool) slog.Value {
	if val == nil {
		return slog.AnyValue(nil)
	}

	// Check if val implements slog.LogValuer
	if valuer, ok := val.(slog.LogValuer); ok {
		// Evaluate the LogValuer
		evaluated := valuer.LogValue()
		return evaluated.Resolve()
	}

	rv := reflect.ValueOf(val)

	// Handle pointers and interfaces
	for rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			return slog.AnyValue(nil)
		}

		// Cycle detection
		addr := rv.Pointer()
		if visited[addr] {
			return slog.AnyValue(rv.Interface())
		}
		visited[addr] = true

		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.Struct:
		return processStruct(rv, visited)
	case reflect.Slice, reflect.Array:
		return processSliceOrArray(rv, visited)
	case reflect.Map:
		return processMap(rv, visited)
	default:
		// For basic types, return the value as is
		return slog.AnyValue(rv.Interface())
	}
}

// processStruct processes struct types, zeroing out sensitive fields.
func processStruct(rv reflect.Value, visited map[uintptr]bool) slog.Value {
	rt := rv.Type()
	var attrs []slog.Attr

	for i := 0; i < rv.NumField(); i++ {
		structField := rt.Field(i)
		fieldValue := rv.Field(i)

		// Check if the field is exported
		if !fieldValue.CanInterface() {
			continue
		}

		fieldName := structField.Name

		// Check for the 'sentinel' tag
		if structField.Tag.Get("sentinel") != "" {
			// Zero out the sensitive field
			zeroValue := reflect.Zero(structField.Type).Interface()
			attrs = append(attrs, slog.Any(fieldName, zeroValue))
		} else {
			// Recursively process the field
			processedValue := processAny(fieldValue.Interface(), visited)
			attrs = append(attrs, slog.Any(fieldName, processedValue.Any()))
		}
	}

	return slog.GroupValue(attrs...)
}

// processSliceOrArray processes slices and arrays, recursively processing each element.
func processSliceOrArray(rv reflect.Value, visited map[uintptr]bool) slog.Value {
	if rv.IsNil() {
		return slog.AnyValue(nil)
	}

	length := rv.Len()
	newSlice := reflect.MakeSlice(rv.Type(), length, length)
	reflect.Copy(newSlice, rv)

	for i := 0; i < length; i++ {
		element := newSlice.Index(i)
		if !element.CanInterface() {
			continue
		}

		// Recursively process the element
		elementValue := element.Interface()
		processedElement := processAny(elementValue, visited)
		newValue := reflect.ValueOf(processedElement.Any())

		// Ensure type compatibility
		if newValue.Type().AssignableTo(element.Type()) {
			element.Set(newValue)
		}
	}

	return slog.AnyValue(newSlice.Interface())
}

// processMap processes map types, recursively processing keys and values.
func processMap(rv reflect.Value, visited map[uintptr]bool) slog.Value {
	if rv.IsNil() {
		return slog.AnyValue(nil)
	}

	newMap := reflect.MakeMapWithSize(rv.Type(), rv.Len())
	iter := rv.MapRange()

	for iter.Next() {
		key := iter.Key()
		value := iter.Value()

		// Process key
		var processedKey reflect.Value
		if key.CanInterface() {
			keyValue := key.Interface()
			processedKeyVal := processAny(keyValue, visited)
			processedKey = reflect.ValueOf(processedKeyVal.Any())
		} else {
			processedKey = key
		}

		// Process value
		var processedValue reflect.Value
		if value.CanInterface() {
			valueValue := value.Interface()
			processedValueVal := processAny(valueValue, visited)
			processedValue = reflect.ValueOf(processedValueVal.Any())
		} else {
			processedValue = value
		}

		// Ensure type compatibility
		if processedKey.Type().AssignableTo(key.Type()) && processedValue.Type().AssignableTo(value.Type()) {
			newMap.SetMapIndex(processedKey, processedValue)
		}
	}

	return slog.AnyValue(newMap.Interface())
}
