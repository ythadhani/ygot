// Copyright 2017 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ytypes

import (
	"errors"
	"fmt"

	"github.com/openconfig/goyang/pkg/yang"
	"github.com/openconfig/ygot/util"
	"github.com/openconfig/ygot/yext"
)

// UnmarshalOpt is an interface used for any option to be supplied
// to the Unmarshal function. Types implementing it can be used to
// control the behaviour of JSON unmarshalling.
type UnmarshalOpt interface {
	IsUnmarshalOpt()
	// yext.ExtensionHandler to process extensions.
	yext.ExtensionHandler
}

type unmarshalConfig struct {
	ignoreExtFields bool
	// extHandler can be used to process extensions defined as struct tags.
	extHandler yext.ExtensionHandler
}

// IgnoreExtraFields is an unmarshal option that controls the
// behaviour of the Unmarshal function when additional fields are
// found in the input JSON. By default, an error will be returned,
// by specifying the IgnoreExtraFields option to Unmarshal, extra
// fields will be discarded.
type IgnoreExtraFields struct{}

// IsUnmarshalOpt marks IgnoreExtraFields as a valid UnmarshalOpt.
func (*IgnoreExtraFields) IsUnmarshalOpt() {}

func (*IgnoreExtraFields) Process(extensions []yext.ExtensionParams, inputData ...interface{}) (interface{}, error) {
	return nil, nil
}

func (*IgnoreExtraFields) IsExtensionHandler() bool {
	return false
}

func (*IgnoreExtraFields) IsExtensionSupported(ext yext.ExtensionParams) bool {
	return false
}

// Unmarshal recursively unmarshals JSON data tree in value into the given
// parent, using the given schema. Any values already in the parent that are
// not present in value are preserved. If provided schema is a leaf or leaf
// list, parent must be referencing the parent GoStruct.
func Unmarshal(schema *yang.Entry, parent interface{}, value interface{}, opts ...UnmarshalOpt) error {
	var (
		ignoreExtFields bool
		extHandler      yext.ExtensionHandler
	)
	for _, o := range opts {
		switch o.(type) {
		case *IgnoreExtraFields:
			ignoreExtFields = true
		default:
			if isExt := o.IsExtensionHandler(); isExt {
				extHandler = o
			}
		}
	}
	return unmarshalGeneric(schema, parent, value, JSONEncoding, unmarshalConfig{
		ignoreExtFields: ignoreExtFields,
		extHandler:      extHandler,
	})
}

// Encoding specifies how the value provided to UnmarshalGeneric function is encoded.
type Encoding int

const (
	// JSONEncoding indicates that provided value is JSON encoded.
	JSONEncoding Encoding = iota

	// GNMIEncoding indicates that provided value is gNMI TypedValue.
	GNMIEncoding

	// gNMIEncodingWithJSONTolerance indicates that provided value is gNMI
	// TypedValue, but it tolerates the case that the values were produced
	// from JSON and that a tolerance may be needed (e.g. positive int is
	// accepted as an uint).
	// This is made unexported because the feature is unstable and could
	// change at any point.
	gNMIEncodingWithJSONTolerance
)

// unmarshalGeneric unmarshals the provided value encoded with the given
// encoding type into the parent with the provided schema. When encoding mode
// is GNMIEncoding, the schema needs to be pointing to a leaf or leaf list
// schema.
func unmarshalGeneric(schema *yang.Entry, parent interface{}, value interface{}, enc Encoding, unmarshalConf unmarshalConfig) error {
	util.Indent()
	defer util.Dedent()

	if schema == nil {
		return fmt.Errorf("nil schema for parent type %T, value %v (%T)", parent, value, value)
	}
	util.DbgPrint("Unmarshal value %v, type %T, into parent type %T, schema name %s", util.ValueStrDebug(value), value, parent, schema.Name)

	if enc == GNMIEncoding && !(schema.IsLeaf() || schema.IsLeafList()) {
		return errors.New("unmarshalling a non leaf node isn't supported in GNMIEncoding mode")
	}

	switch {
	case schema.IsLeaf():
		return unmarshalLeaf(schema, parent, value, enc)
	case schema.IsLeafList():
		return unmarshalLeafList(schema, parent, value, enc)
	case schema.IsList():
		return unmarshalList(schema, parent, value, enc, unmarshalConf)
	case schema.IsChoice():
		return fmt.Errorf("cannot pass choice schema %s to Unmarshal", schema.Name)
	case schema.IsContainer():
		return unmarshalContainer(schema, parent, value, enc, unmarshalConf)
	}
	return fmt.Errorf("unknown schema type for type %T, value %v", value, value)
}
