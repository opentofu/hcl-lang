// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package decoder

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/hcl-lang/lang"
	"github.com/hashicorp/hcl-lang/schema"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

func TestDecoder_CandidateAtPos_incompleteLabels(t *testing.T) {
	ctx := context.Background()
	bodySchema := &schema.BodySchema{
		Blocks: map[string]*schema.BlockSchema{
			"customblock": {
				Labels: []*schema.LabelSchema{
					{
						Name:        "type",
						IsDepKey:    true,
						Completable: true,
					},
				},
				DependentBody: map[schema.SchemaKey]*schema.BodySchema{
					schema.NewSchemaKey(schema.DependencyKeys{
						Labels: []schema.LabelDependent{
							{
								Index: 0,
								Value: "first",
							},
						},
					}): {
						Attributes: map[string]*schema.AttributeSchema{
							"attr1": {Constraint: schema.LiteralType{Type: cty.Number}},
						},
					},
					schema.NewSchemaKey(schema.DependencyKeys{
						Labels: []schema.LabelDependent{
							{
								Index: 0,
								Value: "second",
							},
						},
					}): {
						Attributes: map[string]*schema.AttributeSchema{
							"attr2": {Constraint: schema.LiteralType{Type: cty.Number}},
						},
					},
				},
			},
		},
	}

	f, _ := hclsyntax.ParseConfig([]byte(`customblock "" {
}
`), "test.tf", hcl.InitialPos)

	d := testPathDecoder(t, &PathContext{
		Schema: bodySchema,
		Files: map[string]*hcl.File{
			"test.tf": f,
		},
	})
	d.maxCandidates = 1

	candidates, err := d.CompletionAtPos(ctx, "test.tf", hcl.Pos{
		Line:   1,
		Column: 14,
		Byte:   13,
	})
	if err != nil {
		t.Fatal(err)
	}
	expectedCandidates := lang.Candidates{
		List: []lang.Candidate{
			{
				Label: "first",
				TextEdit: lang.TextEdit{
					Range: hcl.Range{
						Filename: "test.tf",
						Start: hcl.Pos{
							Line:   1,
							Column: 14,
							Byte:   13,
						},
						End: hcl.Pos{
							Line:   1,
							Column: 14,
							Byte:   13,
						},
					},
					NewText: "first",
					Snippet: "first",
				},
				Kind: lang.LabelCandidateKind,
			},
		},
		IsComplete: false,
	}
	if diff := cmp.Diff(expectedCandidates, candidates); diff != "" {
		t.Fatalf("unexpected candidates: %s", diff)
	}
}

func TestDecoder_CandidateAtPos_incompleteLabel_errorRecoveredBlock(t *testing.T) {
	bodySchema := &schema.BodySchema{
		Blocks: map[string]*schema.BlockSchema{
			"customblock": {
				Labels: []*schema.LabelSchema{
					{
						Name:        "type",
						IsDepKey:    true,
						Completable: true,
					},
				},
				DependentBody: map[schema.SchemaKey]*schema.BodySchema{
					schema.NewSchemaKey(schema.DependencyKeys{
						Labels: []schema.LabelDependent{
							{
								Index: 0,
								Value: "first",
							},
						},
					}): {
						Attributes: map[string]*schema.AttributeSchema{
							"attr1": {Constraint: schema.LiteralType{Type: cty.Number}},
						},
					},
					schema.NewSchemaKey(schema.DependencyKeys{
						Labels: []schema.LabelDependent{
							{
								Index: 0,
								Value: "second",
							},
						},
					}): {
						Attributes: map[string]*schema.AttributeSchema{
							"attr2": {Constraint: schema.LiteralType{Type: cty.Number}},
						},
					},
				},
			},
		},
	}

	// Incomplete block: no closing quote, no braces
	f, _ := hclsyntax.ParseConfig([]byte(`customblock "fi`), "test.tf", hcl.InitialPos)

	body := f.Body.(*hclsyntax.Body)
	if len(body.Blocks) == 0 {
		t.Fatal("expected parser error recovery to produce a block")
	}
	block := body.Blocks[0]
	if len(block.LabelRanges) == 0 {
		t.Fatal("expected parser error recovery to produce label ranges")
	}

	labelRange := block.LabelRanges[0]
	if block.Range().ContainsPos(labelRange.Start) {
		t.Log("note: block.Range() contains label start (precondition not as expected)")
	}

	t.Run("cursor at end of label range", func(t *testing.T) {
		pos := labelRange.End

		d := testPathDecoder(t, &PathContext{
			Schema: bodySchema,
			Files: map[string]*hcl.File{
				"test.tf": f,
			},
		})
		d.maxCandidates = 2

		candidates, err := d.CompletionAtPos(t.Context(), "test.tf", pos)
		if err != nil {
			t.Fatal(err)
		}
		if len(candidates.List) == 0 {
			t.Fatal("expected at least one candidate")
		}
		if candidates.List[0].Kind != lang.LabelCandidateKind {
			t.Fatalf("expected LabelCandidateKind, got %s", candidates.List[0].Kind)
		}
		// With prefix "fi", only "first" should match (not "second")
		if len(candidates.List) != 1 {
			t.Fatalf("expected 1 candidate (prefix-filtered by 'fi'), got %d", len(candidates.List))
		}
		if candidates.List[0].Label != "first" {
			t.Fatalf("expected label 'first', got %q", candidates.List[0].Label)
		}
		if !candidates.IsComplete {
			t.Fatal("expected candidates to be complete (not limited by maxCandidates)")
		}
	})

	t.Run("cursor at end of label with trailing newline", func(t *testing.T) {
		// Real-world case: editors append a trailing newline (ie, nvim)
		fNl, _ := hclsyntax.ParseConfig([]byte("customblock \"fi\n"), "test.tf", hcl.InitialPos)
		bodyNl := fNl.Body.(*hclsyntax.Body)
		if len(bodyNl.Blocks) == 0 {
			t.Fatal("expected parser error recovery to produce a block")
		}
		blockNl := bodyNl.Blocks[0]
		// Cursor right after "fi" at byte 15, before the newline
		pos := hcl.Pos{Line: 1, Column: 16, Byte: 15}

		d := testPathDecoder(t, &PathContext{
			Schema: bodySchema,
			Files: map[string]*hcl.File{
				"test.tf": fNl,
			},
		})
		d.maxCandidates = 2

		_ = blockNl // used for debugging
		candidates, err := d.CompletionAtPos(t.Context(), "test.tf", pos)
		if err != nil {
			t.Fatal(err)
		}
		if len(candidates.List) == 0 {
			t.Fatal("expected at least one candidate")
		}
		if candidates.List[0].Kind != lang.LabelCandidateKind {
			t.Fatalf("expected LabelCandidateKind, got %s", candidates.List[0].Kind)
		}
	})

	t.Run("cursor inside label range", func(t *testing.T) {
		// Position one byte into the label content
		pos := labelRange.Start
		pos.Byte++
		pos.Column++

		d := testPathDecoder(t, &PathContext{
			Schema: bodySchema,
			Files: map[string]*hcl.File{
				"test.tf": f,
			},
		})
		d.maxCandidates = 1

		candidates, err := d.CompletionAtPos(t.Context(), "test.tf", pos)
		if err != nil {
			t.Fatal(err)
		}
		if len(candidates.List) == 0 {
			t.Fatal("expected at least one candidate")
		}
		if candidates.List[0].Kind != lang.LabelCandidateKind {
			t.Fatalf("expected LabelCandidateKind, got %s", candidates.List[0].Kind)
		}
	})
}

func TestCompletionAtPos_prefillRequiredFields(t *testing.T) {
	ctx := context.Background()
	startingConfig := "resource \"\" {\n}"
	startingPos := hcl.Pos{
		Line:   1,
		Column: 11,
		Byte:   10,
	}
	wantRange := hcl.Range{
		Filename: "test.tf",
		Start: hcl.Pos{
			Line:   1,
			Column: 11,
			Byte:   10,
		},
		End: hcl.Pos{
			Line:   1,
			Column: 14,
			Byte:   13,
		},
	}
	tests := []struct {
		name    string
		prefill bool
		schema  *schema.BodySchema
		want    lang.Candidates
	}{
		{
			name:    "one dependent label no attributes or blocks",
			prefill: true,
			schema: &schema.BodySchema{
				Blocks: map[string]*schema.BlockSchema{
					"resource": {
						Labels: []*schema.LabelSchema{
							{
								Name:        "type",
								IsDepKey:    true,
								Completable: true,
							},
							{
								Name: "name",
							},
						},
						DependentBody: map[schema.SchemaKey]*schema.BodySchema{
							schema.NewSchemaKey(schema.DependencyKeys{
								Labels: []schema.LabelDependent{
									{Index: 0, Value: "aws_appmesh_route"},
								},
							}): {
								Attributes: map[string]*schema.AttributeSchema{
									"name": {
										Constraint: schema.LiteralType{Type: cty.String},
										IsRequired: false,
									},
								},
							},
						},
					},
				},
			},
			want: lang.CompleteCandidates([]lang.Candidate{
				{
					Label: "aws_appmesh_route",
					Kind:  lang.LabelCandidateKind,
					TextEdit: lang.TextEdit{
						Range:   wantRange,
						NewText: `aws_appmesh_route`,
						Snippet: `aws_appmesh_route" "${2:name}" {
	${0}`,
					},
				},
			}),
		},
		{
			name:    "one dependent label one required attributes no blocks",
			prefill: true,
			schema: &schema.BodySchema{
				Blocks: map[string]*schema.BlockSchema{
					"resource": {
						Labels: []*schema.LabelSchema{
							{
								Name:        "type",
								IsDepKey:    true,
								Completable: true,
							},
							{
								Name: "name",
							},
						},
						DependentBody: map[schema.SchemaKey]*schema.BodySchema{
							schema.NewSchemaKey(schema.DependencyKeys{
								Labels: []schema.LabelDependent{
									{Index: 0, Value: "aws_appmesh_route"},
								},
							}): {
								Attributes: map[string]*schema.AttributeSchema{
									"foo": {
										Constraint: schema.LiteralType{Type: cty.String},
										IsRequired: true,
									},
								},
							},
						},
					},
				},
			},
			want: lang.CompleteCandidates([]lang.Candidate{
				{
					Label: "aws_appmesh_route",
					Kind:  lang.LabelCandidateKind,
					TextEdit: lang.TextEdit{
						Range:   wantRange,
						NewText: `aws_appmesh_route`,
						Snippet: `aws_appmesh_route" "${2:name}" {
	foo = "${3:value}"
	${0}`,
					},
				},
			}),
		},
		{
			name:    "one dependent label multiple required attributes one required block",
			prefill: true,
			schema: &schema.BodySchema{
				Blocks: map[string]*schema.BlockSchema{
					"resource": {
						Labels: []*schema.LabelSchema{
							{
								Name:        "type",
								IsDepKey:    true,
								Completable: true,
							},
							{
								Name: "name",
							},
						},
						DependentBody: map[schema.SchemaKey]*schema.BodySchema{
							schema.NewSchemaKey(schema.DependencyKeys{
								Labels: []schema.LabelDependent{
									{Index: 0, Value: "aws_appmesh_route"},
								},
							}): {
								Attributes: map[string]*schema.AttributeSchema{
									"name": {
										Constraint: schema.LiteralType{Type: cty.String},
										IsRequired: true,
									},
									"anothername": {
										Constraint: schema.LiteralType{Type: cty.String},
										IsRequired: true,
									},
								},
								Blocks: map[string]*schema.BlockSchema{
									"spec": {
										Type:     schema.BlockTypeMap,
										MinItems: 1,
									},
								},
							},
						},
					},
				},
			},
			want: lang.CompleteCandidates([]lang.Candidate{
				{
					Label: "aws_appmesh_route",
					Kind:  lang.LabelCandidateKind,
					TextEdit: lang.TextEdit{
						Range:   wantRange,
						NewText: `aws_appmesh_route`,
						Snippet: `aws_appmesh_route" "${2:name}" {
	anothername = "${3:value}"
	name = "${4:value}"
	spec {
	}
	${0}`,
					},
				},
			}),
		},
		{
			name:    "one dependent label multiple required attributes one required block with required attribute",
			prefill: true,
			schema: &schema.BodySchema{
				Blocks: map[string]*schema.BlockSchema{
					"resource": {
						Labels: []*schema.LabelSchema{
							{
								Name:        "type",
								IsDepKey:    true,
								Completable: true,
							},
							{
								Name: "name",
							},
						},
						DependentBody: map[schema.SchemaKey]*schema.BodySchema{
							schema.NewSchemaKey(schema.DependencyKeys{
								Labels: []schema.LabelDependent{
									{Index: 0, Value: "aws_appmesh_route"},
								},
							}): {
								Attributes: map[string]*schema.AttributeSchema{
									"name": {
										Constraint: schema.LiteralType{Type: cty.String},
										IsRequired: true,
									},
									"anothername": {
										Constraint: schema.LiteralType{Type: cty.String},
										IsRequired: true,
									},
								},
								Blocks: map[string]*schema.BlockSchema{
									"spec": {
										Type:     schema.BlockTypeMap,
										MinItems: 1,
										Body: &schema.BodySchema{
											Attributes: map[string]*schema.AttributeSchema{
												"name": {
													Constraint: schema.LiteralType{Type: cty.String},
													IsRequired: true,
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: lang.CompleteCandidates([]lang.Candidate{
				{
					Label: "aws_appmesh_route",
					Kind:  lang.LabelCandidateKind,
					TextEdit: lang.TextEdit{
						Range:   wantRange,
						NewText: `aws_appmesh_route`,
						Snippet: `aws_appmesh_route" "${2:name}" {
	anothername = "${3:value}"
	name = "${4:value}"
	spec {
		name = "${5:value}"
	}
	${0}`,
					},
				},
			}),
		},
		{
			name:    "one dependent label multiple required attributes one required block with dependent label with no required attributes",
			prefill: true,
			schema: &schema.BodySchema{
				Blocks: map[string]*schema.BlockSchema{
					"resource": {
						Labels: []*schema.LabelSchema{
							{
								Name:        "type",
								IsDepKey:    true,
								Completable: true,
							},
							{
								Name: "name",
							},
						},
						DependentBody: map[schema.SchemaKey]*schema.BodySchema{
							schema.NewSchemaKey(schema.DependencyKeys{
								Labels: []schema.LabelDependent{
									{Index: 0, Value: "aws_appmesh_route"},
								},
							}): {
								Attributes: map[string]*schema.AttributeSchema{
									"name": {
										Constraint: schema.LiteralType{Type: cty.String},
										IsRequired: true,
									},
									"anothername": {
										Constraint: schema.LiteralType{Type: cty.String},
										IsRequired: true,
									},
								},
								Blocks: map[string]*schema.BlockSchema{
									"spec": {
										Type: schema.BlockTypeMap,
										Labels: []*schema.LabelSchema{
											{
												Name: "key",
											},
										},
										MinItems: 1,
										Body: &schema.BodySchema{
											Attributes: map[string]*schema.AttributeSchema{
												"name": {
													Constraint: schema.LiteralType{Type: cty.String},
													IsRequired: false,
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: lang.CompleteCandidates([]lang.Candidate{
				{
					Label: "aws_appmesh_route",
					Kind:  lang.LabelCandidateKind,
					TextEdit: lang.TextEdit{
						Range:   wantRange,
						NewText: `aws_appmesh_route`,
						Snippet: `aws_appmesh_route" "${2:name}" {
	anothername = "${3:value}"
	name = "${4:value}"
	spec "${5:key}" {
	}
	${0}`,
					},
				},
			}),
		},
		{
			name:    "one dependent label multiple required attributes one required block with dependent label with required attribute",
			prefill: true,
			schema: &schema.BodySchema{
				Blocks: map[string]*schema.BlockSchema{
					"resource": {
						Labels: []*schema.LabelSchema{
							{
								Name:        "type",
								IsDepKey:    true,
								Completable: true,
							},
							{
								Name: "name",
							},
						},
						DependentBody: map[schema.SchemaKey]*schema.BodySchema{
							schema.NewSchemaKey(schema.DependencyKeys{
								Labels: []schema.LabelDependent{
									{Index: 0, Value: "aws_appmesh_route"},
								},
							}): {
								Attributes: map[string]*schema.AttributeSchema{
									"name": {
										Constraint: schema.LiteralType{Type: cty.String},
										IsRequired: true,
									},
									"anothername": {
										Constraint: schema.LiteralType{Type: cty.String},
										IsRequired: true,
									},
								},
								Blocks: map[string]*schema.BlockSchema{
									"spec": {
										Type: schema.BlockTypeMap,
										Labels: []*schema.LabelSchema{
											{
												Name: "key",
											},
										},
										MinItems: 1,
										Body: &schema.BodySchema{
											Attributes: map[string]*schema.AttributeSchema{
												"name": {
													Constraint: schema.LiteralType{Type: cty.String},
													IsRequired: true,
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: lang.CompleteCandidates([]lang.Candidate{
				{
					Label: "aws_appmesh_route",
					Kind:  lang.LabelCandidateKind,
					TextEdit: lang.TextEdit{
						Range:   wantRange,
						NewText: `aws_appmesh_route`,
						Snippet: `aws_appmesh_route" "${2:name}" {
	anothername = "${3:value}"
	name = "${4:value}"
	spec "${5:key}" {
		name = "${6:value}"
	}
	${0}`,
					},
				},
			}),
		},
		{
			name:    "one dependent label multiple required attributes one required block with multiple nested required blocks with required attributes",
			prefill: true,
			schema: &schema.BodySchema{
				Blocks: map[string]*schema.BlockSchema{
					"resource": {
						Labels: []*schema.LabelSchema{
							{
								Name:        "type",
								IsDepKey:    true,
								Completable: true,
							},
							{
								Name: "name",
							},
						},
						DependentBody: map[schema.SchemaKey]*schema.BodySchema{
							schema.NewSchemaKey(schema.DependencyKeys{
								Labels: []schema.LabelDependent{
									{Index: 0, Value: "aws_appmesh_route"},
								},
							}): {
								Attributes: map[string]*schema.AttributeSchema{
									"name": {
										Constraint: schema.LiteralType{Type: cty.String},
										IsRequired: true,
									},
									"anothername": {
										Constraint: schema.LiteralType{Type: cty.String},
										IsRequired: true,
									},
								},
								Blocks: map[string]*schema.BlockSchema{
									"spec": {
										Type:     schema.BlockTypeList,
										MinItems: 1,
										Body: &schema.BodySchema{
											Attributes: map[string]*schema.AttributeSchema{
												"name": {
													Constraint: schema.LiteralType{Type: cty.String},
													IsRequired: true,
												},
											},
											Blocks: map[string]*schema.BlockSchema{
												"listener": {
													Type:     schema.BlockTypeList,
													MinItems: 1,
													Body: &schema.BodySchema{
														Blocks: map[string]*schema.BlockSchema{
															"port_mapping": {
																Type:     schema.BlockTypeList,
																MinItems: 1,
																Body: &schema.BodySchema{
																	Attributes: map[string]*schema.AttributeSchema{
																		"port": {
																			Constraint: schema.LiteralType{Type: cty.Number},
																			IsRequired: true,
																		},
																		"protocol": {
																			Constraint: schema.LiteralType{Type: cty.String},
																			IsRequired: true,
																		},
																	},
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			want: lang.CompleteCandidates([]lang.Candidate{
				{
					Label: "aws_appmesh_route",
					Kind:  lang.LabelCandidateKind,
					TextEdit: lang.TextEdit{
						Range:   wantRange,
						NewText: `aws_appmesh_route`,
						Snippet: `aws_appmesh_route" "${2:name}" {
	anothername = "${3:value}"
	name = "${4:value}"
	spec {
		name = "${5:value}"
		listener {
			port_mapping {
				port = ${6:0}
				protocol = "${7:value}"
			}
		}
	}
	${0}`,
					},
				},
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, _ := hclsyntax.ParseConfig([]byte(startingConfig), "test.tf", hcl.InitialPos)

			d := testPathDecoder(t, &PathContext{
				Schema: tt.schema,
				Files: map[string]*hcl.File{
					"test.tf": f,
				},
			})
			d.maxCandidates = 1
			d.PrefillRequiredFields = tt.prefill

			got, err := d.CompletionAtPos(ctx, "test.tf", startingPos)
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("unexpected candidates: %s", diff)
			}
		})
	}
}
