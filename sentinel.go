package sentinel

import (
	"log/slog"
	"reflect"
)

// ReplaceAttr processes each attribute and zeroes out any fields marked with the `sentinel` tag.
func ReplaceAttr(groups []string, a slog.Attr) slog.Attr {
	a.Value = processValue(a.Value, make(map[uintptr]bool))
	return a
}

// processValue recursively processes a slog.Value, handling different kinds appropriately.
func processValue(v slog.Value, visited map[uintptr]bool) slog.Value {
	switch v.Kind() {
	case slog.KindAny:
		return processAny(v.Any(), visited)
	case slog.KindGroup:
		// Process each Attr in the group
		attrs := v.Group()
		for i, attr := range attrs {
			attrs[i].Value = processValue(attr.Value, visited)
		}
		return slog.GroupValue(attrs...)
	case slog.KindLogValuer:
		// Evaluate the LogValuer and process the resulting Value
		evaluated := v.LogValuer().LogValue()
		return processValue(evaluated, visited)
	default:
		// For other kinds, return the value as is
		return v
	}
}

// processAny handles values of KindAny, which can be any Go value.
func processAny(val interface{}, visited map[uintptr]bool) slog.Value {
	if val == nil {
		return slog.AnyValue(nil)
	}

	// Check if val implements slog.LogValuer
	if valuer, ok := val.(slog.LogValuer); ok {
		evaluated := valuer.LogValue()
		return processValue(evaluated, visited)
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

		// Check if rv.Interface() implements slog.LogValuer
		if valuer, ok := rv.Interface().(slog.LogValuer); ok {
			evaluated := valuer.LogValue()
			return processValue(evaluated, visited)
		}

		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.Struct:
		return processStruct(rv, visited)
	case reflect.Slice, reflect.Array:
		return processSliceOrArray(rv, visited)
	case reflect.Map:
		return processMap(rv, visited)
	case reflect.Pointer:
		// Should not reach here due to earlier handling, but included for completeness
		return processPointer(rv, visited)
	default:
		// For basic types, return the value as is
		return slog.AnyValue(rv.Interface())
	}
}

// processStruct processes struct types, zeroing out sensitive fields.
func processStruct(rv reflect.Value, visited map[uintptr]bool) slog.Value {
	rt := rv.Type()

	// Create a copy of the original struct
	newStruct := reflect.New(rt).Elem()
	newStruct.Set(rv)

	for i := 0; i < rv.NumField(); i++ {
		structField := rt.Field(i)
		newField := newStruct.Field(i)

		// Check if the field is exported
		if !newField.CanInterface() {
			// Cannot access unexported fields
			continue
		}

		// Check for the 'sentinel' tag
		if structField.Tag.Get("sentinel") != "" {
			if newField.CanSet() {
				// Zero out the sensitive field
				zeroValue := reflect.Zero(newField.Type())
				newField.Set(zeroValue)
			}
		} else {
			// Recursively process the field
			fieldValue := newField.Interface()
			processedValue := processAny(fieldValue, visited)
			newValue := reflect.ValueOf(processedValue.Any())

			// Ensure type compatibility and that the field is settable
			if newValue.Type().AssignableTo(structField.Type) && newField.CanSet() {
				newField.Set(newValue)
			}
		}
	}

	return slog.AnyValue(newStruct.Interface())
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

// processPointer processes pointer types, handling cycles and recursion.
func processPointer(rv reflect.Value, visited map[uintptr]bool) slog.Value {
	if rv.IsNil() {
		return slog.AnyValue(nil)
	}

	// Cycle detection
	addr := rv.Pointer()
	if visited[addr] {
		return slog.AnyValue(rv.Interface())
	}
	visited[addr] = true

	processedValue := processAny(rv.Elem().Interface(), visited)
	newPtr := reflect.New(rv.Type().Elem())
	newValue := reflect.ValueOf(processedValue.Any())

	// Ensure type compatibility
	if newValue.Type().AssignableTo(rv.Type().Elem()) {
		newPtr.Elem().Set(newValue)
	}

	return slog.AnyValue(newPtr.Interface())
}
