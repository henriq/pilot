package domain

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func populatedConfigurationContext() ConfigurationContext {
	var context ConfigurationContext
	populateEveryField(reflect.ValueOf(&context).Elem())
	return context
}

func populateEveryField(destination reflect.Value) {
	switch destination.Kind() {
	case reflect.Struct:
		for fieldIndex := range destination.NumField() {
			if destination.Type().Field(fieldIndex).PkgPath != "" {
				continue
			}
			populateEveryField(destination.Field(fieldIndex))
		}
	case reflect.Slice:
		element := reflect.New(destination.Type().Elem()).Elem()
		populateEveryField(element)
		slice := reflect.MakeSlice(destination.Type(), 1, 1)
		slice.Index(0).Set(element)
		destination.Set(slice)
	case reflect.Map:
		key := reflect.New(destination.Type().Key()).Elem()
		populateEveryField(key)
		value := reflect.New(destination.Type().Elem()).Elem()
		populateEveryField(value)
		populated := reflect.MakeMap(destination.Type())
		populated.SetMapIndex(key, value)
		destination.Set(populated)
	case reflect.Pointer:
		pointee := reflect.New(destination.Type().Elem())
		populateEveryField(pointee.Elem())
		destination.Set(pointee)
	case reflect.String:
		// Must satisfy ValidateGitRefShape, which ResolveContext applies to
		// several of the string fields this fills.
		destination.SetString("fixture-value")
	case reflect.Int:
		destination.SetInt(1)
	case reflect.Bool:
		destination.SetBool(true)
	}
}

func assertNoSharedReferences(t *testing.T, path string, raw, resolved reflect.Value) {
	t.Helper()
	switch raw.Kind() {
	case reflect.Pointer:
		if raw.IsNil() || resolved.IsNil() {
			return
		}
		if raw.Pointer() == resolved.Pointer() {
			t.Errorf(
				"%s: resolved context still points at the raw value; extend the matching copy helper in context_resolver.go to copy it",
				path,
			)
			return
		}
		assertNoSharedReferences(t, path, raw.Elem(), resolved.Elem())
	case reflect.Slice:
		if raw.Len() == 0 || resolved.Len() == 0 {
			return
		}
		if raw.Pointer() == resolved.Pointer() {
			t.Errorf(
				"%s: resolved context still shares its backing array with the raw config; extend the matching copy helper in context_resolver.go to clone it",
				path,
			)
			return
		}
		for index := range min(raw.Len(), resolved.Len()) {
			assertNoSharedReferences(t, fmt.Sprintf("%s[%d]", path, index), raw.Index(index), resolved.Index(index))
		}
	case reflect.Map:
		if raw.Len() == 0 || resolved.Len() == 0 {
			return
		}
		if raw.Pointer() == resolved.Pointer() {
			t.Errorf(
				"%s: resolved context still shares its underlying map with the raw config; extend the matching copy helper in context_resolver.go to maps.Clone it",
				path,
			)
			return
		}
		// maps.Clone is shallow, so a cloned map of slices or pointers still
		// shares the values it maps to.
		for _, key := range raw.MapKeys() {
			resolvedValue := resolved.MapIndex(key)
			if !resolvedValue.IsValid() {
				continue
			}
			assertNoSharedReferences(
				t,
				fmt.Sprintf("%s[%v]", path, key.Interface()),
				raw.MapIndex(key),
				resolvedValue,
			)
		}
	case reflect.Struct:
		for fieldIndex := range raw.NumField() {
			field := raw.Type().Field(fieldIndex)
			if field.PkgPath != "" {
				continue
			}
			assertNoSharedReferences(t, path+"."+field.Name, raw.Field(fieldIndex), resolved.Field(fieldIndex))
		}
	case reflect.Bool, reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
	default:
		t.Errorf(
			"%s: this test does not inspect %s values for shared references; extend assertNoSharedReferences and populateEveryField",
			path,
			raw.Kind(),
		)
	}
}

// Reflection rather than named assertions so fields added after this test was
// written are covered too. Fields ResolveContext overwrites need no exemption:
// all of them are strings, which assertNoSharedReferences treats as leaves.
func TestResolveContext_DeepCopyIsolatesEveryReferenceTypedField(t *testing.T) {
	raw := populatedConfigurationContext()

	resolved, err := ResolveContext(raw, resolverTestHome, NoOverrides)
	require.NoError(t, err)

	assertNoSharedReferences(t, "ConfigurationContext", reflect.ValueOf(raw), reflect.ValueOf(resolved))
}
