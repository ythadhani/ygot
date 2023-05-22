package ygot

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/kylelemons/godebug/pretty"
	"github.com/openconfig/gnmi/errdiff"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ygot/testutil"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestOptimizedDiff(t *testing.T) {
	tests := []struct {
		desc          string
		inOrig, inMod GoStruct
		inOpts        []DiffOpt
		want          *gnmipb.Notification
		wantErrSubStr string
		skipTest      bool
	}{{
		desc:   "single path addition in modified",
		inOrig: &renderExample{},
		inMod: &renderExample{
			Str: String("cabernet-sauvignon"),
		},
		want: &gnmipb.Notification{
			Update: []*gnmipb.Update{{
				Path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{{
						Name: "str",
					}},
				},
				Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_StringVal{"cabernet-sauvignon"}},
			}},
		},
	}, {
		skipTest: true, // IgnoreAdditions not supported
		desc:     "one path each modified, deleted, and added with IgnoreNewPaths set",
		inOrig: &renderExample{
			IntVal:   Int32(5),
			FloatVal: Float32(1.5),
			Int64Val: Int64(100),
		},
		inMod: &renderExample{
			IntVal:   Int32(10),
			Str:      String("cabernet-sauvignon"),
			Int64Val: Int64(100),
		},
		inOpts: []DiffOpt{&IgnoreAdditions{}},
		want: &gnmipb.Notification{
			Delete: []*gnmipb.Path{{
				Elem: []*gnmipb.PathElem{{
					Name: "floatval",
				}},
			}},
			Update: []*gnmipb.Update{{
				Path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{{
						Name: "int-val",
					}},
				},
				Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_IntVal{10}},
			}},
		},
	}, {
		desc:   "extra empty child struct in modified -- no difference",
		inOrig: &renderExample{},
		inMod: &renderExample{
			Ch: &renderExampleChild{},
		},
		want: &gnmipb.Notification{},
	}, {
		desc:   "ignore YANGEmpty(false) -- no difference",
		inOrig: &renderExample{},
		inMod: &renderExample{
			Empty: YANGEmpty(false),
		},
		want: &gnmipb.Notification{},
	}, {
		desc: "single path deletion in modified",
		inOrig: &renderExample{
			Str: String("chardonnay"),
		},
		inMod: &renderExample{},
		want: &gnmipb.Notification{
			Delete: []*gnmipb.Path{{
				Elem: []*gnmipb.PathElem{{
					Name: "str",
				}},
			}},
		},
	}, {
		desc: "single path modification",
		inOrig: &renderExample{
			Str: String("grenache"),
		},
		inMod: &renderExample{
			Str: String("malbec"),
		},
		want: &gnmipb.Notification{
			Update: []*gnmipb.Update{{
				Path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{{
						Name: "str",
					}},
				},
				Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_StringVal{"malbec"}},
			}},
		},
	}, {
		desc:   "no change",
		inOrig: &renderExample{},
		inMod:  &renderExample{},
		want:   &gnmipb.Notification{},
	}, {
		desc: "leaf only change with enum in same container",
		inOrig: &renderExample{
			Ch: &renderExampleChild{
				Val: Uint64(42),
			},
		},
		inMod: &renderExample{
			Ch: &renderExampleChild{
				Val: Uint64(84),
			},
		},
		want: &gnmipb.Notification{
			Update: []*gnmipb.Update{{
				Path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{{
						Name: "ch",
					}, {
						Name: "val",
					}},
				},
				Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_UintVal{84}},
			}},
		},
	}, {
		desc:   "multiple path addition, with complex types",
		inOrig: &renderExample{},
		inMod: &renderExample{
			IntVal:    Int32(42),
			FloatVal:  Float32(42.42),
			BoolVal:   Bool(true),
			EnumField: EnumTestVALONE,
			Ch: &renderExampleChild{
				Val: Uint64(42),
			},
			LeafList:       []string{"merlot", "pinot-noir"},
			UnionVal:       &renderExampleUnionString{"semillon"},
			UnionValSimple: testutil.UnionString("vermouth"),
			UnionLeafListSimple: []exampleUnion{
				testutil.UnionString("hello"),
				testutil.UnionInt64(42),
				testutil.UnionFloat64(3.14),
				EnumTestVALONE,
				testBinary,
				testutil.UnionBool(true),
				testutil.YANGEmpty(false),
			},
			Binary: Binary{42, 42, 42},
			Empty:  true,
		},
		want: &gnmipb.Notification{
			Update: []*gnmipb.Update{{
				Path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{{
						Name: "int-val",
					}},
				},
				Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_IntVal{42}},
			}, {
				Path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{{
						Name: "floatval",
					}},
				},
				Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_FloatVal{42.42}},
			}, {
				Path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{{
						Name: "boolval",
					}},
				},
				Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_BoolVal{true}},
			}, {
				Path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{{
						Name: "enum",
					}},
				},
				Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_StringVal{"VAL_ONE"}},
			}, {
				Path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{{
						Name: "ch",
					}, {
						Name: "val",
					}},
				},
				Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_UintVal{42}},
			}, {
				Path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{{
						Name: "leaf-list",
					}},
				},
				Val: &gnmipb.TypedValue{
					Value: &gnmipb.TypedValue_LeaflistVal{
						&gnmipb.ScalarArray{
							Element: []*gnmipb.TypedValue{
								{Value: &gnmipb.TypedValue_StringVal{"merlot"}},
								{Value: &gnmipb.TypedValue_StringVal{"pinot-noir"}},
							},
						},
					},
				},
			}, {
				Path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{{
						Name: "union-val",
					}},
				},
				Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_StringVal{"semillon"}},
			}, {
				Path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{{
						Name: "union-val-simple",
					}},
				},
				Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_StringVal{"vermouth"}},
			}, {
				Path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{{
						Name: "union-list-simple",
					}},
				},
				Val: &gnmipb.TypedValue{
					Value: &gnmipb.TypedValue_LeaflistVal{
						&gnmipb.ScalarArray{
							Element: []*gnmipb.TypedValue{
								{Value: &gnmipb.TypedValue_StringVal{"hello"}},
								{Value: &gnmipb.TypedValue_IntVal{42}},
								{Value: &gnmipb.TypedValue_FloatVal{3.14}},
								{Value: &gnmipb.TypedValue_StringVal{"VAL_ONE"}},
								{Value: &gnmipb.TypedValue_BytesVal{[]byte(base64testString)}},
								{Value: &gnmipb.TypedValue_BoolVal{true}},
								{Value: &gnmipb.TypedValue_BoolVal{false}},
							},
						},
					},
				},
			}, {
				Path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{{
						Name: "binary",
					}},
				},
				Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_BytesVal{[]byte{42, 42, 42}}},
			}, {
				Path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{{
						Name: "empty",
					}},
				},
				Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_BoolVal{true}},
			}},
		},
	}, {
		desc:   "union addition: enum",
		inOrig: &renderExample{},
		inMod: &renderExample{
			UnionValSimple: EnumTestVALONE,
		},
		want: &gnmipb.Notification{
			Update: []*gnmipb.Update{{
				Path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{{
						Name: "union-val-simple",
					}},
				},
				Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_StringVal{"VAL_ONE"}},
			}},
		},
	}, {
		desc:   "union addition: int64",
		inOrig: &renderExample{},
		inMod: &renderExample{
			UnionValSimple: testutil.UnionInt64(1),
		},
		want: &gnmipb.Notification{
			Update: []*gnmipb.Update{{
				Path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{{
						Name: "union-val-simple",
					}},
				},
				Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_IntVal{1}},
			}},
		},
	}, {
		desc:   "union addition: float64",
		inOrig: &renderExample{},
		inMod: &renderExample{
			UnionValSimple: testutil.UnionFloat64(3.14),
		},
		want: &gnmipb.Notification{
			Update: []*gnmipb.Update{{
				Path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{{
						Name: "union-val-simple",
					}},
				},
				Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_FloatVal{3.14}},
			}},
		},
	}, {
		desc:   "union addition: bool",
		inOrig: &renderExample{},
		inMod: &renderExample{
			UnionValSimple: testutil.UnionBool(true),
		},
		want: &gnmipb.Notification{
			Update: []*gnmipb.Update{{
				Path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{{
						Name: "union-val-simple",
					}},
				},
				Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_BoolVal{true}},
			}},
		},
	}, {
		desc:   "union addition: empty",
		inOrig: &renderExample{},
		inMod: &renderExample{
			UnionValSimple: testutil.YANGEmpty(true),
		},
		want: &gnmipb.Notification{
			Update: []*gnmipb.Update{{
				Path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{{
						Name: "union-val-simple",
					}},
				},
				Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_BoolVal{true}},
			}},
		},
	}, {
		desc:   "union addition: binary",
		inOrig: &renderExample{},
		inMod: &renderExample{
			UnionValSimple: testBinary,
		},
		want: &gnmipb.Notification{
			Update: []*gnmipb.Update{{
				Path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{{
						Name: "union-val-simple",
					}},
				},
				Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_BytesVal{[]byte(base64testString)}},
			}},
		},
	}, {
		desc: "multiple element set in both - no diff",
		inOrig: &renderExample{
			IntVal:    Int32(42),
			FloatVal:  Float32(42.42),
			BoolVal:   Bool(true),
			EnumField: EnumTestVALONE,
			Ch: &renderExampleChild{
				Val: Uint64(42),
			},
			LeafList:       []string{"merlot", "pinot-noir"},
			UnionVal:       &renderExampleUnionString{"semillon"},
			UnionValSimple: testutil.UnionString("vermouth"),
			UnionLeafListSimple: []exampleUnion{
				testutil.UnionString("hello"),
				testutil.UnionInt64(42),
				testutil.UnionFloat64(3.14),
				EnumTestVALONE,
				testBinary,
				testutil.UnionBool(true),
				testutil.YANGEmpty(false),
			},
			Binary: Binary{42, 42, 42},
			Empty:  true,
		},
		inMod: &renderExample{
			IntVal:    Int32(42),
			FloatVal:  Float32(42.42),
			BoolVal:   Bool(true),
			EnumField: EnumTestVALONE,
			Ch: &renderExampleChild{
				Val: Uint64(42),
			},
			LeafList:       []string{"merlot", "pinot-noir"},
			UnionVal:       &renderExampleUnionString{"semillon"},
			UnionValSimple: testutil.UnionString("vermouth"),
			UnionLeafListSimple: []exampleUnion{
				testutil.UnionString("hello"),
				testutil.UnionInt64(42),
				testutil.UnionFloat64(3.14),
				EnumTestVALONE,
				testBinary,
				testutil.UnionBool(true),
				testutil.YANGEmpty(false),
			},
			Binary: Binary{42, 42, 42},
			Empty:  true,
		},
		want: &gnmipb.Notification{},
	}, {
		desc: "multiple path modify",
		inOrig: &renderExample{
			IntVal:    Int32(43),
			FloatVal:  Float32(43.43),
			BoolVal:   Bool(true),
			EnumField: EnumTestVALTWO,
			Ch: &renderExampleChild{
				Val: Uint64(43),
			},
			LeafList:       []string{"syrah", "tempranillo"},
			UnionVal:       &renderExampleUnionString{"viognier"},
			UnionValSimple: testutil.UnionString("vermouth"),
			UnionLeafListSimple: []exampleUnion{
				testutil.UnionString("hello"),
				testutil.UnionInt64(42),
				testutil.UnionFloat64(3.14),
				EnumTestVALONE,
				testBinary,
				testutil.UnionBool(true),
				testutil.YANGEmpty(false),
			},
			Binary: Binary{43, 43, 43},
			Empty:  false,
		},
		inMod: &renderExample{
			IntVal:    Int32(42),
			FloatVal:  Float32(42.42),
			BoolVal:   Bool(false),
			EnumField: EnumTestVALONE,
			Ch: &renderExampleChild{
				Val: Uint64(42),
			},
			LeafList:       []string{"alcase", "anjou"},
			UnionVal:       &renderExampleUnionString{"arbois"},
			UnionValSimple: testutil.UnionFloat64(2.71828),
			UnionLeafListSimple: []exampleUnion{
				testutil.UnionString("world"),
				testutil.UnionInt64(84),
				testutil.UnionFloat64(6.28),
				EnumTestVALTWO,
				testBinary1,
				testutil.UnionBool(false),
				testutil.YANGEmpty(true),
			},
			Binary: Binary{42, 42, 42},
			Empty:  true,
		},
		want: &gnmipb.Notification{
			Update: []*gnmipb.Update{{
				Path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{{
						Name: "int-val",
					}},
				},
				Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_IntVal{42}},
			}, {
				Path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{{
						Name: "floatval",
					}},
				},
				Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_FloatVal{42.42}},
			}, {
				Path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{{
						Name: "boolval",
					}},
				},
				Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_BoolVal{false}},
			}, {
				Path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{{
						Name: "enum",
					}},
				},
				Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_StringVal{"VAL_ONE"}},
			}, {
				Path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{{
						Name: "ch",
					}, {
						Name: "val",
					}},
				},
				Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_UintVal{42}},
			}, {
				Path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{{
						Name: "leaf-list",
					}},
				},
				Val: &gnmipb.TypedValue{
					Value: &gnmipb.TypedValue_LeaflistVal{
						&gnmipb.ScalarArray{
							Element: []*gnmipb.TypedValue{
								{Value: &gnmipb.TypedValue_StringVal{"alcase"}},
								{Value: &gnmipb.TypedValue_StringVal{"anjou"}},
							},
						},
					},
				},
			}, {
				Path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{{
						Name: "union-val",
					}},
				},
				Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_StringVal{"arbois"}},
			}, {
				Path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{{
						Name: "union-val-simple",
					}},
				},
				Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_FloatVal{2.71828}},
			}, {
				Path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{{
						Name: "union-list-simple",
					}},
				},
				Val: &gnmipb.TypedValue{
					Value: &gnmipb.TypedValue_LeaflistVal{
						&gnmipb.ScalarArray{
							Element: []*gnmipb.TypedValue{
								{Value: &gnmipb.TypedValue_StringVal{"world"}},
								{Value: &gnmipb.TypedValue_IntVal{84}},
								{Value: &gnmipb.TypedValue_FloatVal{6.28}},
								{Value: &gnmipb.TypedValue_StringVal{"VAL_TWO"}},
								{Value: &gnmipb.TypedValue_BytesVal{[]byte("abc")}},
								{Value: &gnmipb.TypedValue_BoolVal{false}},
								{Value: &gnmipb.TypedValue_BoolVal{true}},
							},
						},
					},
				},
			}, {
				Path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{{
						Name: "binary",
					}},
				},
				Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_BytesVal{[]byte{42, 42, 42}}},
			}, {
				Path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{{
						Name: "empty",
					}},
				},
				Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_BoolVal{true}},
			}},
		},
	}, {
		desc: "multiple path delete",
		inOrig: &renderExample{
			IntVal:    Int32(43),
			FloatVal:  Float32(43.43),
			BoolVal:   Bool(true),
			EnumField: EnumTestVALTWO,
			Ch: &renderExampleChild{
				Val: Uint64(43),
			},
			LeafList:       []string{"syrah", "tempranillo"},
			UnionVal:       &renderExampleUnionString{"viognier"},
			UnionValSimple: testutil.UnionString("vermouth"),
			UnionLeafListSimple: []exampleUnion{
				testutil.UnionString("hello"),
				testutil.UnionInt64(42),
				testutil.UnionFloat64(3.14),
				EnumTestVALONE,
				testBinary,
				testutil.UnionBool(true),
				testutil.YANGEmpty(false),
			},
			Binary: Binary{43, 43, 43},
			Empty:  false,
		},
		inMod: &renderExample{},
		want: &gnmipb.Notification{
			Delete: []*gnmipb.Path{{
				Elem: []*gnmipb.PathElem{{
					Name: "int-val",
				}},
			}, {
				Elem: []*gnmipb.PathElem{{
					Name: "floatval",
				}},
			}, {
				Elem: []*gnmipb.PathElem{{
					Name: "boolval",
				}},
			}, {
				Elem: []*gnmipb.PathElem{{
					Name: "enum",
				}},
			}, {
				Elem: []*gnmipb.PathElem{{
					Name: "ch",
				}, {
					Name: "val",
				}},
			}, {
				Elem: []*gnmipb.PathElem{{
					Name: "leaf-list",
				}},
			}, {
				Elem: []*gnmipb.PathElem{{
					Name: "union-val",
				}},
			}, {
				Elem: []*gnmipb.PathElem{{
					Name: "union-val-simple",
				}},
			}, {
				Elem: []*gnmipb.PathElem{{
					Name: "union-list-simple",
				}},
			}, {
				Elem: []*gnmipb.PathElem{{
					Name: "binary",
				}},
			}},
		},
	}, {
		skipTest: true, // "multi-path" not supported
		desc:     "add an item to a list",
		inOrig: &pathElemExample{
			List: map[string]*pathElemExampleChild{
				"p1": {Val: String("p1")},
			},
		},
		inMod: &pathElemExample{
			List: map[string]*pathElemExampleChild{
				"p1": {Val: String("p1")},
				"p2": {Val: String("p2")},
			},
		},
		want: &gnmipb.Notification{
			Update: []*gnmipb.Update{{
				Path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{{
						Name: "list",
						Key:  map[string]string{"val": "p2"},
					}, {
						Name: "val",
					}},
				},
				Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_StringVal{"p2"}},
			}, {
				Path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{{
						Name: "list",
						Key:  map[string]string{"val": "p2"},
					}, {
						Name: "config",
					}, {
						Name: "val",
					}},
				},
				Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_StringVal{"p2"}},
			}},
		},
	}, {
		skipTest: true, // multi-path not supported
		desc:     "remove item from list",
		inOrig: &pathElemExample{
			List: map[string]*pathElemExampleChild{
				"p1": {Val: String("p1")},
				"p2": {Val: String("p2")},
			},
		},
		inMod: &pathElemExample{
			List: map[string]*pathElemExampleChild{
				"p1": {Val: String("p1")},
			},
		},
		want: &gnmipb.Notification{
			Delete: []*gnmipb.Path{{
				Elem: []*gnmipb.PathElem{{
					Name: "list",
					Key:  map[string]string{"val": "p2"},
				}, {
					Name: "val",
				}},
			}, {
				Elem: []*gnmipb.PathElem{{
					Name: "list",
					Key:  map[string]string{"val": "p2"},
				}, {
					Name: "config",
				}, {
					Name: "val",
				}},
			}},
		},
	}, {
		skipTest:      true, // We wouldn't run into this but would be good to fix
		desc:          "invalid original",
		inOrig:        &invalidGoStructEntity{},
		inMod:         &invalidGoStructEntity{},
		wantErrSubStr: "could not extract set leaves from original struct",
	}, {
		desc:   "invalid enum in modified",
		inOrig: &badGoStruct{},
		inMod: &badGoStruct{
			InvalidEnum: 42,
		},
		wantErrSubStr: "cannot represent field value 42 as TypedValue for path /an-enum",
	}, {
		desc: "invalid enum in original",
		inOrig: &badGoStruct{
			InvalidEnum: 44,
		},
		inMod: &badGoStruct{
			InvalidEnum: 42,
		},
		wantErrSubStr: "cannot represent field value 42 as TypedValue for path /an-enum",
	}, {
		desc:          "different types",
		inOrig:        &renderExample{},
		inMod:         &pathElemExample{},
		wantErrSubStr: "cannot diff structs of different types",
	}, {
		skipTest: true, // multiple paths not supported
		desc:     "multiple paths - addition - without single path",
		inOrig:   &multiPathStruct{},
		inMod:    &multiPathStruct{TwoPaths: String("foo")},
		want: &gnmipb.Notification{
			Update: []*gnmipb.Update{{
				Path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{{
						Name: "two-path",
					}},
				},
				Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_StringVal{"foo"}},
			}, {
				Path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{{
						Name: "config",
					}, {
						Name: "two-path",
					}},
				},
				Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_StringVal{"foo"}},
			}},
		},
	}, {
		skipTest: true, // multiple paths not supported
		desc:     "multiple paths - addition - with single path option",
		inOrig:   &multiPathStruct{},
		inMod:    &multiPathStruct{TwoPaths: String("foo")},
		inOpts: []DiffOpt{
			&DiffPathOpt{MapToSinglePath: true},
		},
		want: &gnmipb.Notification{
			Update: []*gnmipb.Update{{
				Path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{{
						Name: "two-path",
					}},
				},
				Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_StringVal{"foo"}},
			}},
		},
	}, {
		skipTest: true, // multiple paths not supported
		desc:     "multiple paths - deletion - without single path option",
		inOrig:   &multiPathStruct{TwoPaths: String("foo")},
		inMod:    &multiPathStruct{},
		want: &gnmipb.Notification{
			Delete: []*gnmipb.Path{{
				Elem: []*gnmipb.PathElem{{
					Name: "config",
				}, {
					Name: "two-path",
				}},
			}, {
				Elem: []*gnmipb.PathElem{{
					Name: "two-path",
				}},
			}},
		},
	}, {
		skipTest: true, // multiple paths not supported
		desc:     "multiple paths - deletion - with single path option",
		inOrig:   &multiPathStruct{TwoPaths: String("foo")},
		inMod:    &multiPathStruct{},
		inOpts: []DiffOpt{
			&DiffPathOpt{MapToSinglePath: true},
		},
		want: &gnmipb.Notification{
			Delete: []*gnmipb.Path{{
				Elem: []*gnmipb.PathElem{{
					Name: "two-path",
				}},
			}},
		},
	}, {
		desc:   "leaf-list of enumerations change",
		inOrig: &renderExample{},
		inMod:  &renderExample{EnumLeafList: []EnumTest{EnumTestVALONE}},
		want: &gnmipb.Notification{
			Update: []*gnmipb.Update{{
				Path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{{
						Name: "enum-leaflist",
					}},
				},
				Val: &gnmipb.TypedValue{
					Value: &gnmipb.TypedValue_LeaflistVal{
						&gnmipb.ScalarArray{
							Element: []*gnmipb.TypedValue{{
								Value: &gnmipb.TypedValue_StringVal{"VAL_ONE"},
							}},
						},
					},
				},
			}},
		},
	}, {
		desc:   "Empty presence container as a leaf",
		inOrig: &basicStruct{},
		inMod:  &basicStruct{PresenceContStruct: &basicStructFour{}},
		want: &gnmipb.Notification{
			Update: []*gnmipb.Update{{
				Path: &gnmipb.Path{
					Elem: []*gnmipb.PathElem{{
						Name: "presence-cont-value",
					}},
				},
				Val: &gnmipb.TypedValue{Value: &gnmipb.TypedValue_JsonIetfVal{JsonIetfVal: []byte(`{}`)}},
			}},
		},
	}}

	for _, tt := range tests {
		if tt.skipTest {
			continue
		}
		got, err := OptimizedDiff(tt.inOrig, tt.inMod)
		if diff := errdiff.Substring(err, tt.wantErrSubStr); diff != "" {
			t.Errorf("%s: Diff(%s, %s): did not get expected error status, got: %s, want: %s", tt.desc, pretty.Sprint(tt.inOrig), pretty.Sprint(tt.inMod), err, tt.wantErrSubStr)
			continue
		}

		if tt.wantErrSubStr != "" {
			continue
		}
		// To re-use the NotificationSetEqual helper, we put the want and got into
		// a slice.
		if !testutil.NotificationSetEqual([]*gnmipb.Notification{tt.want}, []*gnmipb.Notification{got}) {
			diff := cmp.Diff(got, tt.want, protocmp.Transform())
			t.Errorf("%s: Diff(%s, %s): did not get expected Notification, diff(-got,+want):\n%s", tt.desc, pretty.Sprint(tt.inOrig), pretty.Sprint(tt.inMod), diff)
		}
	}
}
