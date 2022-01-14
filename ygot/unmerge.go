package ygot

import (
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

	if err := UnmergeStructFrom(base, unmergeStruct, schema); err != nil {
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
		return fmt.Errorf("cannot unmerge '%s' from '%s'", unmergeVal.Type().Name(), baseVal.Type().Name())
	}
	if !util.IsValueStruct(baseVal) || !util.IsValueStruct(unmergeVal) {
		return fmt.Errorf("cannot handle non-struct types, base: %v, unmerge: %v", baseVal.Type().Kind(), unmergeVal.Type().Kind())
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
			if err := unmergeInterfaceField(baseField, unmergeField, schema); err != nil {
				return err
			}
		case reflect.Map:
			if err := unmergeMapField(baseField, unmergeField, cschema); err != nil {
				return err
			}
		case reflect.Int64:
			if err := unmergeEnumField(baseField, unmergeField, cschema); err != nil {
				return err
			}
		case reflect.Slice:
			if err := unmergeSliceField(baseField, unmergeField, cschema); err != nil {
				return err
			}
		default:
			return fmt.Errorf("field: %s has unsupported type: %s", ft.Name, unmergeField.Type().Name())
		}
	}
	if util.IsKeyedList(schema) {
		// Get rid of struct attributes constituting the key if they are the only attributes left
		pruneStruct(baseVal, keyMap, fieldNameMap)
	}
	return nil
}

func unmergeEnumField(baseVal, unmergeVal reflect.Value, schema *yang.Entry) error {
	vBase, vUnmerge := baseVal.Int(), unmergeVal.Int()
	if vUnmerge == 0 {
		return nil
	}
	if vBase != vUnmerge {
		return fmt.Errorf("value of enum field '%s' in baseStruct differs from unmergeStruct, %d != %d", schema.Name, vBase, vUnmerge)
	}
	baseVal.Set(reflect.Zero(baseVal.Type()))
	return nil
}

func unmergeSliceField(baseVal, unmergeVal reflect.Value, schema *yang.Entry) error {
	if unmergeVal.Len() == 0 {
		return nil
	}
	if baseVal.Len() == 0 {
		return fmt.Errorf("base struct field: '%s' is empty", schema.Name)
	}
	if util.IsTypeStructPtr(unmergeVal.Type().Elem()) {
		return fmt.Errorf("lists without keys are not supported, failed while unmerging field: %s", schema.Name)
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

func unmergeInterfaceField(baseVal, unmergeVal reflect.Value, schema *yang.Entry) error {
	if !util.IsValueInterface(unmergeVal) {
		return fmt.Errorf("non-interface type: %T for field: '%s' in unmerge struct", unmergeVal.Interface(), schema.Name)
	}
	if !util.IsValueInterface(baseVal) {
		return fmt.Errorf("non-interface type: %T for field: '%s' in base struct", baseVal.Interface(), schema.Name)
	}
	if util.IsNilOrInvalidValue(unmergeVal) {
		return nil
	}
	if util.IsNilOrInvalidValue(baseVal) {
		return fmt.Errorf("base struct field: %s was not set", schema.Name)
	}

	_, isGoEnum := unmergeVal.Elem().Interface().(GoEnum)
	switch {
	case util.IsValueStructPtr(unmergeVal.Elem()):
		u := unmergeVal.Elem().Elem() // Dereference unmergeVal to a struct.
		b := baseVal.Elem().Elem()    // Dereference baseVal to a struct.
		if diff := cmp.Diff(b.Interface(), u.Interface()); diff != "" {
			return fmt.Errorf("interface field: '%s' in base and unmerge and was not equal, (-base, +unmerge):\n%s", schema.Name, diff)
		}
		baseVal.Set(reflect.Zero(baseVal.Type()))
		return nil
	case unmergeVal.Elem().Kind() == reflect.Slice && unmergeVal.Elem().Type().Name() == BinaryTypeName:
		fallthrough
	case util.IsValueScalar(unmergeVal.Elem()) && (isGoEnum || unionSingletonUnderlyingTypes[unmergeVal.Elem().Type().Name()] != nil):
		u, b := unmergeVal.Interface(), baseVal.Interface()
		if diff := cmp.Diff(b, u); diff != "" {
			return fmt.Errorf("interface field: '%s' in base and unmerge and was not equal, (-base, +unmerge):\n%s", schema.Name, diff)
		}
		baseVal.Set(reflect.Zero(baseVal.Type()))
		return nil
	}
	return fmt.Errorf("invalid interface type received: %T", unmergeVal.Interface())
}

func unmergeMapField(baseVal, unmergeVal reflect.Value, schema *yang.Entry) error {
	if !util.IsValueMap(unmergeVal) {
		return fmt.Errorf("received a non-map type: '%v' for field: '%s' in unmerge struct", unmergeVal.Kind(), schema.Name)
	}
	if !util.IsValueMap(baseVal) {
		return fmt.Errorf("received a non-map type: '%v' for field: '%s' in unmerge struct", baseVal.Kind(), schema.Name)
	}
	if unmergeVal.Len() == 0 {
		return nil
	}
	if baseVal.Len() == 0 {
		return fmt.Errorf("map field: '%s' in base struct is not populated", schema.Name)
	}

	_, err := validateMap(unmergeVal, baseVal)
	if err != nil {
		return err
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
			return fmt.Errorf("could not find key: %v in base struct", k.Interface())
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
		return false, fmt.Errorf("received non-ptr type: %v for field: '%s'", unmergeVal.Kind(), schema.Name)
	}

	if util.IsNilOrInvalidValue(baseVal) {
		return false, fmt.Errorf("base struct field: %s was not set", schema.Name)
	}

	// Check for struct ptr, or ptr to avoid panic.
	if util.IsValueStructPtr(unmergeVal) {
		// TODO(ythadhani): Should we add the IsZero check here as well to optimize for presence containers?
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
		return false, fmt.Errorf("base value was not equal to source value while unmerging ptr field, (-base, +unmerge):\n%s", diff)
	}
	return true, nil
}
