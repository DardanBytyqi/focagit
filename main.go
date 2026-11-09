package main

import (
	"embed"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"

	"github.com/dardanbytyqi/focagit/internal/credentials"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var icon []byte

func main() {
	// Git starts this same binary to ask for a password; that run never opens a window.
	if code, ok := credentials.Main(os.Args[1:]); ok {
		os.Exit(code)
	}

	logger := setupLogging()
	logger.Info("focagit starting")

	app := NewApp()

	err := wails.Run(&options.App{
		Title:     "focagit",
		Width:     1400,
		Height:    900,
		MinWidth:  900,
		MinHeight: 560,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		// Matches --bg in style.css so the window never flashes white before the frontend paints.
		BackgroundColour: &options.RGBA{R: 0x1e, G: 0x1f, B: 0x22, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind:             []any{app},
		Linux: &linux.Options{
			ProgramName: "focagit",
			Icon:        icon,
			// The webview renders the whole UI.
			WindowIsTranslucent: false,
		},
	})
	if err != nil {
		logger.Error("wails exited with an error", "err", err)
		os.Exit(1)
	}
}
