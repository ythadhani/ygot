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
	err := diffField(&util.NodeInfo{}, reflect.ValueOf(original), reflect.ValueOf(modified), gnmiNotif)
	if err != nil {
		return nil, err
	}
	return gnmiNotif, nil
}

func diffField(ni *util.NodeInfo, origValue, modValue reflect.Value, notif *gnmipb.Notification) error {
	var (
		err         error
		nonNilValue reflect.Value
	)
	isOrigNilOrInvalid := util.IsNilOrInvalidValue(origValue)
	isModNilOrInvalid := util.IsNilOrInvalidValue(modValue)
	if isOrigNilOrInvalid && isModNilOrInvalid {
		return nil
	} else if isOrigNilOrInvalid {
		nonNilValue = modValue
	} else {
		nonNilValue = origValue
	}

	switch {
	case util.IsValueStructPtr(nonNilValue):
		err = diffStructPtr(ni, origValue, modValue, notif)
	case util.IsValuePtr(nonNilValue):
		err = diffPtr(ni, origValue, modValue, notif)
	case util.IsValueStruct(nonNilValue):
		err = diffStructs(ni, origValue, modValue, notif)
	case util.IsValueMap(nonNilValue):
		err = diffMaps(ni, origValue, modValue, notif)
	default:
		err = diffDefault(ni, origValue, modValue, notif)
	}
	return err
}

func diffStructPtr(ni *util.NodeInfo, origValue, modValue reflect.Value, notif *gnmipb.Notification) error {
	var newOrigValue, newModValue reflect.Value
	if !util.IsNilOrInvalidValue(origValue) {
		newOrigValue = origValue.Elem()
	}
	if !util.IsNilOrInvalidValue(modValue) {
		newModValue = modValue.Elem()
	}
	return diffField(ni, newOrigValue, newModValue, notif)
}

func diffPtr(ni *util.NodeInfo, origValue, modValue reflect.Value, notif *gnmipb.Notification) error {
	if util.IsNilOrInvalidValue(origValue) {
		pathSpec, err := generatePathSpec(ni, modValue)
		if err != nil {
			return err
		}
		notif.Update, err = appendUpdate(notif.Update, pathSpec, modValue.Interface())
		if err != nil {
			return err
		}
	} else if util.IsNilOrInvalidValue(modValue) {
		pathSpec, err := generatePathSpec(ni, origValue)
		if err != nil {
			return err
		}
		notif.Delete = append(notif.Delete, pathSpec.gNMIPaths...)
	} else {
		err := diffField(ni, origValue.Elem(), modValue.Elem(), notif)
		if err != nil {
			return err
		}
	}
	return nil
}

func diffStructs(ni *util.NodeInfo, origValue, modValue reflect.Value, notif *gnmipb.Notification) error {
	var (
		nonNilVal          reflect.Value = origValue
		typeOfNonNonNilVal reflect.Type
	)

	isOrigNilOrInvalid := util.IsNilOrInvalidValue(origValue)
	isModNilOrInvalid := util.IsNilOrInvalidValue(modValue)
	isOrigZero := !isOrigNilOrInvalid && origValue.IsZero()
	isModZero := !isModNilOrInvalid && modValue.IsZero()
	isPresence := util.IsYangPresence(ni.StructField)
	if isOrigZero && isModZero {
		return nil
	}

	if isOrigNilOrInvalid {
		if isPresence {
			pathSpec, err := generatePathSpec(ni, modValue)
			if err != nil {
				return err
			}
			notif.Update, err = appendUpdate(notif.Update, pathSpec, reflect.New(modValue.Type()).Interface())
			if err != nil {
				return err
			}
		}
		if isModZero {
			return nil
		}
		nonNilVal = modValue
	} else if isModNilOrInvalid {
		if isPresence {
			pathSpec, err := generatePathSpec(ni, origValue)
			if err != nil {
				return err
			}
			notif.Delete = append(notif.Delete, pathSpec.gNMIPaths...)
		}
		if isOrigZero {
			return nil
		}
		nonNilVal = origValue
	}
	typeOfNonNonNilVal = nonNilVal.Type()

	for i := 0; i < typeOfNonNonNilVal.NumField(); i++ {
		var newOrigValue, newModValue reflect.Value
		sfOrig := typeOfNonNonNilVal.Field(i)
		nn := &util.NodeInfo{
			Parent:      ni,
			StructField: sfOrig,
		}
		if !isOrigNilOrInvalid {
			newOrigValue = origValue.Field(i)
		}
		if !isModNilOrInvalid {
			newModValue = modValue.Field(i)
		}
		ps, err := util.SchemaPaths(nn.StructField)
		if err != nil {
			return err
		}

		for _, p := range ps {
			nn.PathFromParent = p
			if util.IsTypeSlice(sfOrig.Type) || util.IsTypeMap(sfOrig.Type) {
				// Since lists can have path compression - where the path contains more
				// than one element, ensure that the schema path we received is only two
				// elements long. This protects against compression errors where there are
				// trailing spaces (e.g., a path tag of config/bar/).
				nn.PathFromParent = p[0:1]
			}
			pathSpec, err := generatePathSpec(nn, nonNilVal.Field(i))
			if err != nil {
				return err
			}
			nn.Annotation = []interface{}{pathSpec}
			err = diffField(nn, newOrigValue, newModValue, notif)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func diffMaps(ni *util.NodeInfo, origValue, modValue reflect.Value, notif *gnmipb.Notification) error {
	var origMapKeys, modMapKeys []reflect.Value
	isOrigNilOrInvalid := util.IsNilOrInvalidValue(origValue)
	isModNilOrInvalid := util.IsNilOrInvalidValue(modValue)
	if !isOrigNilOrInvalid {
		origMapKeys = origValue.MapKeys()
	}
	if !isModNilOrInvalid {
		modMapKeys = modValue.MapKeys()
	}

	modKeys := map[interface{}]struct{}{}
	for _, k := range modMapKeys {
		modKeys[k.Interface()] = struct{}{}
	}

	commonKeys := map[interface{}]struct{}{}
	for _, key := range origMapKeys {
		var newOrigValue, newModValue reflect.Value
		nn := *ni
		nn.Parent = ni
		newOrigValue = origValue.MapIndex(key)
		if _, present := modKeys[key.Interface()]; present {
			commonKeys[key.Interface()] = struct{}{}
			newModValue = modValue.MapIndex(key)
		}
		pathSpec, err := generatePathSpec(&nn, newOrigValue)
		if err != nil {
			return err
		}
		nn.Annotation = []interface{}{pathSpec}
		err = diffField(&nn, newOrigValue, newModValue, notif)
		if err != nil {
			return err
		}
	}

	for _, key := range modMapKeys {
		if _, present := commonKeys[key.Interface()]; !present {
			var newOrigValue, newModValue reflect.Value
			nn := *ni
			newModValue = modValue.MapIndex(key)
			nn.Parent = ni
			pathSpec, err := generatePathSpec(&nn, newModValue)
			if err != nil {
				return err
			}
			nn.Annotation = []interface{}{pathSpec}
			err = diffField(&nn, newOrigValue, newModValue, notif)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func diffDefault(ni *util.NodeInfo, origValue, modValue reflect.Value, notif *gnmipb.Notification) error {
	isOrigUnset := util.IsNilOrInvalidValue(origValue) || util.IsNilOrDefaultValue(origValue) || isGoEnumUnset(origValue)
	isModUnset := util.IsNilOrInvalidValue(modValue) || util.IsNilOrDefaultValue(modValue) || isGoEnumUnset(modValue)
	if isOrigUnset && isModUnset {
		return nil
	} else if isModUnset {
		pathSpec, err := generatePathSpec(ni, origValue)
		if err != nil {
			return err
		}
		notif.Delete = append(notif.Delete, pathSpec.gNMIPaths...)
		return nil
	} else if isOrigUnset {
		pathSpec, err := generatePathSpec(ni, modValue)
		if err != nil {
			return err
		}
		notif.Update, err = appendUpdate(notif.Update, pathSpec, modValue.Interface())
		if err != nil {
			return err
		}
		return nil
	}

	origVal := origValue.Interface()
	modVal := modValue.Interface()
	if !reflect.DeepEqual(origVal, modVal) {
		pathSpec, err := generatePathSpec(ni, modValue)
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

func generatePathSpec(ni *util.NodeInfo, val reflect.Value) (*pathSpec, error) {
	sp, err := util.SchemaPaths(ni.StructField)
	if err != nil {
		return nil, err
	}
	if len(sp) == 0 {
		err := fmt.Errorf("invalid schema path for %s", ni.StructField.Name)
		return nil, err
	}
	vp, err := nodeValPath(ni, val, sp)
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

// nodeValuePath takes an input util.NodeInfo struct describing an element within
// a GoStruct tree (be it a leaf, leaf-list, container or list) and returns the
// set of paths that the value represents as a pathSpec pointer.
func nodeValPath(ni *util.NodeInfo, val reflect.Value, schemaPaths [][]string) (*pathSpec, error) {
	if ni.Parent == nil || ni.Parent.Annotation == nil {
		return nodeRootPath(schemaPaths), nil
	}

	cp, err := getPathSpec(ni.Parent)
	if err != nil {
		return nil, err
	}

	if l, ok := val.Interface().(KeyHelperGoStruct); ok {
		return nodeMapPath(l, cp)
	}

	return nodeChildPath(cp, schemaPaths)
}
