package main

import (
	"embed"
	"fmt"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/src
var assets embed.FS

func main() {
	// Create an instance of the app structure
	app := NewApp()

	appMenu := buildMenu(app)

	// Create application with options
	err := wails.Run(&options.App{
		Title:  "vibe-tree-desktop",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		Menu:             appMenu,
		OnStartup:        app.startup,
		OnDomReady:       app.domReady,
		OnBeforeClose:    app.beforeClose,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

func buildMenu(app *App) *menu.Menu {
	m := menu.NewMenu()
	m.Append(menu.AppMenu())
	m.Append(menu.EditMenu())

	tools := m.AddSubmenu("Tools")
	tools.AddText("打开数据目录", nil, func(_ *menu.CallbackData) {
		if path, err := app.OpenDataDir(); err != nil {
			fmt.Printf("open data dir failed: path=%s err=%v\n", path, err)
		}
	})

	m.Append(menu.WindowMenu())
	return m
}
