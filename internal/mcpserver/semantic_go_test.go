package mcpserver

import (
	"context"
	"os"
	"strings"
	"testing"
)

const semanticFixture = `package sample

type Service struct{}

func helper(value string) string {
	return value
}

func (s *Service) Run(input string) string {
	return helper(input)
}

func caller() string {
	s := &Service{}
	return s.Run("ok")
}
`

func writeSemanticFixture(t *testing.T) {
	t.Helper()
	if err := os.WriteFile("sample.go", []byte(semanticFixture), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestGoSymbolsReturnsQualifiedMethodsAndSignatures(t *testing.T) {
	inWorkspace(t)
	writeSemanticFixture(t)
	result, err := handleGoSymbols(context.Background(), map[string]any{"path": "sample.go"})
	if err != nil {
		t.Fatal(err)
	}
	text := result.(string)
	for _, want := range []string{"type Service", "func helper", "func Service.Run", "func caller"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q from symbol outline:\n%s", want, text)
		}
	}
}

func TestFindGoSymbolCanReturnOnlyRequestedBody(t *testing.T) {
	inWorkspace(t)
	writeSemanticFixture(t)
	result, err := handleFindGoSymbol(context.Background(), map[string]any{
		"path":         "sample.go",
		"name":         "Service.Run",
		"include_body": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := result.(string)
	if !strings.Contains(text, "return helper(input)") || strings.Contains(text, "func caller") {
		t.Fatalf("symbol retrieval returned incorrect scope:\n%s", text)
	}
}

func TestGoReferencesAndCallersReturnExactLocations(t *testing.T) {
	inWorkspace(t)
	writeSemanticFixture(t)
	references, err := handleFindGoReferences(context.Background(), map[string]any{"name": "helper"})
	if err != nil {
		t.Fatal(err)
	}
	if count := strings.Count(references.(string), "sample.go:"); count != 2 {
		t.Fatalf("expected definition and call reference, got:\n%s", references)
	}
	callers, err := handleGoCallers(context.Background(), map[string]any{"name": "Run"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(callers.(string), "caller -> Run") {
		t.Fatalf("expected caller relationship, got:\n%s", callers)
	}
}

func TestReplaceGoSymbolChangesOnlySelectedBody(t *testing.T) {
	inWorkspace(t)
	writeSemanticFixture(t)
	result, err := handleReplaceGoSymbol(context.Background(), map[string]any{
		"path": "sample.go",
		"name": "Service.Run",
		"body": `return "changed:" + input`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.(string), "replaced Service.Run") {
		t.Fatalf("unexpected result: %s", result)
	}
	content, err := os.ReadFile("sample.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	if !strings.Contains(text, `return "changed:" + input`) {
		t.Fatalf("replacement missing:\n%s", text)
	}
	if !strings.Contains(text, "func helper(value string) string") || !strings.Contains(text, "return value") {
		t.Fatalf("unrelated helper was changed:\n%s", text)
	}
}

func TestSemanticToolDefinitionsAreRegistered(t *testing.T) {
	definitions := GetToolDefinitions()
	names := map[string]bool{}
	for _, definition := range definitions {
		name, _ := definition["name"].(string)
		names[name] = true
	}
	for _, want := range []string{"go_symbols", "find_go_symbol", "find_go_references", "go_callers", "replace_go_symbol", "go_diagnostics"} {
		if !names[want] {
			t.Fatalf("semantic tool %s is not exposed", want)
		}
	}
}
