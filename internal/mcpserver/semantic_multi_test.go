package mcpserver

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestCodeSymbolsSupportsMajorLanguageFamilies(t *testing.T) {
	inWorkspace(t)
	fixtures := map[string]string{
		"service.py": "def load_user(user_id):\n    return user_id\n\nclass UserService:\n    pass\n",
		"client.ts":  "export interface Client {\n  run(): void\n}\nexport const createClient = () => {\n  return {}\n}\n",
		"lib.rs":     "pub struct Store {}\n\npub fn open_store() {\n}\n",
		"Main.java":  "public class Main {\n  public void execute() {\n  }\n}\n",
	}
	for path, content := range fixtures {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	result, err := handleCodeSymbols(context.Background(), map[string]any{"path": "."})
	if err != nil {
		t.Fatal(err)
	}
	text := result.(string)
	for _, want := range []string{
		"[python] def load_user",
		"[python] class UserService",
		"[typescript] interface Client",
		"[typescript] function createClient",
		"[rust] struct Store",
		"[rust] fn open_store",
		"[jvm] class Main",
		"[jvm] function execute",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q from multi-language outline:\n%s", want, text)
		}
	}
}

func TestFindCodeSymbolReturnsPythonBodyOnly(t *testing.T) {
	inWorkspace(t)
	content := "def first():\n    value = 1\n    return value\n\ndef second():\n    return 2\n"
	if err := os.WriteFile("sample.py", []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := handleFindCodeSymbol(context.Background(), map[string]any{
		"path":         "sample.py",
		"name":         "first",
		"include_body": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	text := result.(string)
	if !strings.Contains(text, "return value") || strings.Contains(text, "def second") {
		t.Fatalf("symbol body escaped its scope:\n%s", text)
	}
}

func TestCodeSymbolsHonorsResponseBudget(t *testing.T) {
	inWorkspace(t)
	var source strings.Builder
	for i := 0; i < 100; i++ {
		source.WriteString("def symbol_")
		source.WriteString(strings.Repeat("x", 20))
		source.WriteString("():\n    pass\n")
	}
	if err := os.WriteFile("large.py", []byte(source.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := handleCodeSymbols(context.Background(), map[string]any{"max_chars": 400})
	if err != nil {
		t.Fatal(err)
	}
	text := result.(string)
	if len(text) > 480 || !strings.Contains(text, "truncated") {
		t.Fatalf("symbol output did not respect compact budget: %d bytes\n%s", len(text), text)
	}
}
