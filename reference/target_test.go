// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package reference

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/hcl-lang/lang"
	"github.com/hashicorp/hcl-lang/schema"
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty-debug/ctydebug"
	"github.com/zclconf/go-cty/cty"
)

func TestTarget_Address(t *testing.T) {
	testCases := []struct {
		name            string
		pos             hcl.Pos
		activeSelfRefs  bool
		target          Target
		expectedAddress lang.Address
	}{
		{
			"absolute address and no local address",
			hcl.InitialPos,
			false,
			Target{
				Addr: lang.Address{
					lang.RootStep{Name: "aws_instance"},
					lang.AttrStep{Name: "instance_size"},
				},
			},
			lang.Address{
				lang.RootStep{Name: "aws_instance"},
				lang.AttrStep{Name: "instance_size"},
			},
		},
		{
			"local address and no absolute address",
			hcl.InitialPos,
			false,
			Target{
				LocalAddr: lang.Address{
					lang.RootStep{Name: "count"},
					lang.AttrStep{Name: "index"},
				},
			},
			lang.Address{
				lang.RootStep{Name: "count"},
				lang.AttrStep{Name: "index"},
			},
		},
		{
			"self address with active self and matching range",
			hcl.Pos{Line: 2, Column: 2, Byte: 2},
			true,
			Target{
				Addr: lang.Address{
					lang.RootStep{Name: "aws_instance"},
					lang.AttrStep{Name: "instance_size"},
				},
				LocalAddr: lang.Address{
					lang.RootStep{Name: "self"},
					lang.AttrStep{Name: "instance_size"},
				},
				TargetableFromRangePtr: &hcl.Range{
					Filename: "test.tf",
					Start:    hcl.Pos{Line: 1, Column: 1, Byte: 0},
					End:      hcl.Pos{Line: 3, Column: 1, Byte: 10},
				},
			},
			lang.Address{
				lang.RootStep{Name: "self"},
				lang.AttrStep{Name: "instance_size"},
			},
		},
		{
			"self address without active self but matching range",
			hcl.Pos{Line: 2, Column: 2, Byte: 2},
			false,
			Target{
				Addr: lang.Address{
					lang.RootStep{Name: "aws_instance"},
					lang.AttrStep{Name: "instance_size"},
				},
				LocalAddr: lang.Address{
					lang.RootStep{Name: "self"},
					lang.AttrStep{Name: "instance_size"},
				},
				TargetableFromRangePtr: &hcl.Range{
					Filename: "test.tf",
					Start:    hcl.Pos{Line: 1, Column: 1, Byte: 0},
					End:      hcl.Pos{Line: 3, Column: 1, Byte: 10},
				},
			},
			lang.Address{
				lang.RootStep{Name: "aws_instance"},
				lang.AttrStep{Name: "instance_size"},
			},
		},
		{
			"self address with active self but no matching range",
			hcl.Pos{Line: 5, Column: 2, Byte: 15},
			true,
			Target{
				Addr: lang.Address{
					lang.RootStep{Name: "aws_instance"},
					lang.AttrStep{Name: "instance_size"},
				},
				LocalAddr: lang.Address{
					lang.RootStep{Name: "self"},
					lang.AttrStep{Name: "instance_size"},
				},
				TargetableFromRangePtr: &hcl.Range{
					Filename: "test.tf",
					Start:    hcl.Pos{Line: 1, Column: 1, Byte: 0},
					End:      hcl.Pos{Line: 3, Column: 1, Byte: 10},
				},
			},
			lang.Address{
				lang.RootStep{Name: "aws_instance"},
				lang.AttrStep{Name: "instance_size"},
			},
		},
		{
			"self address with active self and missing targetable",
			hcl.Pos{Line: 5, Column: 2, Byte: 15},
			true,
			Target{
				Addr: lang.Address{
					lang.RootStep{Name: "aws_instance"},
					lang.AttrStep{Name: "instance_size"},
				},
				LocalAddr: lang.Address{
					lang.RootStep{Name: "self"},
					lang.AttrStep{Name: "instance_size"},
				},
			},
			lang.Address{
				lang.RootStep{Name: "aws_instance"},
				lang.AttrStep{Name: "instance_size"},
			},
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("%d-%s", i, tc.name), func(t *testing.T) {
			ctx := context.Background()

			if tc.activeSelfRefs {
				ctx = schema.WithActiveSelfRefs(ctx)
			}

			address := tc.target.Address(ctx, tc.pos)
			if diff := cmp.Diff(tc.expectedAddress, address, ctydebug.CmpOptions); diff != "" {
				t.Fatalf("mismatch of address: %s", diff)
			}
		})
	}
}

func TestTarget_Matches_nestedType(t *testing.T) {
	varAddr := func(name string, steps ...lang.AddressStep) lang.Address {
		return append(lang.Address{
			lang.RootStep{Name: "var"},
			lang.AttrStep{Name: name},
		}, steps...)
	}
	idx := func(key cty.Value) lang.IndexStep {
		return lang.IndexStep{Key: key}
	}
	attr := func(name string) lang.AttrStep {
		return lang.AttrStep{Name: name}
	}

	testCases := []struct {
		name          string
		target        Target
		origin        MatchableOrigin
		expectedMatch bool
	}{
		{
			name:   "map index matching element type",
			target: Target{Addr: varAddr("plain"), Type: cty.Map(cty.List(cty.String))},
			origin: LocalOrigin{
				Addr:        varAddr("plain", idx(cty.StringVal("k"))),
				Constraints: OriginConstraints{{OfType: cty.List(cty.String)}},
			},
			expectedMatch: true,
		},
		{
			name:   "map index with dynamic constraint",
			target: Target{Addr: varAddr("plain"), Type: cty.Map(cty.List(cty.String))},
			origin: LocalOrigin{
				Addr:        varAddr("plain", idx(cty.StringVal("k"))),
				Constraints: OriginConstraints{{OfType: cty.DynamicPseudoType}},
			},
			expectedMatch: true,
		},
		{
			name:   "map index with incompatible constraint",
			target: Target{Addr: varAddr("plain"), Type: cty.Map(cty.List(cty.String))},
			origin: LocalOrigin{
				Addr:        varAddr("plain", idx(cty.StringVal("k"))),
				Constraints: OriginConstraints{{OfType: cty.Bool}},
			},
			expectedMatch: false,
		},
		{
			name:   "map index with mismatching scope",
			target: Target{Addr: varAddr("plain"), Type: cty.Map(cty.String), ScopeId: lang.ScopeId("variable")},
			origin: LocalOrigin{
				Addr:        varAddr("plain", idx(cty.StringVal("k"))),
				Constraints: OriginConstraints{{OfScopeId: lang.ScopeId("local"), OfType: cty.String}},
			},
			expectedMatch: false,
		},
		{
			name:   "map attribute access",
			target: Target{Addr: varAddr("plain"), Type: cty.Map(cty.String)},
			origin: LocalOrigin{
				Addr:        varAddr("plain", attr("k")),
				Constraints: OriginConstraints{{OfType: cty.String}},
			},
			expectedMatch: true,
		},
		{
			name:   "map with numeric index",
			target: Target{Addr: varAddr("plain"), Type: cty.Map(cty.String)},
			origin: LocalOrigin{
				Addr:        varAddr("plain", idx(cty.NumberIntVal(0))),
				Constraints: OriginConstraints{{OfType: cty.String}},
			},
			expectedMatch: false,
		},
		{
			name:   "multiple nested steps",
			target: Target{Addr: varAddr("plain"), Type: cty.Map(cty.List(cty.String))},
			origin: LocalOrigin{
				Addr:        varAddr("plain", idx(cty.StringVal("k")), idx(cty.NumberIntVal(0))),
				Constraints: OriginConstraints{{OfType: cty.String}},
			},
			expectedMatch: true,
		},
		{
			name:   "map of any",
			target: Target{Addr: varAddr("plain"), Type: cty.Map(cty.DynamicPseudoType)},
			origin: LocalOrigin{
				Addr:        varAddr("plain", idx(cty.StringVal("k")), attr("foo")),
				Constraints: OriginConstraints{{OfType: cty.String}},
			},
			expectedMatch: true,
		},
		{
			name:   "list index",
			target: Target{Addr: varAddr("list"), Type: cty.List(cty.String)},
			origin: LocalOrigin{
				Addr:        varAddr("list", idx(cty.NumberIntVal(0))),
				Constraints: OriginConstraints{{OfType: cty.String}},
			},
			expectedMatch: true,
		},
		{
			name:   "list with string index",
			target: Target{Addr: varAddr("list"), Type: cty.List(cty.String)},
			origin: LocalOrigin{
				Addr:        varAddr("list", idx(cty.StringVal("k"))),
				Constraints: OriginConstraints{{OfType: cty.String}},
			},
			expectedMatch: false,
		},
		{
			name:   "list attribute access",
			target: Target{Addr: varAddr("list"), Type: cty.List(cty.String)},
			origin: LocalOrigin{
				Addr:        varAddr("list", attr("foo")),
				Constraints: OriginConstraints{{OfType: cty.String}},
			},
			expectedMatch: false,
		},
		{
			name:   "set index",
			target: Target{Addr: varAddr("set"), Type: cty.Set(cty.String)},
			origin: LocalOrigin{
				Addr:        varAddr("set", idx(cty.NumberIntVal(0))),
				Constraints: OriginConstraints{{OfType: cty.String}},
			},
			expectedMatch: false,
		},
		{
			name:   "tuple index",
			target: Target{Addr: varAddr("tuple"), Type: cty.Tuple([]cty.Type{cty.String, cty.Bool})},
			origin: LocalOrigin{
				Addr:        varAddr("tuple", idx(cty.NumberIntVal(1))),
				Constraints: OriginConstraints{{OfType: cty.Bool}},
			},
			expectedMatch: true,
		},
		{
			name:   "tuple index out of range",
			target: Target{Addr: varAddr("tuple"), Type: cty.Tuple([]cty.Type{cty.String, cty.Bool})},
			origin: LocalOrigin{
				Addr:        varAddr("tuple", idx(cty.NumberIntVal(2))),
				Constraints: OriginConstraints{{OfType: cty.DynamicPseudoType}},
			},
			expectedMatch: false,
		},
		{
			name:   "object attribute",
			target: Target{Addr: varAddr("obj"), Type: cty.Object(map[string]cty.Type{"foo": cty.String})},
			origin: LocalOrigin{
				Addr:        varAddr("obj", attr("foo")),
				Constraints: OriginConstraints{{OfType: cty.String}},
			},
			expectedMatch: true,
		},
		{
			name:   "object index by attribute name",
			target: Target{Addr: varAddr("obj"), Type: cty.Object(map[string]cty.Type{"foo": cty.String})},
			origin: LocalOrigin{
				Addr:        varAddr("obj", idx(cty.StringVal("foo"))),
				Constraints: OriginConstraints{{OfType: cty.String}},
			},
			expectedMatch: true,
		},
		{
			name:   "object unknown attribute",
			target: Target{Addr: varAddr("obj"), Type: cty.Object(map[string]cty.Type{"foo": cty.String})},
			origin: LocalOrigin{
				Addr:        varAddr("obj", attr("bar")),
				Constraints: OriginConstraints{{OfType: cty.DynamicPseudoType}},
			},
			expectedMatch: false,
		},
		{
			name:   "primitive type",
			target: Target{Addr: varAddr("str"), Type: cty.String},
			origin: LocalOrigin{
				Addr:        varAddr("str", idx(cty.NumberIntVal(0))),
				Constraints: OriginConstraints{{OfType: cty.DynamicPseudoType}},
			},
			expectedMatch: false,
		},
		{
			name:   "different target address",
			target: Target{Addr: varAddr("other"), Type: cty.Map(cty.String)},
			origin: LocalOrigin{
				Addr:        varAddr("plain", idx(cty.StringVal("k"))),
				Constraints: OriginConstraints{{OfType: cty.String}},
			},
			expectedMatch: false,
		},
		{
			name: "local address",
			target: Target{
				LocalAddr: lang.Address{lang.RootStep{Name: "self"}, attr("tags")},
				Type:      cty.Map(cty.String),
			},
			origin: LocalOrigin{
				Addr: lang.Address{
					lang.RootStep{Name: "self"},
					attr("tags"),
					idx(cty.StringVal("Name")),
				},
				Constraints: OriginConstraints{{OfType: cty.String}},
			},
			expectedMatch: true,
		},
		{
			name: "target with nested targets",
			target: Target{
				Addr: varAddr("obj"),
				Type: cty.Object(map[string]cty.Type{"foo": cty.String}),
				NestedTargets: Targets{
					{Addr: varAddr("obj", attr("foo")), Type: cty.String},
				},
			},
			origin: LocalOrigin{
				Addr:        varAddr("obj", attr("foo")),
				Constraints: OriginConstraints{{OfType: cty.String}},
			},
			expectedMatch: false,
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("%d-%s", i, tc.name), func(t *testing.T) {
			match := tc.target.Matches(tc.origin)
			if match != tc.expectedMatch {
				t.Fatalf("expected match: %t, given: %t", tc.expectedMatch, match)
			}
		})
	}
}

func TestTargets_Match_nestedType_noDuplicates(t *testing.T) {
	objType := cty.Object(map[string]cty.Type{"foo": cty.String})
	nested := Target{
		Addr: lang.Address{
			lang.RootStep{Name: "var"},
			lang.AttrStep{Name: "obj"},
			lang.AttrStep{Name: "foo"},
		},
		Type: cty.String,
	}
	targets := Targets{
		{
			Addr: lang.Address{
				lang.RootStep{Name: "var"},
				lang.AttrStep{Name: "obj"},
			},
			Type:          objType,
			NestedTargets: Targets{nested},
		},
	}
	origin := LocalOrigin{
		Addr:        nested.Addr,
		Constraints: OriginConstraints{{OfType: cty.String}},
	}

	matched, ok := targets.Match(origin)
	if !ok {
		t.Fatal("expected a match")
	}
	if diff := cmp.Diff(Targets{nested}, matched, ctydebug.CmpOptions); diff != "" {
		t.Fatalf("unexpected targets: %s", diff)
	}
}
