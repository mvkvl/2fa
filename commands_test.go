package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var sixDigitsRE = regexp.MustCompile(`^\d{6}\n$`)

func runCLI(t *testing.T, input string, args ...string) (string, string, error) {
	t.Helper()
	oldStdin, oldStdout, oldStderr, oldTTY := stdin, stdout, stderr, stdinIsTTY
	defer func() {
		stdin, stdout, stderr, stdinIsTTY = oldStdin, oldStdout, oldStderr, oldTTY
	}()

	var out, errOut bytes.Buffer
	stdin = strings.NewReader(input)
	stdout = &out
	stderr = &errOut
	stdinIsTTY = false

	err := newApp().Run(context.Background(), append([]string{"2fa"}, args...))
	return out.String(), errOut.String(), err
}

func runWithConfig(t *testing.T, path, input string, args ...string) (string, string, error) {
	t.Helper()
	return runCLI(t, input, append(args, "--config", path)...)
}

func TestCLIAccountLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "accounts.json")
	run := func(input string, args ...string) (string, string, error) {
		return runWithConfig(t, path, input, args...)
	}

	out, _, err := run("", "init", "--key-provider", "plaintext")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if !strings.Contains(out, "Хранилище создано") {
		t.Errorf("init output = %q", out)
	}

	out, _, err = run("", "add", "github", testSecret, "--desc", "GitHub")
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if !strings.Contains(out, "добавлен") {
		t.Errorf("add output = %q", out)
	}

	out, _, err = run("", "list")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if out != "github\n" {
		t.Errorf("list output = %q, want github", out)
	}

	if _, _, err := run("", "list", "extra"); err == nil {
		t.Error("list accepted a positional argument")
	}

	out, _, err = run("", "list", "--ext")
	if err != nil {
		t.Fatalf("list --ext: %v", err)
	}
	if !strings.Contains(out, "NAME") || !strings.Contains(out, "GitHub") {
		t.Errorf("list --ext output = %q", out)
	}

	out, _, err = run("", "code", "github")
	if err != nil {
		t.Fatalf("code: %v", err)
	}
	if !sixDigitsRE.MatchString(out) {
		t.Errorf("code output = %q, want six digits", out)
	}

	if _, _, err := run("", "edit", "github", "--desc", "Personal"); err != nil {
		t.Fatalf("edit desc: %v", err)
	}
	out, _, err = run("", "list", "--ext")
	if err != nil {
		t.Fatalf("list --ext after edit: %v", err)
	}
	if !strings.Contains(out, "Personal") {
		t.Errorf("edited description missing: %q", out)
	}

	if _, _, err := run("", "edit", "github", "--desc="); err != nil {
		t.Fatalf("edit clear description: %v", err)
	}
	out, _, err = run("", "list", "--ext")
	if err != nil {
		t.Fatalf("list --ext after clearing description: %v", err)
	}
	if strings.Contains(out, "Personal") || !strings.Contains(out, "github") || !strings.Contains(out, "-") {
		t.Errorf("description was not cleared: %q", out)
	}

	const otherSecret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	if _, _, err := run("", "edit", "github", "--secret", otherSecret); err != nil {
		t.Fatalf("edit secret: %v", err)
	}
	out, _, err = run("", "code", "github")
	if err != nil {
		t.Fatalf("code after edit secret: %v", err)
	}
	if !sixDigitsRE.MatchString(out) {
		t.Errorf("code after edit secret = %q", out)
	}

	if _, _, err := run("", "rename", "github", "work"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	out, _, err = run("", "code", "work")
	if err != nil {
		t.Fatalf("code after rename: %v", err)
	}
	if !sixDigitsRE.MatchString(out) {
		t.Errorf("code after rename = %q", out)
	}

	if _, _, err := run("", "remove", "work"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	out, errOut, err := run("", "list")
	if err != nil {
		t.Fatalf("list after remove: %v", err)
	}
	if out != "" || !strings.Contains(errOut, "пуст") {
		t.Errorf("list after remove: out=%q err=%q", out, errOut)
	}

	out, _, err = run("", "init")
	if err != nil {
		t.Fatalf("init idempotent: %v", err)
	}
	if !strings.Contains(out, "уже инициализировано") {
		t.Errorf("init idempotent output = %q", out)
	}

	if _, _, err := run("", "init", "--key-provider", "passphrase"); err == nil {
		t.Fatal("init with different provider succeeded")
	} else if !strings.Contains(err.Error(), "rekey") {
		t.Errorf("init different provider error = %v, want rekey hint", err)
	}
}

func TestCLIAddDuplicateAndForce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "accounts.json")
	run := func(input string, args ...string) (string, string, error) {
		return runWithConfig(t, path, input, args...)
	}

	if _, _, err := run("", "init", "-k", "plaintext"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, _, err := run("", "add", "dup", testSecret, "-d", "one"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := run("", "add", "dup", testSecret, "-d", "two"); err == nil {
		t.Fatal("duplicate add succeeded without --force")
	}
	if _, _, err := run("", "add", "dup", testSecret, "--force"); err != nil {
		t.Fatalf("add --force: %v", err)
	}
	out, _, err := run("", "list", "--ext")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "one") {
		t.Errorf("force without --desc did not preserve description: %q", out)
	}
	if _, _, err := run("", "rename", "dup", "dup"); err == nil {
		t.Fatal("rename to same name succeeded")
	}
	if _, _, err := run("", "remove", "missing"); err == nil {
		t.Fatal("remove missing succeeded")
	}
	if _, _, err := run("", "edit", "dup"); err == nil {
		t.Fatal("edit without parameters succeeded")
	}
	if _, _, err := run(testSecret+"\n", "edit", "dup", "--secret", testSecret, "--secret-stdin"); err == nil {
		t.Fatal("edit accepted --secret and --secret-stdin together")
	}
	if _, _, err := run(testSecret+"\n", "add", "dup2", testSecret, "--secret-stdin"); err == nil {
		t.Fatal("add accepted a secret argument and --secret-stdin together")
	}
	if _, _, err := run("", "add", "bad name", testSecret); err == nil {
		t.Fatal("add accepted an invalid account name")
	}
	if _, _, err := run("", "add", "badsecret", "not-base32!"); err == nil {
		t.Fatal("add accepted an invalid secret")
	}
	if _, _, err := run("", "add", "other", testSecret); err != nil {
		t.Fatalf("add other: %v", err)
	}
	if _, _, err := run("", "rename", "dup", "other"); err == nil {
		t.Fatal("rename to existing account succeeded")
	}
}

func TestCLIAliasesAndSecretStdin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "accounts.json")
	run := func(input string, args ...string) (string, string, error) {
		return runWithConfig(t, path, input, args...)
	}

	if _, _, err := run("", "i", "-k", "plaintext"); err != nil {
		t.Fatalf("init alias: %v", err)
	}
	if _, _, err := run(testSecret+"\n", "a", "piped", "--secret-stdin"); err != nil {
		t.Fatalf("add alias with --secret-stdin: %v", err)
	}
	out, _, err := run("", "l")
	if err != nil {
		t.Fatalf("list alias: %v", err)
	}
	if !strings.Contains(out, "piped") {
		t.Errorf("list alias output = %q", out)
	}
	out, _, err = run("", "c", "piped")
	if err != nil {
		t.Fatalf("code alias: %v", err)
	}
	if !sixDigitsRE.MatchString(out) {
		t.Errorf("code alias output = %q", out)
	}
	if _, _, err := run("", "e", "piped", "-d", "Piped"); err != nil {
		t.Fatalf("edit alias: %v", err)
	}
	if _, _, err := run("", "mv", "piped", "renamed"); err != nil {
		t.Fatalf("rename alias: %v", err)
	}
	out, _, err = run("", "g", "renamed")
	if err != nil {
		t.Fatalf("code alias g: %v", err)
	}
	if !sixDigitsRE.MatchString(out) {
		t.Errorf("code alias g output = %q", out)
	}
	if _, _, err := run("", "rm", "renamed"); err != nil {
		t.Fatalf("remove alias: %v", err)
	}
}

func TestCLIAddPromptsFromStdin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "accounts.json")
	if _, _, err := runWithConfig(t, path, "", "init", "-k", "plaintext"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, _, err := runWithConfig(t, path, testSecret+"\n", "add", "frompipe"); err != nil {
		t.Fatalf("add from stdin: %v", err)
	}
	out, _, err := runWithConfig(t, path, "", "list")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "frompipe") {
		t.Errorf("list output = %q", out)
	}
	if _, _, err := runWithConfig(t, path, "", "add", "empty"); err == nil {
		t.Fatal("add with empty stdin succeeded")
	}
}

func TestCLIRootResolution(t *testing.T) {
	path := filepath.Join(t.TempDir(), "accounts.json")

	out, _, err := runCLI(t, "", testSecret, "--config", path)
	if err != nil {
		t.Fatalf("raw secret without vault: %v", err)
	}
	if !sixDigitsRE.MatchString(out) {
		t.Errorf("raw secret output = %q", out)
	}

	if _, _, err := runCLI(t, "", "init", "-k", "plaintext", "--config", path); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, _, err := runCLI(t, "", "add", "github", testSecret, "--config", path); err != nil {
		t.Fatalf("add: %v", err)
	}

	out, _, err = runCLI(t, "", "github", "--config", path)
	if err != nil {
		t.Fatalf("root account: %v", err)
	}
	if !sixDigitsRE.MatchString(out) {
		t.Errorf("root account output = %q", out)
	}

	out, _, err = runCLI(t, "", testSecret, "--config", path)
	if err != nil {
		t.Fatalf("root raw fallback: %v", err)
	}
	if !sixDigitsRE.MatchString(out) {
		t.Errorf("root raw fallback output = %q", out)
	}

	if _, _, err := runCLI(t, "", "missing!", "--config", path); err == nil {
		t.Fatal("missing account succeeded")
	}
	if _, _, err := runCLI(t, "", "code", testSecret, "--config", path); err == nil {
		t.Fatal("code without --raw used raw secret")
	}
	out, _, err = runCLI(t, "", "code", "--raw", testSecret, "--config", path)
	if err != nil {
		t.Fatalf("code --raw: %v", err)
	}
	if !sixDigitsRE.MatchString(out) {
		t.Errorf("code --raw output = %q", out)
	}
}

func TestCLIListMissingVault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")
	out, errOut, err := runCLI(t, "", "list", "--config", path)
	if err != nil {
		t.Fatalf("list missing vault: %v", err)
	}
	if out != "" {
		t.Errorf("list missing vault stdout = %q", out)
	}
	if !strings.Contains(errOut, "не инициализировано") {
		t.Errorf("list missing vault stderr = %q", errOut)
	}
}

func TestCLIDocs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")

	out, _, err := runCLI(t, "", "docs", "--format", "markdown", "--config", path)
	if err != nil {
		t.Fatalf("docs markdown: %v", err)
	}
	if !strings.Contains(out, "# NAME") || !strings.Contains(out, "## Назначение") {
		t.Errorf("docs markdown output = %q", out)
	}

	out, _, err = runCLI(t, "", "docs", "--config", path)
	if err != nil {
		t.Fatalf("docs text: %v", err)
	}
	if !strings.Contains(out, "NAME") || strings.Contains(out, "```") {
		t.Errorf("docs text output = %q", out)
	}

	out, _, err = runCLI(t, "", "docs", "-f", "man", "--config", path)
	if err != nil {
		t.Fatalf("docs man: %v", err)
	}
	if !strings.Contains(out, ".TH 2fa 1") {
		t.Errorf("docs man output = %q", out)
	}

	outputPath := filepath.Join(t.TempDir(), "nested", "2fa.md")
	out, _, err = runCLI(t, "", "docs", "-f", "markdown", "-o", outputPath, "--config", path)
	if err != nil {
		t.Fatalf("docs output file: %v", err)
	}
	if !strings.Contains(out, "записана") {
		t.Errorf("docs output message = %q", out)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read docs output: %v", err)
	}
	if !strings.Contains(string(data), "# NAME") {
		t.Errorf("docs output file does not contain markdown")
	}

	out, _, err = runCLI(t, "", "docs", "-f", "text", "-o", "-", "--config", path)
	if err != nil {
		t.Fatalf("docs output dash: %v", err)
	}
	if !strings.Contains(out, "NAME") {
		t.Errorf("docs output dash = %q", out)
	}
}

func TestCLIMultipleStdinLines(t *testing.T) {
	t.Setenv("2FA_PASSPHRASE", "")
	path := filepath.Join(t.TempDir(), "accounts.json")
	const passphrase = "secret-passphrase"
	const secret = "JBSWY3DPEHPK3PXP"

	if _, _, err := runWithConfig(t, path, passphrase+"\n", "init", "--key-provider", "passphrase", "--passphrase-stdin"); err != nil {
		t.Fatalf("init passphrase: %v", err)
	}

	if _, _, err := runWithConfig(t, path, secret+"\n"+passphrase+"\n", "add", "github", "--secret-stdin", "--passphrase-stdin"); err != nil {
		t.Fatalf("add with piped secret and passphrase: %v", err)
	}

	out, _, err := runWithConfig(t, path, passphrase+"\n", "list", "--passphrase-stdin")
	if err != nil {
		t.Fatalf("list with piped passphrase: %v", err)
	}
	if !strings.Contains(out, "github") {
		t.Errorf("list output = %q, want github", out)
	}

	out, _, err = runWithConfig(t, path, passphrase+"\n", "code", "github", "--passphrase-stdin")
	if err != nil {
		t.Fatalf("code with piped passphrase: %v", err)
	}
	if !sixDigitsRE.MatchString(out) {
		t.Errorf("code output = %q, want six digits", out)
	}
}

func TestCLIAddCreatesPlaintextVaultWithWarning(t *testing.T) {
	path := filepath.Join(t.TempDir(), "accounts.json")

	out, errOut, err := runWithConfig(t, path, "", "add", "github", testSecret, "--key-provider", "plaintext")
	if err != nil {
		t.Fatalf("add with lazy plaintext vault: %v", err)
	}
	if !strings.Contains(out, "добавлен") {
		t.Errorf("add output = %q", out)
	}
	if !strings.Contains(errOut, "без шифрования") {
		t.Errorf("add stderr = %q, want plaintext warning", errOut)
	}

	out, _, err = runWithConfig(t, path, "", "list")
	if err != nil {
		t.Fatalf("list after lazy init: %v", err)
	}
	if !strings.Contains(out, "github") {
		t.Errorf("list output = %q, want github", out)
	}
}

func TestExpandHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	for _, input := range []string{"~/vault.json", `~\vault.json`} {
		got, err := expandHome(input)
		if err != nil {
			t.Fatalf("expandHome(%q): %v", input, err)
		}
		if want := filepath.Join(home, "vault.json"); got != want {
			t.Errorf("expandHome(%q) = %q, want %q", input, got, want)
		}
	}

	got, err := expandHome("relative/vault.json")
	if err != nil {
		t.Fatalf("expandHome relative: %v", err)
	}
	if got != "relative/vault.json" {
		t.Errorf("expandHome relative = %q", got)
	}
}
