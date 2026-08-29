package tray

import (
	"context"
	"fmt"
	"image/color"
	"sync"
	"time"

	"github.com/gogpu/systray"
	"github.com/maazrashid/AgentsUsage/internal/parser"
	"github.com/maazrashid/AgentsUsage/internal/service"
)

type ServerController interface {
	Start() error
	StopWithTimeout() error
	Status() service.Status
}

type UsageSource interface {
	Snapshot() parser.Stats
	Refresh(context.Context) error
}

type Options struct {
	DashboardURL string
	ConfigPath   string
	Quit         func()
}

func Run(ctx context.Context, options Options, controller ServerController, usage UsageSource) error {
	native := systray.New()
	menu := systray.NewMenu()
	statusItem := menu.Add("Server: Starting", func() {})
	statusItem.SetDisabled(true)
	usageItem := menu.Add("Today: loading usage…", func() {})
	usageItem.SetDisabled(true)
	menu.AddSeparator()
	menu.Add("Open Dashboard", func() {
		if err := OpenURL(options.DashboardURL); err != nil {
			native.ShowNotification("AgentsUsage", "Could not open the dashboard.")
		}
	})
	startStopItem := menu.Add("Stop Server", func() {
		go toggleServer(native, controller)
	})
	menu.Add("Refresh Usage", func() {
		go func() {
			refreshCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()
			if err := usage.Refresh(refreshCtx); err != nil {
				native.ShowNotification("AgentsUsage", "Usage refresh completed with a warning.")
			}
		}()
	})
	menu.Add("Open Settings", func() {
		if err := OpenPath(options.ConfigPath); err != nil {
			native.ShowNotification("AgentsUsage", "Could not open config.json.")
		}
	})
	menu.AddSeparator()

	var removeOnce sync.Once
	remove := func() { removeOnce.Do(native.Remove) }
	menu.Add("Quit", func() {
		if options.Quit != nil {
			options.Quit()
		}
		remove()
	})

	lightIcon := appIcon(32, color.RGBA{R: 14, G: 42, B: 35, A: 255})
	darkIcon := appIcon(32, color.RGBA{R: 108, G: 244, B: 177, A: 255})
	native.SetIcon(lightIcon).
		SetDarkModeIcon(darkIcon).
		SetTemplateIcon(darkIcon).
		SetTooltip("AgentsUsage").
		SetMenu(menu)
	native.OnClick(func() {
		if err := OpenURL(options.DashboardURL); err != nil {
			native.ShowNotification("AgentsUsage", "Could not open the dashboard.")
		}
	})
	native.Show()

	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			updateMenu(native, statusItem, usageItem, startStopItem, controller.Status(), usage.Snapshot())
			select {
			case <-ctx.Done():
				remove()
				return
			case <-ticker.C:
			}
		}
	}()
	return native.Run()
}

func toggleServer(native *systray.SystemTray, controller ServerController) {
	status := controller.Status()
	var err error
	if status.State == service.StateRunning {
		err = controller.StopWithTimeout()
	} else if status.State == service.StateStopped {
		err = controller.Start()
	}
	if err != nil {
		native.ShowNotification("AgentsUsage", "The dashboard server could not change state.")
	}
}

func updateMenu(native *systray.SystemTray, statusItem, usageItem, startStopItem *systray.MenuItem, status service.Status, stats parser.Stats) {
	switch status.State {
	case service.StateRunning:
		statusItem.SetLabel("Server: Running · " + status.Address)
		startStopItem.SetLabel("Stop Server")
		startStopItem.SetDisabled(false)
	case service.StateStarting:
		statusItem.SetLabel("Server: Starting…")
		startStopItem.SetLabel("Starting Server…")
		startStopItem.SetDisabled(true)
	case service.StateStopping:
		statusItem.SetLabel("Server: Stopping…")
		startStopItem.SetLabel("Stopping Server…")
		startStopItem.SetDisabled(true)
	default:
		label := "Server: Stopped"
		if status.LastError != nil {
			label += " · error"
		}
		statusItem.SetLabel(label)
		startStopItem.SetLabel("Start Server")
		startStopItem.SetDisabled(false)
	}
	usageItem.SetLabel(fmt.Sprintf("Today: %s tokens · $%.2f", compactTokens(stats.Today.ProcessedTokens), stats.Today.EstimatedCostUSD))
	native.SetTooltip(fmt.Sprintf("AgentsUsage · %s tokens today", compactTokens(stats.Today.ProcessedTokens)))
}

func compactTokens(value int64) string {
	switch {
	case value >= 1_000_000_000:
		return fmt.Sprintf("%.1fB", float64(value)/1_000_000_000)
	case value >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(value)/1_000_000)
	case value >= 1_000:
		return fmt.Sprintf("%.1fK", float64(value)/1_000)
	default:
		return fmt.Sprintf("%d", value)
	}
}
