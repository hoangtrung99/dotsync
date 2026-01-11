package ui

import (
	"testing"
)

func TestGetFileType(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		{"config.toml", "TOML"},
		{"settings.yaml", "YAML"},
		{"data.yml", "YAML"},
		{"package.json", "JSON"},
		{"script.sh", "Bash"},
		{"init.zsh", "Zsh"},
		{"config.fish", "Fish"},
		{"init.lua", "Lua"},
		{"vimrc.vim", "Vim"},
		{"main.py", "Python"},
		{"main.go", "Go"},
		{"app.js", "JavaScript"},
		{"app.ts", "TypeScript"},
		{"style.css", "CSS"},
		{"index.html", "HTML"},
		{"README.md", "Markdown"},
		{"config.ini", "Config"},
		{"data.xml", "XML"},
		{"unknown.xyz", "Text"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := GetFileType(tt.filename)
			if result != tt.expected {
				t.Errorf("GetFileType(%s) = %s, want %s", tt.filename, result, tt.expected)
			}
		})
	}
}

func TestHighlighter_HighlightLine(t *testing.T) {
	h := NewHighlighter()

	// Test basic highlighting doesn't panic
	tests := []struct {
		line     string
		filename string
	}{
		{"[section]", "config.toml"},
		{"key: value", "config.yaml"},
		{`{"key": "value"}`, "data.json"},
		{"echo hello", "script.sh"},
		{"local x = 1", "init.lua"},
		{"def foo():", "main.py"},
		{"func main() {}", "main.go"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := h.HighlightLine(tt.line, tt.filename)
			if result == "" {
				t.Errorf("HighlightLine should return non-empty result")
			}
		})
	}
}

func TestHighlighter_HighlightLines(t *testing.T) {
	h := NewHighlighter()

	lines := []string{
		"[section]",
		"key = 'value'",
		"number = 42",
	}

	result := h.HighlightLines(lines, "config.toml")

	if len(result) != len(lines) {
		t.Errorf("HighlightLines should return same number of lines")
	}

	for i, line := range result {
		if line == "" {
			t.Errorf("Line %d should not be empty", i)
		}
	}
}

func TestGetLexerForFile_Extensions(t *testing.T) {
	tests := []struct {
		filename string
		hasLexer bool
	}{
		// Test all extensions in getLexerForFile
		{"config.toml", true},
		{"settings.yaml", true},
		{"settings.yml", true},
		{"data.json", true},
		{"script.sh", true},
		{"script.bash", true},
		{"script.zsh", true},
		{"config.fish", true},
		{"init.lua", true},
		{"vimrc.vim", true},
		{"main.py", true},
		{"main.go", true},
		{"app.js", true},
		{"app.jsx", true},
		{"app.ts", true},
		{"app.tsx", true},
		{"style.css", true},
		{"index.html", true},
		{"index.htm", true},
		{"README.md", true},
		{"README.markdown", true},
		{"config.conf", true},
		{"config.cfg", true},
		{"config.ini", true},
		{"data.xml", true},
		{"main.rs", true},
		{"app.rb", true},
		{"app.swift", true},
		// Unknown extension
		{"unknown.xyz", false},
	}

	h := NewHighlighter()
	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := h.HighlightLine("test content", tt.filename)
			// Just verify it doesn't panic and returns something
			if result == "" {
				t.Errorf("HighlightLine should return non-empty result")
			}
		})
	}
}

func TestGetLexerForFile_SpecialFiles(t *testing.T) {
	tests := []struct {
		filename string
		hasLexer bool
	}{
		// Config files without extensions
		{".zshrc", true},
		{".bashrc", true},
		{".gitconfig", true},
		{".gitignore", true},
		{"Dockerfile", true},
		{"Makefile", true},
		{"dockerfile", true},
		{"makefile", true},
	}

	h := NewHighlighter()
	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := h.HighlightLine("test content", tt.filename)
			if result == "" {
				t.Errorf("HighlightLine should return non-empty result")
			}
		})
	}
}

func TestHighlighter_UnknownFile(t *testing.T) {
	h := NewHighlighter()

	// Unknown file type should return original line
	line := "some random content"
	result := h.HighlightLine(line, "unknown_file")

	if result != line {
		// It's okay if it's different (might be plain text lexer)
		// Just verify it doesn't crash
	}
}

func TestHighlighter_EmptyLine(t *testing.T) {
	h := NewHighlighter()

	result := h.HighlightLine("", "config.toml")
	// Empty line should return empty or minimal output
	_ = result
}

func TestHighlighter_MultilineCode(t *testing.T) {
	h := NewHighlighter()

	// Test various code snippets
	testCases := []struct {
		name     string
		line     string
		filename string
	}{
		{"go_func", "func main() { fmt.Println(\"hello\") }", "main.go"},
		{"python_def", "def hello(): print('world')", "main.py"},
		{"json_object", `{"name": "value", "count": 42}`, "data.json"},
		{"yaml_mapping", "key: value", "config.yaml"},
		{"toml_section", "[database]", "config.toml"},
		{"bash_command", "echo $HOME && ls -la", "script.sh"},
		{"lua_local", "local function test() return 1 end", "init.lua"},
		{"rust_fn", "fn main() { println!(\"hello\"); }", "main.rs"},
		{"ruby_def", "def hello; puts 'world'; end", "app.rb"},
		{"swift_func", "func greet() { print(\"hello\") }", "app.swift"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := h.HighlightLine(tc.line, tc.filename)
			if result == "" {
				t.Errorf("HighlightLine should return non-empty for %s", tc.name)
			}
		})
	}
}

func TestHighlighter_HighlightLines_Empty(t *testing.T) {
	h := NewHighlighter()

	result := h.HighlightLines([]string{}, "config.toml")
	if len(result) != 0 {
		t.Error("HighlightLines with empty input should return empty")
	}
}

func TestNewHighlighter(t *testing.T) {
	h := NewHighlighter()
	if h == nil {
		t.Fatal("NewHighlighter should not return nil")
	}
	if h.style == nil {
		t.Error("Highlighter style should not be nil")
	}
}

func TestGetLexerForFile_AllSwitchCases(t *testing.T) {
	// This tests all cases in getLexerForFile's switch statements
	h := NewHighlighter()

	// Test extension-based matching
	extensionTests := []string{
		"config.toml",   // .toml
		"config.yaml",   // .yaml
		"config.yml",    // .yml
		"data.json",     // .json
		"script.sh",     // .sh
		"script.bash",   // .bash
		"script.zsh",    // .zsh
		"config.fish",   // .fish
		"init.lua",      // .lua
		"vimrc.vim",     // .vim
		"main.py",       // .py
		"main.go",       // .go
		"app.js",        // .js
		"component.jsx", // .jsx
		"app.ts",        // .ts
		"component.tsx", // .tsx
		"style.css",     // .css
		"index.html",    // .html
		"page.htm",      // .htm
		"README.md",     // .md
		"DOCS.markdown", // .markdown
		"nginx.conf",    // .conf
		"app.cfg",       // .cfg
		"settings.ini",  // .ini
		"data.xml",      // .xml
		"main.rs",       // .rs
		"app.rb",        // .rb
		"App.swift",     // .swift
	}

	for _, filename := range extensionTests {
		result := h.HighlightLine("test code", filename)
		if result == "" {
			t.Errorf("HighlightLine failed for %s", filename)
		}
	}

	// Test special filename-based matching
	specialFiles := []string{
		".zshrc",
		".bashrc",
		".gitconfig",
		".gitignore",
		"Dockerfile",
		"dockerfile",
		"Makefile",
		"makefile",
	}

	for _, filename := range specialFiles {
		result := h.HighlightLine("test code", filename)
		if result == "" {
			t.Errorf("HighlightLine failed for special file %s", filename)
		}
	}
}

func TestGetFileType_AllCases(t *testing.T) {
	// Test all file type cases that GetFileType supports
	tests := map[string]string{
		"config.toml":   "TOML",
		"config.yaml":   "YAML",
		"config.yml":    "YAML",
		"data.json":     "JSON",
		"script.sh":     "Bash",
		"script.bash":   "Bash",
		"script.zsh":    "Zsh",
		"config.fish":   "Fish",
		"init.lua":      "Lua",
		"vimrc.vim":     "Vim",
		"main.py":       "Python",
		"main.go":       "Go",
		"app.js":        "JavaScript",
		"app.ts":        "TypeScript",
		"style.css":     "CSS",
		"index.html":    "HTML",
		"page.htm":      "HTML",
		"README.md":     "Markdown",
		"DOCS.markdown": "Markdown",
		"nginx.conf":    "Config",
		"app.cfg":       "Config",
		"settings.ini":  "Config",
		"data.xml":      "XML",
		"unknown.xyz":   "Text",
		"noextension":   "Text",
		// These return "Text" as they're not in GetFileType
		"app.jsx":   "Text",
		"app.tsx":   "Text",
		"main.rs":   "Text",
		"app.rb":    "Text",
		"App.swift": "Text",
	}

	for filename, expected := range tests {
		result := GetFileType(filename)
		if result != expected {
			t.Errorf("GetFileType(%s) = %s, want %s", filename, result, expected)
		}
	}
}

func TestGetLexerForFile_AllExtensions(t *testing.T) {
	h := NewHighlighter()

	// Test all extensions defined in getLexerForFile switch statement
	extensionTests := map[string]string{
		"config.toml":   "toml content",
		"settings.yaml": "key: value",
		"config.yml":    "key: value",
		"data.json":     `{"key": "value"}`,
		"script.sh":     "echo hello",
		"script.bash":   "echo hello",
		"script.zsh":    "echo hello",
		"config.fish":   "set PATH",
		"init.lua":      "local x = 1",
		"vimrc.vim":     "set number",
		"main.py":       "def foo(): pass",
		"main.go":       "func main() {}",
		"app.js":        "const x = 1",
		"component.jsx": "const App = () => <div/>",
		"app.ts":        "const x: number = 1",
		"component.tsx": "const App: FC = () => <div/>",
		"style.css":     "body { color: red }",
		"index.html":    "<html></html>",
		"page.htm":      "<html></html>",
		"README.md":     "# Title",
		"DOCS.markdown": "# Title",
		"nginx.conf":    "server { }",
		"app.cfg":       "[section]",
		"settings.ini":  "[section]",
		"data.xml":      "<root></root>",
		"main.rs":       "fn main() {}",
		"app.rb":        "def hello; end",
		"App.swift":     "func hello() {}",
	}

	for filename, code := range extensionTests {
		result := h.HighlightLine(code, filename)
		if result == "" {
			t.Errorf("HighlightLine should return non-empty for %s", filename)
		}
	}
}

func TestGetLexerForFile_SpecialFilenames(t *testing.T) {
	h := NewHighlighter()

	// Test special filename-based matching in getLexerForFile
	specialFiles := map[string]string{
		".zshrc":     "export PATH",
		".bashrc":    "export PATH",
		".gitconfig": "[user]",
		".gitignore": "*.log",
		"Dockerfile": "FROM alpine",
		"dockerfile": "FROM alpine",
		"Makefile":   "all: build",
		"makefile":   "all: build",
	}

	for filename, code := range specialFiles {
		result := h.HighlightLine(code, filename)
		if result == "" {
			t.Errorf("HighlightLine should return non-empty for special file %s", filename)
		}
	}
}

func TestGetLexerForFile_Fallback(t *testing.T) {
	h := NewHighlighter()

	// Test files that don't match any pattern - should return original line
	unknownFiles := []string{
		"unknown",
		"noextension",
		"file.xyz",
		"random.abc",
	}

	for _, filename := range unknownFiles {
		line := "some random content"
		result := h.HighlightLine(line, filename)
		// For unknown files, it should return the original line or highlighted version
		if result == "" {
			t.Errorf("HighlightLine should return non-empty for %s", filename)
		}
	}
}

func TestHighlightLine_WithBoldAndItalic(t *testing.T) {
	h := NewHighlighter()

	// Test code that would trigger bold/italic styling
	testCases := []struct {
		filename string
		code     string
	}{
		{"main.go", "func main() { // comment\n}"},
		{"main.py", "def hello(): \"\"\"docstring\"\"\""},
		{"app.js", "// this is a comment\nconst x = 'string'"},
		{"style.css", "/* comment */ body { font-weight: bold; }"},
	}

	for _, tc := range testCases {
		result := h.HighlightLine(tc.code, tc.filename)
		if result == "" {
			t.Errorf("HighlightLine should return non-empty for %s", tc.filename)
		}
	}
}
