// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package reference

import (
	"context"
	"math/big"

	"github.com/hashicorp/hcl-lang/lang"
	"github.com/hashicorp/hcl-lang/schema"
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/convert"
)

type Target struct {
	// Addr represents the address of the target, as available
	// elsewhere in the configuration
	Addr lang.Address

	// LocalAddr represents the address of the target
	// as available *locally* (e.g. self.attr_name)
	LocalAddr lang.Address

	// TargetableFromRangePtr defines where the target is targetable from.
	// This is considered when matching the target against origin.
	//
	// e.g. count.index is only available within the body of the block
	// where count is declared (and extension enabled)
	TargetableFromRangePtr *hcl.Range

	// ScopeId provides scope for matching/filtering
	// (in addition to Type & Addr/LocalAddr).
	//
	// There should never be two targets with the same Type & address,
	// but there are contexts (e.g. completion) where we don't filter
	// by address and may not have type either (e.g. because targets
	// are type-unaware).
	ScopeId lang.ScopeId

	// RangePtr represents range of the whole attribute or block
	// or nil if the target is not addressable.
	RangePtr *hcl.Range

	// DefRangePtr represents a definition range, i.e. block header,
	// or an attribute name or nil if the target is not addressable
	// or when it represents multiple list, set or map blocks.
	//
	// This is useful in situation where a representative single-line
	// range is needed - e.g. to render a contextual UI element in
	// the editor near the middle of this range.
	DefRangePtr *hcl.Range

	Type        cty.Type
	Name        string
	Description lang.MarkupContent

	NestedTargets Targets
}

// rangeOverlaps is a copy of hcl.Range.Overlaps
// https://github.com/hashicorp/hcl/blob/v2.14.1/pos.go#L195-L212
// which accounts for empty ranges that are common in the context of LS
func rangeOverlaps(one, other hcl.Range) bool {
	switch {
	case one.Filename != other.Filename:
		// If the ranges are in different files then they can't possibly overlap
		return false
	case one.Empty() && other.Empty():
		// Empty ranges can never overlap
		return false
	case one.ContainsOffset(other.Start.Byte) || one.ContainsOffset(other.End.Byte):
		return true
	case other.ContainsOffset(one.Start.Byte) || other.ContainsOffset(one.End.Byte):
		return true
	default:
		return false
	}
}

func (ref Target) Copy() Target {
	return Target{
		Addr:                   ref.Addr,
		LocalAddr:              ref.LocalAddr,
		TargetableFromRangePtr: copyHclRangePtr(ref.TargetableFromRangePtr),
		ScopeId:                ref.ScopeId,
		RangePtr:               copyHclRangePtr(ref.RangePtr),
		DefRangePtr:            copyHclRangePtr(ref.DefRangePtr),
		Type:                   ref.Type, // cty.Type is immutable by design
		Name:                   ref.Name,
		Description:            ref.Description,
		NestedTargets:          ref.NestedTargets.Copy(),
	}
}

func copyHclRangePtr(rng *hcl.Range) *hcl.Range {
	if rng == nil {
		return nil
	}
	return rng.Ptr()
}

// Address returns any of the two non-empty addresses
// depending on the provided context
func (r Target) Address(ctx context.Context, pos hcl.Pos) lang.Address {
	if len(r.LocalAddr) > 0 {
		// If the target has only local address, use it
		if len(r.Addr) == 0 {
			return r.LocalAddr
		}

		// If the target has local self address & self is active
		if r.LocalAddr[0].String() == "self" && schema.ActiveSelfRefsFromContext(ctx) {
			// and we targeting it from the expected range
			if r.TargetableFromRangePtr != nil && r.TargetableFromRangePtr.ContainsPos(pos) {
				return r.LocalAddr
			}
		}
	}

	return r.Addr
}

func (r Target) FriendlyName() string {
	if r.Name != "" {
		return r.Name
	}

	if r.Type != cty.NilType {
		return r.Type.FriendlyName()
	}

	return "reference"
}

func (r Target) TargetRange() (hcl.Range, bool) {
	if r.RangePtr == nil {
		return hcl.Range{}, false
	}

	return *r.RangePtr, true
}

func (target Target) MatchesConstraint(ref schema.Reference) bool {
	return target.MatchesScopeId(ref.OfScopeId) && target.IsConvertibleToType(ref.OfType)
}

func (ref Target) MatchesScopeId(scopeId lang.ScopeId) bool {
	return scopeId == "" || ref.ScopeId == scopeId
}

func (ref Target) IsConvertibleToType(typ cty.Type) bool {
	isConvertible := false
	if typ != cty.NilType && ref.Type != cty.NilType {
		if ref.Type == cty.DynamicPseudoType {
			// anything is convertible to dynamic
			isConvertible = true
		}
		if _, err := convert.Convert(cty.UnknownVal(ref.Type), typ); err == nil {
			isConvertible = true
		}
	}

	return isConvertible || (typ == cty.NilType && ref.Type == cty.NilType)
}

func (target Target) Matches(origin MatchableOrigin) bool {
	originAddr, localOriginAddr := origin.Address(), origin.Address()

	matchesCons := false

	// Unconstrained origins should be uncommon, but they match any target
	if len(origin.OriginConstraints()) == 0 {
		// As long as the target is type-aware. Type-unaware targets
		// generally don't have Type, so we avoid false positive here.
		if target.Type != cty.NilType {
			matchesCons = true
		}
	}

	for _, cons := range origin.OriginConstraints() {
		if !target.MatchesScopeId(cons.OfScopeId) {
			continue
		}

		if target.Type == cty.DynamicPseudoType {
			// Account for the case where the origin address points to a nested
			// segment, which the target address doesn't explicitly contain
			// but implies.
			// e.g. If self.foo target is of "any type" (cty.DynamicPseudoType),
			// then we assume it is a match for self.foo.anything
			// by ignoring the last "anything" segment.
			if len(target.Addr) < len(origin.Address()) {
				originAddr = origin.Address().FirstSteps(uint(len(target.Addr)))
			}
			if len(target.LocalAddr) < len(origin.Address()) {
				localOriginAddr = origin.Address().FirstSteps(uint(len(target.LocalAddr)))
			}
			matchesCons = true
			continue
		}
		if len(target.NestedTargets) == 0 {
			// Account for the case where the origin address points into
			// a collection or structural type via index or attribute steps
			// which the target doesn't declare as nested targets.
			// e.g. If var.foo target is of type map(list(string)),
			// then var.foo["key"] is a match if list(string) satisfies
			// the constraint.
			//
			// Targets with nested targets are skipped, so that the origin
			// only matches the most specific (nested) target.
			if prefixLen, ok := target.matchesNestedType(target.Addr, origin.Address(), cons); ok {
				originAddr = origin.Address().FirstSteps(prefixLen)
				matchesCons = true
				continue
			}
			if prefixLen, ok := target.matchesNestedType(target.LocalAddr, origin.Address(), cons); ok {
				localOriginAddr = origin.Address().FirstSteps(prefixLen)
				matchesCons = true
				continue
			}
		}
		if typeMatchesConstraint(target.Type, cons) {
			matchesCons = true
		}
		if cons.OfType == cty.NilType && target.Type == cty.NilType {
			// This just simplifies testing
			matchesCons = true
		}
	}

	// If the target is only targetable from a particular range
	// we confirm that the origin is within that range.
	targetRangeMatches := true
	if target.TargetableFromRangePtr != nil && !rangeOverlaps(*target.TargetableFromRangePtr, origin.OriginRange()) {
		targetRangeMatches = false
	}

	return ((target.LocalAddr.Equals(localOriginAddr) && targetRangeMatches) || target.Addr.Equals(originAddr)) && matchesCons
}

// matchesNestedType checks whether originAddr points to a value nested
// inside the target at targetAddr, and if so, whether the type of that
// nested value satisfies the given constraint.
// It returns the length of targetAddr when it does.
func (target Target) matchesNestedType(targetAddr, originAddr lang.Address, cons OriginConstraint) (uint, bool) {
	if len(targetAddr) == 0 || len(originAddr) <= len(targetAddr) {
		return 0, false
	}
	prefixLen := uint(len(targetAddr))
	if !targetAddr.Equals(originAddr.FirstSteps(prefixLen)) {
		return 0, false
	}

	typ, ok := typeOfNestedSteps(target.Type, originAddr[prefixLen:])
	if !ok {
		return 0, false
	}

	return prefixLen, typeMatchesConstraint(typ, cons)
}

func typeMatchesConstraint(typ cty.Type, cons OriginConstraint) bool {
	if cons.OfType.IsTupleType() && cons.OfType.Length() == 0 && typ.IsTupleType() {
		// This is a special case where we match an empty tuple (cty.EmptyTuple)
		// against any tuple.
		return true
	}
	if cons.OfType != cty.NilType && (Target{Type: typ}).IsConvertibleToType(cons.OfType) {
		return true
	}
	return false
}

// typeOfNestedSteps returns the type of the value found by following
// the given index or attribute steps into a value of the given type.
// It returns false if the steps cannot be applied to the type.
func typeOfNestedSteps(typ cty.Type, steps lang.Address) (cty.Type, bool) {
	for _, step := range steps {
		if typ == cty.DynamicPseudoType {
			// Any nested value of a dynamic value is dynamic too
			return cty.DynamicPseudoType, true
		}

		switch s := step.(type) {
		case lang.AttrStep:
			switch {
			case typ.IsObjectType():
				if !typ.HasAttribute(s.Name) {
					return cty.NilType, false
				}
				typ = typ.AttributeType(s.Name)
			case typ.IsMapType():
				typ = typ.ElementType()
			default:
				return cty.NilType, false
			}
		case lang.IndexStep:
			nestedType, ok := typeOfIndex(typ, s.Key)
			if !ok {
				return cty.NilType, false
			}
			typ = nestedType
		default:
			return cty.NilType, false
		}
	}

	return typ, true
}

func typeOfIndex(typ cty.Type, key cty.Value) (cty.Type, bool) {
	if key.IsNull() || !key.IsKnown() {
		return cty.NilType, false
	}

	switch {
	case typ.IsListType():
		if key.Type() != cty.Number {
			return cty.NilType, false
		}
		return typ.ElementType(), true
	case typ.IsMapType():
		if key.Type() != cty.String {
			return cty.NilType, false
		}
		return typ.ElementType(), true
	case typ.IsTupleType():
		if key.Type() != cty.Number {
			return cty.NilType, false
		}
		idx, accuracy := key.AsBigFloat().Int64()
		if accuracy != big.Exact || idx < 0 || idx >= int64(typ.Length()) {
			return cty.NilType, false
		}
		return typ.TupleElementType(int(idx)), true
	case typ.IsObjectType():
		if key.Type() != cty.String || !typ.HasAttribute(key.AsString()) {
			return cty.NilType, false
		}
		return typ.AttributeType(key.AsString()), true
	}

	return cty.NilType, false
}
