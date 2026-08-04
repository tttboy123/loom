package api_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestP3AAPIModelsDeclareStrictAssetWireSurface(t *testing.T) {
	parsed, err := parser.ParseFile(
		token.NewFileSet(), "local_product_assets.go", nil, parser.SkipObjectResolution,
	)
	if err != nil {
		t.Fatalf("parse required asset API models: %v", err)
	}
	declared := make(map[string]bool)
	for _, declaration := range parsed.Decls {
		if general, ok := declaration.(*ast.GenDecl); ok {
			for _, specification := range general.Specs {
				if named, ok := specification.(*ast.TypeSpec); ok {
					declared[named.Name.Name] = true
				}
			}
		}
	}
	for _, name := range []string{
		"EvolutionAssetSnapshotRequest",
		"EvolutionAssetSnapshot",
		"EvolutionAssetDiffRequest",
		"EvolutionAssetDiff",
		"EvolutionAssetCommandRequest",
		"EvolutionAssetCommandResult",
	} {
		if !declared[name] {
			t.Fatalf("asset API type %s is missing", name)
		}
	}
}
