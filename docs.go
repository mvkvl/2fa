package main

import (
	_ "embed"
	"regexp"
	"strings"

	docs "github.com/urfave/cli-docs/v3"
	"github.com/urfave/cli/v3"
)

//go:embed docs/manual.md
var manualTemplate string

func manualMarkdown() string {
	text := manualTemplate
	text = strings.ReplaceAll(text, "{{VERSION}}", version)
	text = strings.ReplaceAll(text, "{{CONFIG_PATH}}", defaultVaultPathString())
	return text
}

func docsRoot(root *cli.Command) *cli.Command {
	clone := *root
	clone.Description = manualMarkdown()
	return &clone
}

func generateDocsMarkdown(root *cli.Command) (string, error) {
	return docs.ToMarkdown(docsRoot(root))
}

func generateDocsMan(root *cli.Command) (string, error) {
	return docs.ToManWithSection(docsRoot(root), 1)
}

func generateDocsText(root *cli.Command) (string, error) {
	markdown, err := generateDocsMarkdown(root)
	if err != nil {
		return "", err
	}
	return markdownToText(markdown), nil
}

var markdownLinkRE = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

func markdownToText(markdown string) string {
	var b strings.Builder
	inCode := false
	for _, line := range strings.Split(markdown, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCode = !inCode
			continue
		}
		if inCode {
			b.WriteString("    ")
			b.WriteString(line)
			b.WriteByte('\n')
			continue
		}
		if trimmed == "" {
			b.WriteByte('\n')
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			title := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			b.WriteString(strings.ToUpper(title))
			b.WriteByte('\n')
			b.WriteString(strings.Repeat("=", len([]rune(title))))
			b.WriteString("\n\n")
			continue
		}
		line = strings.ReplaceAll(line, "**", "")
		line = strings.ReplaceAll(line, "__", "")
		line = strings.ReplaceAll(line, "`", "")
		line = markdownLinkRE.ReplaceAllString(line, "$1 ($2)")
		b.WriteString(strings.TrimRight(line, " "))
		b.WriteByte('\n')
	}
	result := b.String()
	for strings.Contains(result, "\n\n\n") {
		result = strings.ReplaceAll(result, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(result) + "\n"
}
