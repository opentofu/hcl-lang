// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package decoder

import (
	"context"

	"github.com/hashicorp/hcl/v2"
	"github.com/opentofu/hcl-lang/lang"
)

type unknownExpression struct{}

func (oo unknownExpression) CompletionAtPos(ctx context.Context, pos hcl.Pos) []lang.Candidate {
	return []lang.Candidate{}
}

func (oo unknownExpression) HoverAtPos(ctx context.Context, pos hcl.Pos) *lang.HoverData {
	return nil
}

func (oo unknownExpression) SemanticTokens(ctx context.Context) []lang.SemanticToken {
	return []lang.SemanticToken{}
}
