// Package desktop exposes native file dialogs and clipboard operations to the GUI.
package desktop

import (
	"errors"

	"github.com/mvkvl/2fa/internal/service"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type ImportSelection struct {
	Path    string                `json:"path"`
	Preview service.ImportPreview `json:"preview"`
}

type Desktop struct {
	app      *application.App
	window   application.Window
	svc      *service.Service
	onChange func()
}

func New(app *application.App, svc *service.Service, window application.Window, onChange func()) *Desktop {
	return &Desktop{app: app, svc: svc, window: window, onChange: onChange}
}

// RefreshTray is called by the frontend after state-changing operations.
func (d *Desktop) RefreshTray() {
	if d.onChange != nil {
		d.onChange()
	}
}

// CloseWindow hides the main window, matching the native close button behavior.
func (d *Desktop) CloseWindow() {
	if d.window != nil {
		d.window.Close()
	}
}

func (d *Desktop) CopyCode(name string) (service.CopyResult, error) {
	if d.app == nil {
		return service.CopyResult{}, errors.New("приложение ещё не готово")
	}
	result, err := d.svc.CopyCode(name)
	if err != nil {
		return result, err
	}
	d.app.Clipboard.SetText(result.Code)
	return result, nil
}

func (d *Desktop) SaveExport(passphrase string) (string, error) {
	if d.app == nil {
		return "", errors.New("приложение ещё не готово")
	}
	dialog := d.app.Dialog.SaveFile().
		SetMessage("Экспорт 2FA").
		SetFilename("2fa-backup.2fa").
		AddFilter("2FA backup", "*.2fa")
	if d.window != nil {
		dialog.AttachToWindow(d.window)
	}
	path, err := dialog.PromptForSingleSelection()
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", nil
	}
	if err := d.svc.Export(path, passphrase); err != nil {
		return "", err
	}
	return path, nil
}

func (d *Desktop) ChooseImport(passphrase string) (ImportSelection, error) {
	if d.app == nil {
		return ImportSelection{}, errors.New("приложение ещё не готово")
	}
	dialog := d.app.Dialog.OpenFile().
		SetTitle("Импорт 2FA").
		AddFilter("2FA backup", "*.2fa")
	if d.window != nil {
		dialog.AttachToWindow(d.window)
	}
	path, err := dialog.PromptForSingleSelection()
	if err != nil {
		return ImportSelection{}, err
	}
	if path == "" {
		return ImportSelection{}, nil
	}
	preview, err := d.svc.PreviewImport(path, passphrase)
	if err != nil {
		return ImportSelection{}, err
	}
	return ImportSelection{Path: path, Preview: preview}, nil
}

func (d *Desktop) ApplyImport(path, passphrase string, overwrite bool) (service.ImportStats, error) {
	if path == "" {
		return service.ImportStats{}, errors.New("файл импорта не выбран")
	}
	return d.svc.Import(path, passphrase, overwrite)
}
