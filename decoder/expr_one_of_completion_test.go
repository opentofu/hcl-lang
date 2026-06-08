// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package decoder

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/hcl-lang/lang"
	"github.com/hashicorp/hcl-lang/reference"
	"github.com/hashicorp/hcl-lang/schema"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

func TestCompletionAtPos_exprOneOf(t *testing.T) {
	testCases := []struct {
		testName           string
		attrSchema         map[string]*schema.AttributeSchema
		refTargets         reference.Targets
		cfg                string
		pos                hcl.Pos
		expectedCandidates lang.Candidates
	}{
		{
			"all expressions",
			map[string]*schema.AttributeSchema{
				"attr": {
					Constraint: schema.OneOf{
						schema.Keyword{
							Keyword: "akeyword",
						},
						schema.Keyword{
							Keyword: "bkeyword",
						},
						schema.Keyword{
							Keyword: "ckeyword",
						},
					},
				},
			},
			nil,
			`attr =
		`,
			hcl.Pos{Line: 1, Column: 8, Byte: 7},
			lang.CompleteCandidates([]lang.Candidate{
				{
					Label:  "akeyword",
					Detail: "keyword",
					Kind:   lang.KeywordCandidateKind,
					TextEdit: lang.TextEdit{
						NewText: "akeyword",
						Snippet: "akeyword",
						Range: hcl.Range{
							Filename: "test.tf",
							Start:    hcl.Pos{Line: 1, Column: 8, Byte: 7},
							End:      hcl.Pos{Line: 1, Column: 8, Byte: 7},
						},
					},
				},
				{
					Label:  "bkeyword",
					Detail: "keyword",
					Kind:   lang.KeywordCandidateKind,
					TextEdit: lang.TextEdit{
						NewText: "bkeyword",
						Snippet: "bkeyword",
						Range: hcl.Range{
							Filename: "test.tf",
							Start:    hcl.Pos{Line: 1, Column: 8, Byte: 7},
							End:      hcl.Pos{Line: 1, Column: 8, Byte: 7},
						},
					},
				},
				{
					Label:  "ckeyword",
					Detail: "keyword",
					Kind:   lang.KeywordCandidateKind,
					TextEdit: lang.TextEdit{
						NewText: "ckeyword",
						Snippet: "ckeyword",
						Range: hcl.Range{
							Filename: "test.tf",
							Start:    hcl.Pos{Line: 1, Column: 8, Byte: 7},
							End:      hcl.Pos{Line: 1, Column: 8, Byte: 7},
						},
					},
				},
			}),
		},
		{
			"partial match first",
			map[string]*schema.AttributeSchema{
				"attr": {
					Constraint: schema.OneOf{
						schema.Keyword{
							Keyword: "akeyword",
						},
						schema.Keyword{
							Keyword: "bkeyword",
						},
						schema.Keyword{
							Keyword: "ckeyword",
						},
					},
				},
			},
			nil,
			`attr = akey
		`,
			hcl.Pos{Line: 1, Column: 12, Byte: 11},
			lang.CompleteCandidates([]lang.Candidate{
				{
					Label:  "akeyword",
					Detail: "keyword",
					Kind:   lang.KeywordCandidateKind,
					TextEdit: lang.TextEdit{
						NewText: "akeyword",
						Snippet: "akeyword",
						Range: hcl.Range{
							Filename: "test.tf",
							Start:    hcl.Pos{Line: 1, Column: 8, Byte: 7},
							End:      hcl.Pos{Line: 1, Column: 12, Byte: 11},
						},
					},
				},
			}),
		},
		{
			"partial match multiple",
			map[string]*schema.AttributeSchema{
				"attr": {
					Constraint: schema.OneOf{
						schema.Keyword{
							Keyword: "akeyword",
						},
						schema.Keyword{
							Keyword: "keyword1",
						},
						schema.Keyword{
							Keyword: "keyword2",
						},
					},
				},
			},
			nil,
			`attr = key
		`,
			hcl.Pos{Line: 1, Column: 11, Byte: 10},
			lang.CompleteCandidates([]lang.Candidate{
				{
					Label:  "keyword1",
					Detail: "keyword",
					Kind:   lang.KeywordCandidateKind,
					TextEdit: lang.TextEdit{
						NewText: "keyword1",
						Snippet: "keyword1",
						Range: hcl.Range{
							Filename: "test.tf",
							Start:    hcl.Pos{Line: 1, Column: 8, Byte: 7},
							End:      hcl.Pos{Line: 1, Column: 11, Byte: 10},
						},
					},
				},
				{
					Label:  "keyword2",
					Detail: "keyword",
					Kind:   lang.KeywordCandidateKind,
					TextEdit: lang.TextEdit{
						NewText: "keyword2",
						Snippet: "keyword2",
						Range: hcl.Range{
							Filename: "test.tf",
							Start:    hcl.Pos{Line: 1, Column: 8, Byte: 7},
							End:      hcl.Pos{Line: 1, Column: 11, Byte: 10},
						},
					},
				},
			}),
		},
		{
			"no expr defined",
			map[string]*schema.AttributeSchema{
				"attr": {
					Constraint: schema.OneOf{},
				},
			},
			nil,
			`attr =
		`,
			hcl.Pos{Line: 1, Column: 8, Byte: 7},
			lang.CompleteCandidates([]lang.Candidate{}),
		},
		{
			"no duplicate references across for_each constraints",
			// Based on attribute_extensions.go ForEachAttributeSchema
			map[string]*schema.AttributeSchema{
				"attr": {
					Constraint: schema.OneOf{
						schema.AnyExpression{OfType: cty.Map(cty.DynamicPseudoType)},
						schema.AnyExpression{OfType: cty.Set(cty.String)},
						schema.AnyExpression{OfType: cty.EmptyObject},
					},
				},
			},
			reference.Targets{
				{
					Addr: lang.Address{
						lang.RootStep{Name: "local"},
						lang.AttrStep{Name: "some_set"},
					},
					Type: cty.DynamicPseudoType,
				},
			},
			`attr = local.some`,
			hcl.Pos{Line: 1, Column: 18, Byte: 17},
			lang.CompleteCandidates([]lang.Candidate{
				{
					Label:  "local.some_set",
					Detail: "dynamic",
					Kind:   lang.ReferenceCandidateKind,
					TextEdit: lang.TextEdit{
						NewText: "local.some_set",
						Snippet: "local.some_set",
						Range: hcl.Range{
							Filename: "test.tf",
							Start:    hcl.Pos{Line: 1, Column: 8, Byte: 7},
							End:      hcl.Pos{Line: 1, Column: 18, Byte: 17},
						},
					},
				},
			}),
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("%d-%s", i, tc.testName), func(t *testing.T) {
			bodySchema := &schema.BodySchema{
				Attributes: tc.attrSchema,
			}

			f, _ := hclsyntax.ParseConfig([]byte(tc.cfg), "test.tf", hcl.InitialPos)
			d := testPathDecoder(t, &PathContext{
				Schema: bodySchema,
				Files: map[string]*hcl.File{
					"test.tf": f,
				},
				ReferenceTargets: tc.refTargets,
			})

			ctx := context.Background()
			candidates, err := d.CompletionAtPos(ctx, "test.tf", tc.pos)
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(tc.expectedCandidates, candidates); diff != "" {
				t.Logf("pos: %#v, config: %s\n", tc.pos, tc.cfg)
				t.Fatalf("unexpected candidates: %s", diff)
			}
		})
	}
}
