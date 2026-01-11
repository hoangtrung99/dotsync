package ui

import (
	"path/filepath"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/lipgloss"
)

// Highlighter provides syntax highlighting for code
type Highlighter struct {
	style *chroma.Style
}

// NewHighlighter creates a new syntax highlighter
func NewHighlighter() *Highlighter {
	return &Highlighter{
		style: styles.Get("catppuccin-mocha"),
	}
}

// HighlightLine highlights a single line of code based on file extension
func (h *Highlighter) HighlightLine(line, filename string) string {
	lexer := getLexerForFile(filename)
	if lexer == nil {
		return line
	}

	iterator, err := lexer.Tokenise(nil, line)
	if err != nil {
		return line
	}

	var result strings.Builder
	for token := iterator(); token != chroma.EOF; token = iterator() {
		style := h.style.Get(token.Type)
		text := token.Value

		if style.Colour.IsSet() {
			color := style.Colour.String()
			styled := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
			if style.Bold == chroma.Yes {
				styled = styled.Bold(true)
			}
			if style.Italic == chroma.Yes {
				styled = styled.Italic(true)
			}
			result.WriteString(styled.Render(text))
		} else {
			result.WriteString(text)
		}
	}

	return result.String()
}

// HighlightLines highlights multiple lines
func (h *Highlighter) HighlightLines(lines []string, filename string) []string {
	result := make([]string, len(lines))
	for i, line := range lines {
		result[i] = h.HighlightLine(line, filename)
	}
	return result
}

// getLexerForFile returns the appropriate lexer for a filename
func getLexerForFile(filename string) chroma.Lexer {
	// Try by filename first
	lexer := lexers.Match(filename)
	if lexer != nil {
		return lexer
	}

	// Try by extension
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".toml":
		return lexers.Get("toml")
	case ".yaml", ".yml":
		return lexers.Get("yaml")
	case ".json":
		return lexers.Get("json")
	case ".sh", ".bash", ".zsh":
		return lexers.Get("bash")
	case ".fish":
		return lexers.Get("fish")
	case ".lua":
		return lexers.Get("lua")
	case ".vim":
		return lexers.Get("vim")
	case ".py":
		return lexers.Get("python")
	case ".go":
		return lexers.Get("go")
	case ".js", ".jsx":
		return lexers.Get("javascript")
	case ".ts", ".tsx":
		return lexers.Get("typescript")
	case ".css":
		return lexers.Get("css")
	case ".html", ".htm":
		return lexers.Get("html")
	case ".md", ".markdown":
		return lexers.Get("markdown")
	case ".conf", ".cfg", ".ini":
		return lexers.Get("ini")
	case ".xml":
		return lexers.Get("xml")
	case ".rs":
		return lexers.Get("rust")
	case ".rb":
		return lexers.Get("ruby")
	case ".swift":
		return lexers.Get("swift")
	}

	// Check for common config files without extensions
	base := strings.ToLower(filepath.Base(filename))
	switch {
	case strings.HasPrefix(base, ".zshrc"), strings.HasPrefix(base, ".bashrc"):
		return lexers.Get("bash")
	case strings.HasPrefix(base, ".gitconfig"), strings.HasPrefix(base, ".gitignore"):
		return lexers.Get("ini")
	case base == "dockerfile":
		return lexers.Get("docker")
	case base == "makefile":
		return lexers.Get("makefile")
	}

	return nil
}

// GetFileType returns a human-readable file type for display
func GetFileType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".toml":
		return "TOML"
	case ".yaml", ".yml":
		return "YAML"
	case ".json":
		return "JSON"
	case ".sh", ".bash":
		return "Bash"
	case ".zsh":
		return "Zsh"
	case ".fish":
		return "Fish"
	case ".lua":
		return "Lua"
	case ".vim":
		return "Vim"
	case ".py":
		return "Python"
	case ".go":
		return "Go"
	case ".js":
		return "JavaScript"
	case ".ts":
		return "TypeScript"
	case ".css":
		return "CSS"
	case ".html", ".htm":
		return "HTML"
	case ".md", ".markdown":
		return "Markdown"
	case ".conf", ".cfg", ".ini":
		return "Config"
	case ".xml":
		return "XML"
	default:
		return "Text"
	}
}
