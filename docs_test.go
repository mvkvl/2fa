package main

import (
	"strings"
	"testing"
)

func TestDocsFormats(t *testing.T) {
	app := newApp()

	markdown, err := generateDocsMarkdown(app)
	if err != nil {
		t.Fatalf("generateDocsMarkdown: %v", err)
	}
	for _, want := range []string{"# NAME", "# SYNOPSIS", "# DESCRIPTION", "## Назначение", "## Режимы хранения", "```console"} {
		if !strings.Contains(markdown, want) {
			t.Errorf("markdown docs missing %q", want)
		}
	}
	if !strings.Contains(markdown, "Версия: `"+version+"`") {
		t.Errorf("markdown docs missing version %q", version)
	}
	if !strings.Contains(markdown, defaultVaultPathString()) {
		t.Errorf("markdown docs missing default config path %q", defaultVaultPathString())
	}

	text, err := generateDocsText(app)
	if err != nil {
		t.Fatalf("generateDocsText: %v", err)
	}
	for _, want := range []string{"NAME", "НАЗНАЧЕНИЕ", "РЕЖИМЫ ХРАНЕНИЯ", "КОДЫ ВОЗВРАТА"} {
		if !strings.Contains(text, want) {
			t.Errorf("text docs missing %q", want)
		}
	}
	if strings.Contains(text, "```") {
		t.Error("text docs still contain code fences")
	}

	man, err := generateDocsMan(app)
	if err != nil {
		t.Fatalf("generateDocsMan: %v", err)
	}
	for _, want := range []string{".TH 2fa 1", ".SH NAME", ".SH DESCRIPTION"} {
		if !strings.Contains(man, want) {
			t.Errorf("man docs missing %q", want)
		}
	}
}
