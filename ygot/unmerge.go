package ygot

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/goyang/pkg/yang"
	"github.com/openconfig/ygot/util"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func UnmergeStructs(baseStruct, unmergeStruct ValidatedGoStruct, schema *yang.Entry) (ValidatedGoStruct, error) {
	if reflect.TypeOf(baseStruct) != reflect.TypeOf(unmergeStruct) {
		return nil, fmt.Errorf("cannot unmerge structs that are not of matching types, %T != %T", baseStruct, unmergeStruct)
	}
	tn, err := DeepCopy(baseStruct)
	if err != nil {
		return nil, err
	}
	// This conversion is safe as DeepCopy will use the same underlying type as
	// `baseStruct`, which was passed in as a ValidatedGoStruct.
	base := tn.(ValidatedGoStruct)

	u, err := DeepCopy(unmergeStruct)
	if err != nil {
		return nil, err
	}
	// This conversion is safe as DeepCopy will use the same underlying type as
	// `unmergeStruct`, which was passed in as a ValidatedGoStruct.
	unmerge := u.(ValidatedGoStruct)

	if err := UnmergeStructFrom(base, unmerge, schema); err != nil {
		return nil, fmt.Errorf("error unmerging unmergeStruct from baseStruct: %v", err)
	}
	return base, nil
}

func UnmergeStructFrom(baseStruct, unmergeStruct ValidatedGoStruct, schema *yang.Entry) error {
	if reflect.TypeOf(baseStruct) != reflect.TypeOf(unmergeStruct) {
		return fmt.Errorf("cannot unmerge structs that are not of matching types, %T != %T", baseStruct, unmergeStruct)
	}
	return unmergeStructs(reflect.ValueOf(baseStruct).Elem(), reflect.ValueOf(unmergeStruct).Elem(), schema)
}

func unmergeStructs(baseVal, unmergeVal reflect.Value, schema *yang.Entry) error {
	if unmergeVal.Type() != baseVal.Type() {
		return fmt.Errorf("field: '%s': cannot unmerge '%s' from '%s'", schema.Name, unmergeVal.Type().Name(), baseVal.Type().Name())
	}
	if !util.IsValueStruct(baseVal) || !util.IsValueStruct(unmergeVal) {
		return fmt.Errorf("field: '%s': cannot handle non-struct types, base: %v, unmerge: %v", schema.Name, baseVal.Type().Kind(), unmergeVal.Type().Kind())
	}

	keyMap := map[string]struct{}{}
	if util.IsKeyedList(schema) {
		keys := strings.Split(schema.Key, " ")
		for _, key := range keys {
			keyMap[key] = struct{}{}
		}
	}

	fieldNameMap := map[string]string{}
	for i := 0; i < unmergeVal.NumField(); i++ {
		unmergeField := unmergeVal.Field(i)
		baseField := baseVal.Field(i)

		ft := unmergeVal.Type().Field(i)

		// TODO(ythadhani): Do we need to care about IsYgotAnnotation?
		cschema, err := util.ChildSchema(schema, ft)
		if !util.IsYgotAnnotation(ft) {
			switch {
			case err != nil:
				return status.Errorf(codes.Unknown, "failed to get child schema for %T, field %s: %s", unmergeVal.Interface(), ft.Name, err)
			case cschema == nil:
				return status.Errorf(codes.InvalidArgument, "could not find schema for type %T, field %s", unmergeVal.Interface(), ft.Name)
			}
		}

		fieldNameMap[ft.Name] = cschema.Name

		if _, present := keyMap[cschema.Name]; present {
			continue
		}

		switch unmergeField.Kind() {
		case reflect.Ptr:
			resetPtr, err := unmergePtrField(baseField, unmergeField, cschema)
			if err != nil {
				return err
			}
			if resetPtr {
				baseField.Set(reflect.Zero(baseField.Type()))
			}
		case reflect.Interface:
			if err := unmergeInterfaceField(baseField, unmergeField, cschema.Name); err != nil {
				return err
			}
		case reflect.Map:
			if err := unmergeMapField(baseField, unmergeField, cschema); err != nil {
				return err
			}
		case reflect.Int64:
			if err := unmergeEnumField(baseField, unmergeField, cschema.Name); err != nil {
				return err
			}
		case reflect.Slice:
			if err := unmergeSliceField(baseField, unmergeField, cschema.Name); err != nil {
				return err
			}
		default:
			if diff := cmp.Diff(baseField.Interface(), unmergeField.Interface()); diff != "" {
				return fmt.Errorf("field: '%s' in base and unmerge structs was not equal, (-base, +unmerge):\n%s", cschema.Name, diff)
			}
			baseField.Set(reflect.Zero(baseField.Type()))
		}
	}
	if util.IsKeyedList(schema) {
		// Get rid of struct attributes constituting the key if they are the only attributes left
		pruneStruct(baseVal, keyMap, fieldNameMap)
	}
	return nil
}

func unmergeEnumField(baseVal, unmergeVal reflect.Value, fieldName string) error {
	vBase, vUnmerge := baseVal.Int(), unmergeVal.Int()
	if vUnmerge == 0 {
		return nil
	}
	if vBase != vUnmerge {
		return fmt.Errorf("value of enum field: '%s' in baseStruct differs from unmergeStruct, %d != %d", fieldName, vBase, vUnmerge)
	}
	baseVal.Set(reflect.Zero(baseVal.Type()))
	return nil
}

func unmergeSliceField(baseVal, unmergeVal reflect.Value, fieldName string) error {
	if unmergeVal.Len() == 0 {
		return nil
	}
	if baseVal.Len() == 0 {
		return fmt.Errorf("base struct field: '%s' is empty", fieldName)
	}
	if util.IsTypeStructPtr(unmergeVal.Type().Elem()) {
		return fmt.Errorf("lists without keys are not supported, failed while unmerging field: %s", fieldName)
	}

	// TODO(ythadhani): Do we need to support Annotations?
	if !cmp.Equal(unmergeVal.Interface(), baseVal.Interface()) {
		return nil
	}
	baseVal.Set(reflect.Zero(baseVal.Type()))
	return nil
}

func pruneStruct(structVal reflect.Value, keyMap map[string]struct{}, fieldNameMap map[string]string) {
	keyFieldVals := []reflect.Value{}
	for i := 0; i < structVal.NumField(); i++ {
		structField := structVal.Field(i)
		if !structField.IsZero() {
			ft := structVal.Type().Field(i)
			fieldName := fieldNameMap[ft.Name]
			if _, present := keyMap[fieldName]; !present {
				return
			} else {
				keyFieldVals = append(keyFieldVals, structField)
			}
		}
	}
	for _, keyFieldVal := range keyFieldVals {
		keyFieldVal.Set(reflect.Zero(keyFieldVal.Type()))
	}
	return
}

func unmergeInterfaceField(baseVal, unmergeVal reflect.Value, fieldName string) error {
	if !util.IsValueInterface(unmergeVal) {
		return fmt.Errorf("non-interface type: %T for field: '%s' in unmerge struct", unmergeVal.Interface(), fieldName)
	}
	if !util.IsValueInterface(baseVal) {
		return fmt.Errorf("non-interface type: %T for field: '%s' in base struct", baseVal.Interface(), fieldName)
	}
	if util.IsNilOrInvalidValue(unmergeVal) {
		return nil
	}
	if util.IsNilOrInvalidValue(baseVal) {
		return fmt.Errorf("base struct interface field: %s was not set", fieldName)
	}

	_, isGoEnum := unmergeVal.Elem().Interface().(GoEnum)
	switch {
	case util.IsValueStructPtr(unmergeVal.Elem()):
		u := unmergeVal.Elem().Elem() // Dereference unmergeVal to a struct.
		b := baseVal.Elem().Elem()    // Dereference baseVal to a struct.
		if diff := cmp.Diff(b.Interface(), u.Interface()); diff != "" {
			return fmt.Errorf("interface field: '%s' in base and unmerge and was not equal, (-base, +unmerge):\n%s", fieldName, diff)
		}
		baseVal.Set(reflect.Zero(baseVal.Type()))
		return nil
	case unmergeVal.Elem().Kind() == reflect.Slice && unmergeVal.Elem().Type().Name() == BinaryTypeName:
		fallthrough
	case util.IsValueScalar(unmergeVal.Elem()) && (isGoEnum || unionSingletonUnderlyingTypes[unmergeVal.Elem().Type().Name()] != nil):
		u, b := unmergeVal.Interface(), baseVal.Interface()
		if diff := cmp.Diff(b, u); diff != "" {
			return fmt.Errorf("interface field: '%s' in base and unmerge and was not equal, (-base, +unmerge):\n%s", fieldName, diff)
		}
		baseVal.Set(reflect.Zero(baseVal.Type()))
		return nil
	}
	return fmt.Errorf("invalid interface type: %T received for field: '%s'", unmergeVal.Interface(), fieldName)
}

func unmergeMapField(baseVal, unmergeVal reflect.Value, schema *yang.Entry) error {
	_, err := validateMap(baseVal, unmergeVal, "base", "unmerge")
	if err != nil {
		return fmt.Errorf("%s: %s", schema.Name, err.Error())
	}
	if unmergeVal.Len() == 0 {
		return nil
	}
	if baseVal.Len() == 0 {
		return fmt.Errorf("map field: '%s' in base struct is not populated", schema.Name)
	}

	baseKeys := map[interface{}]bool{}
	for _, k := range baseVal.MapKeys() {
		baseKeys[k.Interface()] = true
	}

	for _, k := range unmergeVal.MapKeys() {
		v := unmergeVal.MapIndex(k)
		d := reflect.New(v.Elem().Type())
		// TODO(ythadhani): Do we need to maintain dstKeys, can't we directly check if v is zero value
		if _, ok := baseKeys[k.Interface()]; !ok {
			return fmt.Errorf("could not find key: %v in base map field: '%s'", k.Interface(), schema.Name)
		}
		d = baseVal.MapIndex(k)
		if err := unmergeStructs(d.Elem(), v.Elem(), schema); err != nil {
			return err
		}
		if d.Elem().IsZero() {
			baseVal.SetMapIndex(k, reflect.Value{})
			if baseVal.Len() == 0 {
				baseVal.Set(reflect.Zero(baseVal.Type()))
			}
		}
	}
	return nil
}

func unmergePtrField(baseVal, unmergeVal reflect.Value, schema *yang.Entry) (bool, error) {
	if util.IsNilOrInvalidValue(unmergeVal) {
		return false, nil
	}
	if !util.IsValuePtr(unmergeVal) {
		return false, fmt.Errorf("received non-ptr type: %v for unmerge struct field: '%s'", unmergeVal.Kind(), schema.Name)
	}
	if util.IsNilOrInvalidValue(baseVal) {
		unmergeValJs, err := json.Marshal(unmergeVal.Interface())
		if err != nil {
			return false, nil
		}
		return false, fmt.Errorf("base struct field: %s was not set but unmerge struct field was set to: %s", schema.Name, string(unmergeValJs))
	}

	// Check for struct ptr, or ptr to avoid panic.
	if util.IsValueStructPtr(unmergeVal) {
		if unmergeVal.Elem().IsZero() {
			if baseVal.Elem().IsZero() {
				return true, nil
			}
			return false, nil
		}
		if err := unmergeStructs(baseVal.Elem(), unmergeVal.Elem(), schema); err != nil {
			return false, err
		}
		if baseVal.Elem().IsZero() {
			return true, nil
		}
		return false, nil
	}

	b, u := baseVal.Elem().Interface(), unmergeVal.Elem().Interface()
	if diff := cmp.Diff(b, u); diff != "" {
		return false, fmt.Errorf("base value was not equal to source value while unmerging ptr field: '%s', (-base, +unmerge):\n%s", schema.Name, diff)
	}
	return true, nil
}
