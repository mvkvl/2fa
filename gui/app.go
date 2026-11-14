package main

import (
	"fmt"
	"runtime"
	"time"

	"github.com/mvkvl/2fa/gui/internal/desktop"
	"github.com/mvkvl/2fa/internal/core"
	"github.com/mvkvl/2fa/internal/service"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type App struct {
	wails        *application.App
	window       *application.WebviewWindow
	service      *service.Service
	desktop      *desktop.Desktop
	tray         *application.SystemTray
	trayUnlocked []byte
	trayLocked   []byte
}

func (a *App) setupTray() {
	a.tray = a.wails.SystemTray.New()
	a.rebuildTray()
}

func (a *App) showWindow() {
	if a.window == nil {
		return
	}
	setMacOSActivationPolicy(true)
	a.window.Show()
	a.window.Focus()
}

func (a *App) updateTrayIcon(status service.Status) {
	if a.tray == nil {
		return
	}
	icon := a.trayUnlocked
	tooltip := "2fa"
	if !status.Exists || status.Locked {
		icon = a.trayLocked
		tooltip = "2fa — заблокирован"
	} else {
		tooltip = fmt.Sprintf("2fa — аккаунтов: %d", status.AccountCount)
	}
	if runtime.GOOS == "darwin" {
		a.tray.SetTemplateIcon(icon)
	} else {
		a.tray.SetIcon(icon)
	}
	a.tray.SetTooltip(tooltip)
}

func (a *App) rebuildTray() {
	if a.wails == nil || a.tray == nil {
		return
	}
	status := a.service.Status()
	menu := a.wails.NewMenu()

	if !status.Exists {
		menu.Add("Создать хранилище…").OnClick(func(*application.Context) {
			a.showWindow()
		})
	} else if status.Locked {
		menu.Add("Разблокировать…").OnClick(func(*application.Context) {
			if status.Provider == core.ProviderKeyring && core.KeyringAvailable() {
				if _, err := a.service.Unlock(""); err == nil {
					a.rebuildTray()
					a.wails.Event.Emit("vault-changed")
					return
				}
			}
			a.showWindow()
		})
	} else {
		favorites, err := a.service.Favorites()
		if err == nil {
			for _, account := range favorites {
				name := account.Name
				menu.Add("★ " + name).OnClick(func(*application.Context) {
					a.copyFavorite(name)
				})
			}
		}
		menu.AddSeparator()
		menu.Add("Заблокировать").OnClick(func(*application.Context) {
			a.service.Lock()
			a.rebuildTray()
			a.wails.Event.Emit("vault-changed")
		})
	}

	menu.AddSeparator()
	menu.Add("Открыть").OnClick(func(*application.Context) {
		a.showWindow()
	})
	menu.Add("Выход").OnClick(func(*application.Context) {
		a.wails.Quit()
	})

	a.tray.SetMenu(menu)
	a.updateTrayIcon(status)
}

func (a *App) copyFavorite(name string) {
	if a.desktop == nil {
		return
	}
	if _, err := a.desktop.CopyCode(name); err != nil {
		a.flashTray("!")
		return
	}
	a.flashTray("✓")
}

func (a *App) flashTray(text string) {
	if a.tray == nil {
		return
	}
	a.tray.SetLabel(text)
	time.AfterFunc(1500*time.Millisecond, func() {
		a.tray.SetLabel("")
	})
}
