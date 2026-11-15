package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

// директива компилятора go:embed вшивает папку с фронтендом прямо внутрь .exe файла
//
//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:         "winwid", // по этому имени мы найдем окно через Win32 API
		Width:         580,
		Height:        340,
		Frameless:     true, // убирает стандартную рамку винды и кнопки закрыть, свернуть
		DisableResize: true, // фиксированный рамер виджета
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		// RGBA c нулевым альфа каналом делает подложку окна прозрачной
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 0},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app, // прокидываем методы структуры App в JS
		},
		Windows: &windows.Options{
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			BackdropType:         windows.None,
		},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "winwid-minimal-apple-widget-id",
		},
	})

	if err != nil {
		log.Fatal(err)
	}
}
