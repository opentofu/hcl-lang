// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package decoder

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/opentofu/hcl-lang/schema"
)

type Tuple struct {
	expr    hcl.Expression
	cons    schema.Tuple
	pathCtx *PathContext
}
