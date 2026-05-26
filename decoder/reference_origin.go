// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package decoder

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/opentofu/hcl-lang/lang"
)

type ReferenceOrigin struct {
	Path  lang.Path
	Range hcl.Range
}

type ReferenceOrigins []ReferenceOrigin
