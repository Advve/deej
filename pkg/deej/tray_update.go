package deej

import (
	"context"

	"fyne.io/systray"
)

// The update items in the tray menu (see update.go): "Update available" at the top, hidden until a
// check finds a newer release, and an "Updates" submenu next to the autostart item.
type updateMenu struct {
	d   *Deej
	upd *updater

	available, install, notes, skip *systray.MenuItem
}

func onMenuClick(item *systray.MenuItem, fn func()) {
	go func() {
		for range item.ClickedCh {
			fn()
		}
	}()
}

// addUpdateAvailable adds the (hidden) "Update available" item where the menu is at the moment.
func (d *Deej) addUpdateAvailable() *updateMenu {
	logger := d.logger.Named("update")
	m := &updateMenu{d: d, upd: newUpdater(d.appVersion, func(err error) {
		logger.Warnw("Update check failed", "error", err)
	})}

	m.available = systray.AddMenuItem("", "")
	m.install = m.available.AddSubMenuItem("", "")
	m.notes = m.available.AddSubMenuItem("What's new", "Open the release page")
	m.skip = m.available.AddSubMenuItem("Skip this version", "Don't show this version again")
	m.available.Hide()

	m.upd.onChange = func() { // a background check found a new release
		m.show()
		r := m.upd.Available()
		if r == nil || r.Version() == d.config.prefString(prefKeySkipVersion) ||
			r.Version() == d.config.prefString(prefKeyNotifiedVersion) {
			return // one notification per version, none for skipped ones
		}
		m.savePref(prefKeyNotifiedVersion, r.Version())
		d.notifier.Notify("deej "+r.Version()+" is available", "Install it from the tray menu.")
	}

	onMenuClick(m.install, func() {
		r := m.upd.Available()
		if r == nil {
			return
		}
		if !m.upd.CanInstall(r) {
			m.openPage(r)
			return
		}
		m.install.Disable()
		m.available.SetTitle("Installing update…")
		exe, err := m.upd.Install(context.Background(), r)
		if err != nil {
			logger.Warnw("Failed to install update", "version", r.Version(), "error", err)
			d.notifier.Notify("Couldn't install the update", "Please check deej's logs for more details.")
			m.install.Enable()
			m.show()
			return
		}
		logger.Infow("Installed update, restarting", "version", r.Version())
		d.restartExe = exe // run() starts it once the serial port is closed
		d.signalStop()
	})
	onMenuClick(m.notes, func() {
		if r := m.upd.Available(); r != nil {
			m.openPage(r)
		}
	})
	onMenuClick(m.skip, func() {
		if r := m.upd.Available(); r != nil {
			m.savePref(prefKeySkipVersion, r.Version())
		}
		m.show()
	})

	return m
}

// addSettings adds the "Updates" submenu where the menu is at the moment, and starts the checks.
func (m *updateMenu) addSettings() {
	d := m.d
	updates := systray.AddMenuItem("Updates", "")
	auto := updates.AddSubMenuItemCheckbox("Check automatically",
		"Once a day, ask GitHub whether a new version of deej is out", d.config.prefBool(prefKeyUpdateCheck))
	checkNow := updates.AddSubMenuItem("Check now", "")

	onMenuClick(auto, func() {
		on := !auto.Checked()
		if on {
			auto.Check()
		} else {
			auto.Uncheck()
		}
		m.savePref(prefKeyUpdateCheck, on)
	})
	onMenuClick(checkNow, func() {
		r, err := m.upd.Check(context.Background())
		switch {
		case err != nil:
			d.logger.Warnw("Update check failed", "error", err)
			d.notifier.Notify("Couldn't check for updates", "Please check deej's logs for more details.")
		case r == nil:
			d.notifier.Notify("deej is up to date", "You have the latest version, "+d.appVersion+".")
		default:
			if r.Version() == d.config.prefString(prefKeySkipVersion) { // asking again brings it back
				m.savePref(prefKeySkipVersion, "")
			}
			m.show()
			d.notifier.Notify("deej "+r.Version()+" is available", "Install it from the tray menu.")
		}
	})

	// automatic checks only in release builds, so dev runs don't nag
	go m.upd.Run(context.Background(), func() bool {
		return d.releaseBuild && d.config.prefBool(prefKeyUpdateCheck)
	})
	if d.justUpdated {
		d.notifier.Notify("deej updated", "Now running version "+d.appVersion+".")
	}
}

func (m *updateMenu) show() {
	r := m.upd.Available()
	if r == nil || r.Version() == m.d.config.prefString(prefKeySkipVersion) {
		m.available.Hide()
		return
	}
	m.available.SetTitle("Update available: " + r.Version())
	if m.upd.CanInstall(r) {
		m.install.SetTitle("Install and restart")
		m.install.SetTooltip("Download, verify and install " + r.Version() + ", then restart deej")
	} else {
		m.install.SetTitle("Download…")
		m.install.SetTooltip("Open the release page")
	}
	m.available.Show()
}

func (m *updateMenu) openPage(r *release) {
	if err := openURL(r.Page); err != nil {
		m.d.logger.Warnw("Failed to open release page", "error", err)
	}
}

func (m *updateMenu) savePref(key string, value any) {
	if err := m.d.config.setPref(key, value); err != nil {
		m.d.logger.Warnw("Failed to save preferences", "key", key, "error", err)
	}
}
