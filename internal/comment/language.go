package comment

// CommentSyntax describes how to wrap an llmdoc block for a given language.
// When BlockOpen is non-empty, block-comment form is used (e.g. /* ... */).
// When BlockOpen is empty, each line is prefixed with LinePrefix.
type CommentSyntax struct {
	BlockOpen  string // e.g. "/*"
	BlockClose string // e.g. "*/"
	LinePrefix string // e.g. "// " or "# "
	Language   string // human-readable name
}

func blockComment(lang string) CommentSyntax {
	return CommentSyntax{BlockOpen: "/*", BlockClose: "*/", Language: lang}
}

func lineComment(prefix, lang string) CommentSyntax {
	return CommentSyntax{LinePrefix: prefix, Language: lang}
}

func htmlComment(lang string) CommentSyntax {
	return CommentSyntax{BlockOpen: "<!--", BlockClose: "-->", Language: lang}
}

var extensionMap = map[string]CommentSyntax{
	// C-family block comments
	".go":     blockComment("Go"),
	".c":      blockComment("C"),
	".h":      blockComment("C"),
	".cpp":    blockComment("C++"),
	".hpp":    blockComment("C++"),
	".cc":     blockComment("C++"),
	".cs":     blockComment("C#"),
	".java":   blockComment("Java"),
	".kt":     blockComment("Kotlin"),
	".kts":    blockComment("Kotlin"),
	".rs":     blockComment("Rust"),
	".js":     blockComment("JavaScript"),
	".jsx":    blockComment("JavaScript"),
	".ts":     blockComment("TypeScript"),
	".tsx":    blockComment("TypeScript"),
	".mjs":    blockComment("JavaScript"),
	".cjs":    blockComment("JavaScript"),
	".swift":  blockComment("Swift"),
	".php":    blockComment("PHP"),
	".scala":  blockComment("Scala"),
	".groovy": blockComment("Groovy"),
	".dart":   blockComment("Dart"),

	// Hash line comments
	".py":   lineComment("# ", "Python"),
	".rb":   lineComment("# ", "Ruby"),
	".sh":   lineComment("# ", "Shell"),
	".bash": lineComment("# ", "Bash"),
	".zsh":  lineComment("# ", "Zsh"),
	".fish": lineComment("# ", "Fish"),
	".yaml": lineComment("# ", "YAML"),
	".yml":  lineComment("# ", "YAML"),
	".toml": lineComment("# ", "TOML"),
	".conf": lineComment("# ", "Config"),
	".ini":  lineComment("# ", "INI"),
	".r":    lineComment("# ", "R"),
	".R":    lineComment("# ", "R"),
	".pl":   lineComment("# ", "Perl"),
	".pm":   lineComment("# ", "Perl"),
	".ex":   lineComment("# ", "Elixir"),
	".exs":  lineComment("# ", "Elixir"),
	".cr":   lineComment("# ", "Crystal"),

	// Double-dash line comments
	".sql": lineComment("-- ", "SQL"),
	".lua": lineComment("-- ", "Lua"),
	".hs":  lineComment("-- ", "Haskell"),
	".elm": lineComment("-- ", "Elm"),
	".ads": lineComment("-- ", "Ada"),
	".adb": lineComment("-- ", "Ada"),

	// HTML/XML block comments
	".html":   htmlComment("HTML"),
	".htm":    htmlComment("HTML"),
	".xml":    htmlComment("XML"),
	".svg":    htmlComment("SVG"),
	".vue":    htmlComment("Vue"),
	".svelte": htmlComment("Svelte"),
}

// ForExtension returns the CommentSyntax for a file extension (including the dot).
// Returns (zero value, false) if the extension is not supported.
func ForExtension(ext string) (CommentSyntax, bool) {
	s, ok := extensionMap[ext]
	return s, ok
}
