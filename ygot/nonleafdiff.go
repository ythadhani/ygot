package ygot

import (
	"fmt"
	"reflect"

	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ygot/util"
)

func OptimizedDiff(original, modified GoStruct) (*gnmipb.Notification, error) {
	if reflect.TypeOf(original) != reflect.TypeOf(modified) {
		return nil, fmt.Errorf("cannot diff structs of different types, original: %T, modified: %T", original, modified)
	}
	gnmiNotif := &gnmipb.Notification{}
	err := diffField(&util.NodeInfo{FieldValue: reflect.ValueOf(original)},
		&util.NodeInfo{FieldValue: reflect.ValueOf(modified)}, gnmiNotif)
	if err != nil {
		return nil, err
	}
	return gnmiNotif, nil
}

func diffField(origNI, modNI *util.NodeInfo, notif *gnmipb.Notification) error {
	var err error
	if util.IsNilOrInvalidValue(origNI.FieldValue) && util.IsNilOrInvalidValue(modNI.FieldValue) {
		return nil
	}

	switch {
	case util.IsValuePtr(origNI.FieldValue):
		err = diffPtr(origNI, modNI, notif)
	case util.IsValueStruct(origNI.FieldValue):
		err = diffStructs(origNI, modNI, notif)
	case util.IsValueMap(origNI.FieldValue):
		err = diffMaps(origNI, modNI, notif)
	default:
		err = diffDefault(origNI, modNI, notif)
	}
	return err
}

func diffPtr(origNI, modNI *util.NodeInfo, notif *gnmipb.Notification) error {
	if util.IsNilOrInvalidValue(origNI.FieldValue) {
		pathSpec, err := generatePathSpec(modNI)
		if err != nil {
			return err
		}
		notif.Update, err = appendUpdate(notif.Update, pathSpec, modNI.FieldValue.Interface())
		if err != nil {
			return err
		}
	} else if util.IsNilOrInvalidValue(modNI.FieldValue) {
		pathSpec, err := generatePathSpec(origNI)
		if err != nil {
			return err
		}
		notif.Delete = append(notif.Delete, pathSpec.gNMIPaths...)
	} else {
		origNI.FieldValue = origNI.FieldValue.Elem()
		modNI.FieldValue = modNI.FieldValue.Elem()
		err := diffField(origNI, modNI, notif)
		if err != nil {
			return err
		}
	}
	return nil
}

func diffStructs(origNI, modNI *util.NodeInfo, notif *gnmipb.Notification) error {
	vOrig := origNI.FieldValue
	vMod := modNI.FieldValue
	tOrig := vOrig.Type()
	tMod := vMod.Type()

	if vOrig.IsZero() && vMod.IsZero() {
		return nil
	}

	for i := 0; i < tOrig.NumField(); i++ {
		sfOrig := tOrig.Field(i)
		origNN := &util.NodeInfo{
			Parent:      origNI,
			StructField: sfOrig,
			FieldValue:  reflect.Zero(sfOrig.Type),
		}
		origNN.FieldValue = vOrig.Field(i)

		sfMod := tMod.Field(i)
		modNN := &util.NodeInfo{
			Parent:      modNI,
			StructField: sfMod,
			FieldValue:  reflect.Zero(sfMod.Type),
		}
		modNN.FieldValue = vMod.Field(i)

		ps, err := util.SchemaPaths(origNN.StructField)
		if err != nil {
			return err
		}

		for _, p := range ps {
			origNN.PathFromParent = p
			modNN.PathFromParent = p
			if util.IsTypeSlice(sfOrig.Type) || util.IsTypeMap(sfOrig.Type) {
				// Since lists can have path compression - where the path contains more
				// than one element, ensure that the schema path we received is only two
				// elements long. This protects against compression errors where there are
				// trailing spaces (e.g., a path tag of config/bar/).
				modNN.PathFromParent = p[0:1]
				origNN.PathFromParent = p[0:1]
			}
			pathSpec, err := generatePathSpec(origNN)
			if err != nil {
				return err
			}
			origNN.Annotation = []interface{}{pathSpec}
			modNN.Annotation = []interface{}{pathSpec}
			err = diffField(origNN, modNN, notif)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func diffMaps(origNI, modNI *util.NodeInfo, notif *gnmipb.Notification) error {
	origMapKeys := origNI.FieldValue.MapKeys()
	modMapKeys := modNI.FieldValue.MapKeys()

	modKeys := map[interface{}]struct{}{}
	for _, k := range modMapKeys {
		modKeys[k.Interface()] = struct{}{}
	}

	commonKeys := map[interface{}]struct{}{}
	for _, key := range origMapKeys {
		origNN := *origNI
		origNN.Parent = origNI
		origNN.FieldValue = origNI.FieldValue.MapIndex(key)
		origNN.FieldKey = key
		origNN.FieldKeys = origMapKeys
		modNN := *modNI
		if _, present := modKeys[key.Interface()]; present {
			commonKeys[key.Interface()] = struct{}{}
			modNN.Parent = modNI
			modNN.FieldKeys = modMapKeys
			modNN.FieldKey = key
			modNN.FieldValue = modNI.FieldValue.MapIndex(key)
		}
		pathSpec, err := generatePathSpec(&origNN)
		if err != nil {
			return err
		}
		origNN.Annotation = []interface{}{pathSpec}
		modNN.Annotation = []interface{}{pathSpec}
		err = diffField(&origNN, &modNN, notif)
		if err != nil {
			return err
		}
	}

	for _, key := range modMapKeys {
		if _, present := commonKeys[key.Interface()]; !present {
			modNN := *modNI
			modNN.Parent = modNI
			modNN.FieldKey = key
			modNN.FieldValue = modNI.FieldValue.MapIndex(key)
			modNN.FieldKeys = modMapKeys
			origNN := *origNI
			origNN.FieldValue = reflect.Zero(origNN.StructField.Type.Elem())
			pathSpec, err := generatePathSpec(&modNN)
			if err != nil {
				return err
			}
			origNN.Annotation = []interface{}{pathSpec}
			modNN.Annotation = []interface{}{pathSpec}
			err = diffField(&origNN, &modNN, notif)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func diffDefault(origNI, modNI *util.NodeInfo, notif *gnmipb.Notification) error {
	isOrigUnsetGoEnum := isGoEnumUnset(origNI.FieldValue)
	isModUnsetGoEnum := isGoEnumUnset(modNI.FieldValue)
	if isOrigUnsetGoEnum && isModUnsetGoEnum {
		return nil
	} else if isModUnsetGoEnum {
		pathSpec, err := generatePathSpec(origNI)
		if err != nil {
			return err
		}
		notif.Delete = append(notif.Delete, pathSpec.gNMIPaths...)
		return nil
	}

	origVal := origNI.FieldValue.Interface()
	modVal := modNI.FieldValue.Interface()
	if !reflect.DeepEqual(origVal, modVal) {
		pathSpec, err := generatePathSpec(origNI)
		if err != nil {
			return err
		}
		notif.Update, err = appendUpdate(notif.Update, pathSpec, modVal)
		if err != nil {
			return err
		}
	}
	return nil
}

func generatePathSpec(ni *util.NodeInfo) (*pathSpec, error) {
	sp, err := util.SchemaPaths(ni.StructField)
	if err != nil {
		return nil, err
	}
	if len(sp) == 0 {
		err := fmt.Errorf("invalid schema path for %s", ni.StructField.Name)
		return nil, err
	}
	vp, err := nodeValuePath(ni, sp)
	if err != nil {
		return nil, err
	}
	return vp, nil
}

func isGoEnumUnset(fieldValue reflect.Value) bool {
	// If this is an enumerated value in the output structs, then check whether
	// it is set. Only include values that are set to a non-zero value.
	ival := fieldValue.Interface()
	if _, isEnum := ival.(GoEnum); isEnum {
		// If the value is a simple union enum, then extract
		// the underlying enum value from the interface.
		if fieldValue.Kind() == reflect.Interface {
			fieldValue = fieldValue.Elem()
		}
		if fieldValue.Int() == 0 {
			return true
		}
	}
	return false
}
