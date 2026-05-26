// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package decoder

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/opentofu/hcl-lang/schema"
)

type TypeDeclaration struct {
	expr    hcl.Expression
	cons    schema.TypeDeclaration
	pathCtx *PathContext
}

func isTypeNameWithElementOnly(name string) bool {
	return name == "list" || name == "set" || name == "map"
}
