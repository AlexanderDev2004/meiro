package main

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"github.com/elianiva/meiro/m3"
)

// lastFMCard is where the user connects Last.fm, pauses scrobbling and
// disconnects. Connecting is the user's choice: nothing is sent before it.
func (a *app) lastFMCard(c *ui.Context) {
	s := &a.scrobble
	sc := m3.Of(c).Scheme
	a.settingsCard(c, "Last.fm", m3.IconMusicNote, func() {
		switch {
		case !s.client.Configured():
			settingLabel(c, "Scrobbling", "This build of Meiro has no Last.fm application key, so it cannot connect to Last.fm.")
		case s.connect.busy():
			ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
				settingLabel(c, "Waiting for approval", "Approve Meiro on the Last.fm page that opened in your browser. Meiro connects as soon as you do, and never sees your Last.fm password.").Grow(1)
				m3.LoadingIndicator(c, 40, false)
				if m3.Button(c, m3.ButtonSpec{Label: "Cancel", Kind: m3.Outlined, Size: m3.Medium56, Key: "lastfm-cancel"}).Clicked() {
					a.cancelLastFMConnect()
				}
			})
		case a.settings.LastFMUser == "":
			ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
				settingLabel(c, "Scrobbling", "Send the tracks you listen to, and what is playing now, to your Last.fm profile.").Grow(1)
				if m3.Button(c, m3.ButtonSpec{Label: "Connect Last.fm", Kind: m3.Filled, Size: m3.Medium56, Key: "lastfm-connect"}).Clicked() {
					a.connectLastFM()
				}
			})
		default:
			a.lastFMConnected(c)
		}
		if s.connectErr != "" {
			m3.Text(c, m3.BodyMedium, s.connectErr).TextColor(sc.Error).MaxLines(3)
		}
	})
}

// lastFMConnected is the card of an account that is connected.
func (a *app) lastFMConnected(c *ui.Context) {
	s := &a.scrobble
	sc := m3.Of(c).Scheme
	ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
		body := "Plays are sent to Last.fm after you have listened to them for long enough."
		if n := s.outbox.Len(); n > 0 {
			body += fmt.Sprintf(" %d waiting to be sent.", n)
		}
		settingLabel(c, "Connected as "+a.settings.LastFMUser, body).Grow(1)
		if m3.Button(c, m3.ButtonSpec{Label: "Disconnect", Kind: m3.Tonal, Size: m3.Medium56, Key: "lastfm-disconnect"}).Clicked() {
			a.disconnectLastFM()
		}
	})
	if s.reconnect {
		ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, m3.IconError).FontSize(24).TextColor(sc.Error)
			settingLabel(c, "Connect again", "Last.fm no longer accepts this connection. Plays wait on this device until you reconnect.").Grow(1)
			if m3.Button(c, m3.ButtonSpec{Label: "Reconnect", Kind: m3.Filled, Size: m3.Medium56, Key: "lastfm-reconnect"}).Clicked() {
				a.connectLastFM()
			}
		})
	}
	ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
		settingLabel(c, "Scrobble plays", "Turn off to pause scrobbling without disconnecting.").Grow(1)
		on := !a.settings.LastFMPaused
		if m3.Switch(c, &on, "Scrobble to Last.fm").Changed() {
			a.setLastFMPaused(!on)
		}
	})
}
