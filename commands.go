package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/urfave/cli/v3"
	"golang.org/x/term"
)

var (
	stdin      io.Reader = os.Stdin
	stdout     io.Writer = os.Stdout
	stderr     io.Writer = os.Stderr
	stdinIsTTY           = term.IsTerminal(int(os.Stdin.Fd()))
)

func newApp() *cli.Command {
	return &cli.Command{
		Name:        "2fa",
		Usage:       "TOTP-аутентификатор с сохранением секретов по именам аккаунтов",
		UsageText:   "2fa [глобальные флаги] <аккаунт|секрет>\n   2fa [глобальные флаги] <команда> [аргументы...]",
		ArgsUsage:   "<аккаунт|секрет>",
		Version:     version,
		Description: "Генерирует шестизначные TOTP-коды (RFC 6238, SHA-1, 30 секунд) и хранит секреты в зашифрованном локальном хранилище.",
		Flags:       globalFlags(),
		Commands: []*cli.Command{
			codeCommand(),
			listCommand(),
			addCommand(),
			removeCommand(),
			editCommand(),
			renameCommand(),
			rekeyCommand(),
			initCommand(),
			docsCommand(),
		},
		Action: rootAction,
	}
}

func globalFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Usage:   "путь к файлу хранилища",
			Value:   defaultVaultPathString(),
			Sources: cli.EnvVars("2FA_CONFIG"),
		},
		&cli.StringFlag{
			Name:    "key-provider",
			Aliases: []string{"k"},
			Usage:   "провайдер ключа: auto, keyring, passphrase, plaintext",
			Value:   providerAuto,
		},
		&cli.BoolFlag{
			Name:    "raw",
			Aliases: []string{"r"},
			Usage:   "интерпретировать аргумент как Base32-секрет, не обращаясь к хранилищу",
		},
		&cli.BoolFlag{
			Name:  "secret-stdin",
			Usage: "читать секрет из stdin (для скриптов)",
		},
		&cli.BoolFlag{
			Name:  "passphrase-stdin",
			Usage: "читать пароль vault из stdin (для скриптов)",
		},
	}
}

func codeCommand() *cli.Command {
	return &cli.Command{
		Name:      "code",
		Aliases:   []string{"c", "g"},
		Usage:     "сгенерировать код для аккаунта или секрета",
		ArgsUsage: "<аккаунт|секрет>",
		Action:    codeAction,
	}
}

func listCommand() *cli.Command {
	return &cli.Command{
		Name:    "list",
		Aliases: []string{"ls", "l"},
		Usage:   "показать список аккаунтов",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "ext",
				Aliases: []string{"e"},
				Usage:   "показать описания аккаунтов",
			},
		},
		Action: listAction,
	}
}

func addCommand() *cli.Command {
	return &cli.Command{
		Name:      "add",
		Aliases:   []string{"a"},
		Usage:     "добавить аккаунт",
		ArgsUsage: "<имя> [секрет]",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "desc",
				Aliases: []string{"d"},
				Usage:   "описание аккаунта",
			},
			&cli.BoolFlag{
				Name:    "force",
				Aliases: []string{"f"},
				Usage:   "перезаписать существующий аккаунт",
			},
		},
		Action: addAction,
	}
}

func removeCommand() *cli.Command {
	return &cli.Command{
		Name:      "remove",
		Aliases:   []string{"rm", "del"},
		Usage:     "удалить аккаунт",
		ArgsUsage: "<имя>",
		Action:    removeAction,
	}
}

func editCommand() *cli.Command {
	return &cli.Command{
		Name:      "edit",
		Aliases:   []string{"e"},
		Usage:     "изменить описание или секрет аккаунта",
		ArgsUsage: "<имя>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "desc",
				Aliases: []string{"d"},
				Usage:   "новое описание (пустая строка очищает)",
			},
			&cli.StringFlag{
				Name:    "secret",
				Aliases: []string{"s"},
				Usage:   "новый Base32-секрет (попадает в историю оболочки)",
			},
		},
		Action: editAction,
	}
}

func renameCommand() *cli.Command {
	return &cli.Command{
		Name:      "rename",
		Aliases:   []string{"mv"},
		Usage:     "переименовать аккаунт",
		ArgsUsage: "<старое имя> <новое имя>",
		Action:    renameAction,
	}
}

func rekeyCommand() *cli.Command {
	return &cli.Command{
		Name:    "rekey",
		Aliases: []string{"rk"},
		Usage:   "сменить провайдер ключа и перешифровать хранилище",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "to",
				Usage:    "целевой провайдер: keyring, passphrase или plaintext",
				Required: true,
			},
		},
		Action: rekeyAction,
	}
}

func initCommand() *cli.Command {
	return &cli.Command{
		Name:    "init",
		Aliases: []string{"i"},
		Usage:   "создать пустое хранилище",
		Action:  initAction,
	}
}

func docsCommand() *cli.Command {
	return &cli.Command{
		Name:    "docs",
		Aliases: []string{"doc", "manual"},
		Usage:   "вывести подробную документацию",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "format",
				Aliases: []string{"f"},
				Usage:   "формат: text, markdown (md), man",
				Value:   "text",
			},
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Usage:   "файл для записи (по умолчанию stdout, '-' означает stdout)",
			},
		},
		Action: docsAction,
	}
}

func providerOptions(cmd *cli.Command) ProviderOptions {
	return ProviderOptions{
		Stdin:           stdin,
		Stderr:          stderr,
		StdinIsTTY:      stdinIsTTY,
		PassphraseStdin: cmd.Bool("passphrase-stdin"),
	}
}

func configPath(cmd *cli.Command) (string, error) {
	path := cmd.String("config")
	if path == "" {
		return defaultVaultPath()
	}
	return expandHome(path)
}

func expandHome(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") && !strings.HasPrefix(path, `~\`) {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("не удалось раскрыть домашний каталог в пути %q: %w", path, err)
	}
	if path == "~" {
		return home, nil
	}
	return filepath.Join(home, path[2:]), nil
}

func loadVault(cmd *cli.Command) (*Vault, error) {
	path, err := configPath(cmd)
	if err != nil {
		return nil, err
	}
	return openVault(path, cmd.String("key-provider"), providerOptions(cmd))
}

func loadOrInitVault(cmd *cli.Command) (*Vault, error) {
	path, err := configPath(cmd)
	if err != nil {
		return nil, err
	}
	return openOrCreateVault(path, cmd.String("key-provider"), providerOptions(cmd))
}

// withVault opens the configured vault while holding a cross-process lock.
// Callers must use exclusive=true for any path that may write to the vault and
// keep the whole open-modify-save sequence inside fn.
func withVault(cmd *cli.Command, exclusive, create bool, fn func(*Vault) error) error {
	path, err := configPath(cmd)
	if err != nil {
		return err
	}
	return withVaultLock(path, exclusive, func() error {
		var vault *Vault
		if create {
			vault, err = loadOrInitVault(cmd)
		} else {
			vault, err = loadVault(cmd)
		}
		if err != nil {
			return err
		}
		return fn(vault)
	})
}

func rootAction(ctx context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() == 0 {
		return cli.ShowRootCommandHelp(cmd)
	}
	if cmd.Args().Len() != 1 {
		return errors.New("ожидается один аргумент: имя аккаунта или Base32-секрет")
	}
	return codeForArg(cmd, cmd.Args().Get(0), true)
}

func codeAction(ctx context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() != 1 {
		return errors.New("использование: 2fa code <аккаунт|секрет>")
	}
	return codeForArg(cmd, cmd.Args().Get(0), false)
}

func codeForArg(cmd *cli.Command, arg string, allowRawFallback bool) error {
	if strings.TrimSpace(arg) == "" {
		return errors.New("пустой аргумент")
	}
	if cmd.Bool("raw") {
		return printCode(arg)
	}

	var secret string
	var found bool
	err := withVault(cmd, false, false, func(vault *Vault) error {
		if account, ok := vault.Payload.Get(arg); ok {
			secret = account.Secret
			found = true
		}
		return nil
	})
	if err == nil {
		if found {
			return printCode(secret)
		}
		if allowRawFallback {
			if _, decodeErr := decodeSecret(arg); decodeErr == nil {
				return printCode(arg)
			}
		}
		return fmt.Errorf("аккаунт %q не найден; список аккаунтов: 2fa list (для сырого секрета используйте --raw)", arg)
	}

	if errors.Is(err, ErrVaultNotFound) {
		if allowRawFallback {
			if _, decodeErr := decodeSecret(arg); decodeErr == nil {
				return printCode(arg)
			}
		}
		return fmt.Errorf("аккаунт %q не найден, хранилище не инициализировано (2fa init); для сырого секрета используйте --raw", arg)
	}

	if allowRawFallback {
		if _, decodeErr := decodeSecret(arg); decodeErr == nil {
			fmt.Fprintf(stderr, "Предупреждение: не удалось открыть хранилище: %v; использую аргумент как сырой секрет\n", err)
			return printCode(arg)
		}
	}
	return err
}

func printCode(secret string) error {
	code, err := generateTOTP(secret)
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, code)
	return nil
}

func listAction(ctx context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() != 0 {
		return errors.New("list не принимает позиционных аргументов")
	}
	var infos []AccountInfo
	err := withVault(cmd, false, false, func(vault *Vault) error {
		infos = vault.Payload.List()
		return nil
	})
	if errors.Is(err, ErrVaultNotFound) {
		fmt.Fprintln(stderr, "Хранилище не инициализировано. Выполните: 2fa init")
		return nil
	}
	if err != nil {
		return err
	}

	if len(infos) == 0 {
		fmt.Fprintln(stderr, "Список аккаунтов пуст. Добавьте: 2fa add <имя>")
		return nil
	}

	if !cmd.Bool("ext") {
		for _, info := range infos {
			fmt.Fprintln(stdout, info.Name)
		}
		return nil
	}

	w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tDESCRIPTION")
	for _, info := range infos {
		description := strings.ReplaceAll(info.Description, "\n", " ")
		description = strings.ReplaceAll(description, "\t", " ")
		if description == "" {
			description = "-"
		}
		fmt.Fprintf(w, "%s\t%s\n", info.Name, description)
	}
	return w.Flush()
}

func addAction(ctx context.Context, cmd *cli.Command) error {
	args := cmd.Args()
	if args.Len() < 1 || args.Len() > 2 {
		return errors.New("использование: 2fa add <имя> [секрет]")
	}
	name := args.Get(0)
	if err := validateAccountName(name); err != nil {
		return err
	}

	var secret string
	if args.Len() == 2 {
		secret = args.Get(1)
	}
	if secret != "" && cmd.Bool("secret-stdin") {
		return errors.New("нельзя одновременно указать секрет аргументом и --secret-stdin")
	}
	if secret == "" {
		var err error
		secret, err = readSecretValue(providerOptions(cmd), "Секрет (Base32): ", stdinIsTTY, cmd.Bool("secret-stdin"))
		if err != nil {
			return err
		}
	}
	if strings.TrimSpace(secret) == "" {
		return errors.New("секрет не может быть пустым")
	}
	if _, err := normalizeAndValidateSecret(secret); err != nil {
		return fmt.Errorf("некорректный секрет: %w", err)
	}

	var description *string
	if cmd.IsSet("desc") {
		value := cmd.String("desc")
		description = &value
	}

	var overwritten bool
	err := withVault(cmd, true, true, func(vault *Vault) error {
		if vault.WasCreated() && vault.Header.Provider == providerPlaintext {
			fmt.Fprintln(stderr, "Предупреждение: секреты хранятся без шифрования.")
		}
		var addErr error
		overwritten, addErr = vault.Payload.Add(name, secret, description, cmd.Bool("force"))
		if addErr != nil {
			return addErr
		}
		return vault.Save()
	})
	if err != nil {
		return err
	}
	if overwritten {
		fmt.Fprintf(stdout, "Аккаунт %q обновлён.\n", name)
	} else {
		fmt.Fprintf(stdout, "Аккаунт %q добавлен.\n", name)
	}
	return nil
}

func removeAction(ctx context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() != 1 {
		return errors.New("использование: 2fa remove <имя>")
	}
	name := cmd.Args().Get(0)
	if err := withVault(cmd, true, false, func(vault *Vault) error {
		if err := vault.Payload.Remove(name); err != nil {
			return err
		}
		return vault.Save()
	}); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Аккаунт %q удалён.\n", name)
	return nil
}

func editAction(ctx context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() != 1 {
		return errors.New("использование: 2fa edit <имя> [--desc ...] [--secret ...]")
	}
	name := cmd.Args().Get(0)

	var description *string
	if cmd.IsSet("desc") {
		value := cmd.String("desc")
		description = &value
	}

	if cmd.IsSet("secret") && cmd.Bool("secret-stdin") {
		return errors.New("нельзя одновременно использовать --secret и --secret-stdin")
	}
	var secret *string
	if cmd.IsSet("secret") {
		value := cmd.String("secret")
		secret = &value
	}
	if cmd.Bool("secret-stdin") {
		value, err := readSecretValue(providerOptions(cmd), "Новый секрет (Base32): ", stdinIsTTY, true)
		if err != nil {
			return err
		}
		secret = &value
	}
	if description == nil && secret == nil {
		return errors.New("укажите хотя бы один параметр: --desc, --secret или --secret-stdin")
	}

	if err := withVault(cmd, true, false, func(vault *Vault) error {
		if err := vault.Payload.Update(name, secret, description); err != nil {
			return err
		}
		return vault.Save()
	}); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Аккаунт %q обновлён.\n", name)
	return nil
}

func renameAction(ctx context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() != 2 {
		return errors.New("использование: 2fa rename <старое имя> <новое имя>")
	}
	oldName := cmd.Args().Get(0)
	newName := cmd.Args().Get(1)
	if err := withVault(cmd, true, false, func(vault *Vault) error {
		if err := vault.Payload.Rename(oldName, newName); err != nil {
			return err
		}
		return vault.Save()
	}); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Аккаунт %q переименован в %q.\n", oldName, newName)
	return nil
}

func rekeyAction(ctx context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() != 0 {
		return errors.New("rekey не принимает позиционных аргументов")
	}
	target := cmd.String("to")
	if target == "" {
		return errors.New("укажите целевой провайдер: --to keyring|passphrase|plaintext")
	}
	var provider string
	if err := withVault(cmd, true, false, func(vault *Vault) error {
		if err := rekeyVault(vault, target, providerOptions(cmd)); err != nil {
			return err
		}
		provider = vault.Header.Provider
		return nil
	}); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Провайдер изменён на %q.\n", provider)
	if provider == providerPlaintext {
		fmt.Fprintln(stderr, "Предупреждение: секреты хранятся без шифрования.")
	}
	return nil
}

func initAction(ctx context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() != 0 {
		return errors.New("init не принимает позиционных аргументов")
	}
	path, err := configPath(cmd)
	if err != nil {
		return err
	}
	return withVaultLock(path, true, func() error {
		if _, statErr := os.Stat(path); statErr == nil {
			vault, err := openVault(path, cmd.String("key-provider"), providerOptions(cmd))
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, "Хранилище уже инициализировано.\nПуть: %s\nПровайдер: %s\nАккаунтов: %d\n",
				path, vault.Header.Provider, len(vault.Payload.Accounts))
			return nil
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return fmt.Errorf("не удалось проверить хранилище: %w", statErr)
		}

		vault, err := createVault(path, cmd.String("key-provider"), providerOptions(cmd))
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Хранилище создано.\nПуть: %s\nПровайдер: %s\nVault ID: %s\n",
			vault.Path, vault.Header.Provider, vault.Header.VaultID)
		if vault.Header.Provider == providerPlaintext {
			fmt.Fprintln(stderr, "Предупреждение: секреты хранятся без шифрования.")
		}
		return nil
	})
}

func docsAction(ctx context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() != 0 {
		return errors.New("docs не принимает позиционных аргументов")
	}
	format := strings.ToLower(cmd.String("format"))
	var (
		content string
		err     error
	)
	switch format {
	case "text":
		content, err = generateDocsText(cmd.Root())
	case "markdown", "md":
		content, err = generateDocsMarkdown(cmd.Root())
	case "man":
		content, err = generateDocsMan(cmd.Root())
	default:
		return fmt.Errorf("неизвестный формат %q (допустимо: text, markdown, md, man)", format)
	}
	if err != nil {
		return err
	}

	output := cmd.String("output")
	if output == "" || output == "-" {
		_, err = io.WriteString(stdout, content)
		return err
	}
	if dir := filepath.Dir(output); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("не удалось создать каталог: %w", err)
		}
	}
	if err := os.WriteFile(output, []byte(content), 0644); err != nil {
		return fmt.Errorf("не удалось записать документацию: %w", err)
	}
	fmt.Fprintf(stdout, "Документация записана в %s\n", output)
	return nil
}
