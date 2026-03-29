package main

import (
	_ "embed"
	"sync"

	"github.com/getlantern/systray"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed assets/tray/tray_gray.ico
var trayIconGray []byte

//go:embed assets/tray/tray_green.ico
var trayIconGreen []byte

type trayController struct {
	app          *App
	itemConnect  *systray.MenuItem
	itemOpen     *systray.MenuItem
	itemExit     *systray.MenuItem
	ready        chan struct{}
	shutdownOnce sync.Once
}

func (a *App) initTray() {
	if a.tray != nil {
		return
	}
	tc := &trayController{
		app:   a,
		ready: make(chan struct{}),
	}
	a.tray = tc

	go systray.Run(tc.onReady, tc.onExit)
	<-tc.ready
}

func (a *App) refreshTray() {
	if a.tray == nil {
		return
	}
	a.tray.update(a.connected, a.uiLang)
}

func (t *trayController) onReady() {
	systray.SetIcon(trayIconGray)
	systray.SetTitle("Furor Davidis")
	systray.SetTooltip("Furor Davidis")

	t.itemConnect = systray.AddMenuItem("Libertad", "Connect/Disconnect")
	t.itemOpen = systray.AddMenuItem("Open", "Show window")
	t.itemExit = systray.AddMenuItem("Exit", "Exit app")

	close(t.ready)
	t.update(t.app.connected, t.app.uiLang)

	go t.loop()
}

func (t *trayController) onExit() {}

func (t *trayController) loop() {
	for {
		select {
		case <-t.itemConnect.ClickedCh:
			if t.app.connected {
				t.app.DisconnectAWG()
				continue
			}
			if !t.app.running {
				if err := t.app.Start(); err != nil {
					t.app.log.Errorf("tray start failed: %v", err)
					continue
				}
			}
			if err := t.app.ConnectAWG(); err != nil {
				t.app.log.Errorf("tray connect failed: %v", err)
			}
		case <-t.itemOpen.ClickedCh:
			runtime.WindowUnminimise(t.app.ctx)
			runtime.WindowShow(t.app.ctx)
		case <-t.itemExit.ClickedCh:
			t.app.quitFromTray()
			return
		}
	}
}

func (t *trayController) update(connected bool, lang string) {
	if t.itemConnect == nil {
		return
	}
	if lang == "ru" {
		t.itemOpen.SetTitle("Открыть")
		t.itemExit.SetTitle("Выход")
	} else {
		t.itemOpen.SetTitle("Open")
		t.itemExit.SetTitle("Exit")
	}
	if connected {
		t.itemConnect.SetTitle("Libero")
		systray.SetIcon(trayIconGreen)
		return
	}
	t.itemConnect.SetTitle("Libertad")
	systray.SetIcon(trayIconGray)
}

func (t *trayController) shutdown() {
	t.shutdownOnce.Do(func() {
		systray.Quit()
	})
}
