package main

import (
	"embed"
	"log"
	"os"
	"runtime"

	"github.com/mvkvl/2fa/gui/internal/desktop"
	"github.com/mvkvl/2fa/internal/core"
	"github.com/mvkvl/2fa/internal/service"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed assets/appicon.png
var appIcon []byte

//go:embed assets/tray-unlocked.png
var trayUnlockedIcon []byte

//go:embed assets/tray-locked.png
var trayLockedIcon []byte

var version = "dev"

func main() {
	path := os.Getenv("2FA_CONFIG")
	svc := service.New(path, version)

	status := svc.Status()
	if status.Exists && status.Provider == core.ProviderKeyring && core.KeyringAvailable() {
		if _, err := svc.Unlock(""); err != nil {
			log.Printf("2fa: не удалось разблокировать keyring vault: %v", err)
		}
		status = svc.Status()
	}

	showWindow := !status.Exists || status.Locked || status.Error != ""
	// На Linux статус-нотификаторы поддерживаются не во всех окружениях,
	// поэтому показываем окно, чтобы приложение оставалось доступным.
	showWindow = showWindow || runtime.GOOS == "linux"

	initialActivationPolicy := application.ActivationPolicyAccessory
	if showWindow {
		initialActivationPolicy = application.ActivationPolicyRegular
	}

	selectedWindow := &App{service: svc}

	wailsApp := application.New(application.Options{
		Name:        "2fa",
		Description: "TOTP-аутентификатор",
		Icon:        appIcon,
		Mac: application.MacOptions{
			ActivationPolicy: initialActivationPolicy,
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
		Linux: application.LinuxOptions{
			DisableQuitOnLastWindowClosed: true,
			ProgramName:                   "2fa",
			ApplicationID:                 "com.mvkvl.2fa",
		},
		Windows: application.WindowsOptions{
			DisableQuitOnLastWindowClosed: true,
		},
		Assets: application.AssetOptions{
			Handler: application.BundledAssetFileServer(assets),
		},
		Services: []application.Service{
			application.NewService(svc),
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "com.mvkvl.2fa",
			OnSecondInstanceLaunch: func(application.SecondInstanceData) {
				selectedWindow.showWindow()
				selectedWindow.wails.Event.Emit("vault-changed")
			},
		},
		OnShutdown: func() {
			svc.Lock()
		},
		ErrorHandler: func(err error) {
			log.Printf("2fa: %v", err)
		},
	})

	window := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            "2fa",
		Width:            660,
		Height:           780,
		MinWidth:         560,
		MinHeight:        600,
		DisableResize:    true,
		Hidden:           !showWindow,
		URL:              "/",
		BackgroundColour: application.NewRGB(15, 21, 28),
	})

	desk := desktop.New(wailsApp, svc, window, selectedWindow.rebuildTray)
	selectedWindow.wails = wailsApp
	selectedWindow.window = window
	selectedWindow.desktop = desk
	selectedWindow.trayUnlocked = trayUnlockedIcon
	selectedWindow.trayLocked = trayLockedIcon
	wailsApp.RegisterService(application.NewService(desk))

	window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		e.Cancel()
		window.Hide()
		setMacOSActivationPolicy(false)
	})

	selectedWindow.setupTray()
	if showWindow {
		selectedWindow.showWindow()
	}

	if err := wailsApp.Run(); err != nil {
		log.Fatal(err)
	}
}
