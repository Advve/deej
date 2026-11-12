package deej

import (
	"os"

	// fyne.io/systray draws the Linux tray icon over D-Bus (StatusNotifierItem), so unlike the
	// old getlantern/systray it needs no GTK, WebKit or libappindicator, and no cgo at all
	"fyne.io/systray"
	"go.uber.org/zap"

	"github.com/omriharel/deej/pkg/deej/icon"
	"github.com/omriharel/deej/pkg/deej/util"
)

func (d *Deej) initializeTray(onDone func()) {
	logger := d.logger.Named("tray")

	onReady := func() {
		logger.Debug("Tray instance ready")

		watchSystemTheme()

		systray.SetTemplateIcon(icon.DeejLogo, icon.DeejLogo)
		systray.SetTitle("deej")
		systray.SetTooltip("deej")

		updates := d.addUpdateAvailable()

		editConfig := systray.AddMenuItem("Edit configuration", "Open config file with notepad")
		editConfig.SetIcon(icon.EditConfig)

		refreshSessions := systray.AddMenuItem("Re-scan audio sessions", "Manually refresh audio sessions if something's stuck")
		refreshSessions.SetIcon(icon.RefreshSessions)

		systray.AddSeparator()
		autostart := systray.AddMenuItemCheckbox(autostartLabel, autostartTooltip, autostartEnabled())
		updates.addSettings()

		if d.version != "" {
			systray.AddSeparator()
			versionInfo := systray.AddMenuItem(d.version, "")
			versionInfo.Disable()
		}

		systray.AddSeparator()
		quit := systray.AddMenuItem("Quit", "Stop deej and quit")

		// wait on things to happen
		go func() {
			for {
				select {

				// quit
				case <-quit.ClickedCh:
					logger.Info("Quit menu item clicked, stopping")

					d.signalStop()

				// edit config
				case <-editConfig.ClickedCh:
					logger.Info("Edit config menu item clicked, opening config for editing")

					// on Linux, open it with whatever the desktop uses for text files
					// (current GNOME no longer ships gedit)
					editor := "notepad.exe"
					if util.Linux() {
						editor = "xdg-open"
					}

					if err := util.OpenExternal(logger, editor, userConfigFilepath); err != nil {
						logger.Warnw("Failed to open config file for editing", "error", err)
					}

				// start with the OS
				case <-autostart.ClickedCh:
					enable := !autostart.Checked()
					logger.Infow("Autostart menu item clicked", "enable", enable)

					if err := setAutostart(enable); err != nil {
						logger.Warnw("Failed to change autostart", "error", err)
						continue
					}

					if enable {
						autostart.Check()
					} else {
						autostart.Uncheck()
					}

				// refresh sessions
				case <-refreshSessions.ClickedCh:
					logger.Info("Refresh sessions menu item clicked, triggering session map refresh")

					// performance: the reason that forcing a refresh here is okay is that users can't spam the
					// right-click -> select-this-option sequence at a rate that's meaningful to performance
					d.sessions.opLock.Lock()
					d.sessions.refreshSessions(true)
					d.sessions.opLock.Unlock()
				}
			}
		}()

		// actually start the main runtime
		onDone()
	}

	onExit := func() {
		logger.Debug("Tray exited")
	}

	repairAutostart(logger)

	// start the tray icon
	logger.Debug("Running in tray")
	followSystemMenuTheme() // before any menu exists
	systray.Run(onReady, onExit)
}

// repairAutostart re-points the autostart entry at this copy of deej if the program it pointed to
// was moved or deleted. A working entry is left alone, so trying a second copy doesn't take it over.
func repairAutostart(logger *zap.SugaredLogger) {
	p := autostartPath()
	if p == "" {
		return
	}
	if _, err := os.Stat(p); os.IsNotExist(err) {
		if err := setAutostart(true); err != nil {
			logger.Warnw("Failed to repair autostart entry", "error", err)
		} else {
			logger.Infow("Autostart entry pointed to a missing program, now points to this copy", "old", p)
		}
	}
}

func (d *Deej) stopTray() {
	d.logger.Debug("Quitting tray")
	systray.Quit()
}
