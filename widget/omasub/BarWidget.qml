import QtQuick
import Quickshell
import Quickshell.Io
import Quickshell.Services.Mpris
import qs.Commons
import qs.Ui

BarWidget {
  id: root
  moduleName: "io.github.bprendie.omasub"

  property bool popupOpen: false
  readonly property bool opened: popupOpen
  readonly property bool popoutSwitchClosing: false
  readonly property var players: Mpris.players ? Mpris.players.values : []
  readonly property var player: findOmasubPlayer()
  readonly property bool active: player !== null
  readonly property bool playing: active && player.isPlaying
  readonly property bool paused: active && !playing && player.trackTitle !== ""
  readonly property string vuIcon: "󰺢"
  readonly property string stateIcon: playing ? "󰏤" : (paused ? "󰐊" : "󰓛")
  readonly property string tooltipText: trackLabel()
  readonly property string home: Quickshell.env("HOME") || ""
  readonly property string defaultOmasubBin: home + "/.local/bin/omasub"
  readonly property string omasubBin: settings && String(settings.binaryPath || "") !== ""
    ? String(settings.binaryPath) : defaultOmasubBin
  readonly property string configHome: Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")

  function findOmasubPlayer() {
    for (var i = 0; i < players.length; i++) {
      var candidate = players[i]
      var name = String(candidate && candidate.dbusName || "").toLowerCase()
      var identity = String(candidate && candidate.identity || "").toLowerCase()
      var desktop = String(candidate && candidate.desktopEntry || "").toLowerCase()
      if (name === "org.mpris.mediaplayer2.omasub" || identity === "omasub" || desktop === "omasub")
        return candidate
    }
    return null
  }

  function stateLabel() {
    if (!active) return "Omasub is not running"
    if (playing) return "Playing"
    if (paused) return "Paused"
    return "Idle"
  }

  function trackLabel() {
    if (!active || !player.trackTitle) return stateLabel()
    return player.trackArtist ? player.trackTitle + " — " + player.trackArtist : player.trackTitle
  }

  function playbackModeLabel() {
    if (!active) return "off"
    var repeats = player.loopState === MprisLoopState.Playlist
    if (player.shuffle && repeats) return "shuffle/repeat"
    if (player.shuffle) return "shuffle"
    return repeats ? "repeat" : "off"
  }

  function runPlayback(action) {
    if (!player) return
    if (action === "previous" && player.canGoPrevious) player.previous()
    else if (action === "next" && player.canGoNext) player.next()
    else if (action === "toggle" && player.canTogglePlaying) player.togglePlaying()
    else if (action === "toggle" && player.isPlaying && player.canPause) player.pause()
    else if (action === "toggle" && !player.isPlaying && player.canPlay) player.play()
    else if (action === "stop" && player.canControl) player.stop()
  }

  function runRemote(action) {
    if (actionProc.running) return
    actionProc.command = [root.omasubBin, "remote", action]
    actionProc.running = true
  }

  function launch() {
    if (!launchProc.running) launchProc.running = true
  }

  function close() { popupOpen = false }
  function open() { popupOpen = true }
  function toggle() { popupOpen = !popupOpen }
  function closeForPopoutSwitch() { close() }

  function openOmasub() {
    close()
    launch()
  }

  implicitWidth: row.implicitWidth + Style.space(14)
  implicitHeight: barSize

  Process {
    id: actionProc
  }

  Process {
    id: launchProc
    command: [root.configHome + "/omarchy/plugins/io.github.bprendie.omasub/widget/omasub/launch.sh", root.omasubBin]
  }

  Row {
    id: row
    anchors.centerIn: parent
    spacing: Style.space(6)

    Text {
      anchors.verticalCenter: parent.verticalCenter
      text: root.vuIcon
      color: root.active ? root.bar.barForeground : Qt.darker(root.bar.barForeground, 1.55)
      font.family: "CaskaydiaMono Nerd Font"
      font.pixelSize: Style.font.icon

    }

    Text {
      anchors.verticalCenter: parent.verticalCenter
      visible: !root.bar.vertical && root.active && root.player.trackTitle !== ""
      width: Math.min(180, implicitWidth)
      text: root.active ? root.player.trackTitle : ""
      color: root.bar.barForeground
      font.family: root.bar.fontFamily
      font.pixelSize: Style.font.body
      elide: Text.ElideRight
    }
  }

  MouseArea {
    anchors.fill: parent
    hoverEnabled: true
    cursorShape: Qt.PointingHandCursor
    acceptedButtons: Qt.LeftButton | Qt.MiddleButton
    onClicked: function(mouse) {
      if (mouse.button === Qt.MiddleButton && root.active) root.runPlayback("next")
      else if (root.active) root.toggle()
      else root.launch()
    }
    onWheel: function(wheel) {
      if (!root.active) return
      root.runPlayback(wheel.angleDelta.y > 0 ? "previous" : "next")
    }
    onEntered: if (root.bar) root.bar.showTooltip(root, root.tooltipText)
    onExited: if (root.bar) root.bar.hideTooltip(root)
  }

  PopupCard {
    id: popup
    anchorItem: root
    bar: root.bar
    owner: root
    open: root.popupOpen
    contentWidth: popup.fittedContentWidth(Style.space(320))
    contentHeight: popup.fittedContentHeight(content.implicitHeight)

    Column {
      id: content
      anchors.fill: parent
      spacing: Style.space(10)

      Row {
        width: parent.width
        spacing: Style.space(10)

        BorderSurface {
          width: Style.space(58)
          height: Style.space(58)
          radius: Style.spacing.labelGap
          color: Style.normalFillFor(root.bar.foreground, Color.accent)
          borderSpec: Border.controlSpec("normal", root.bar.foreground, Color.accent)

          Text {
            anchors.centerIn: parent
            text: root.vuIcon
            color: root.bar.foreground
            font.family: "CaskaydiaMono Nerd Font"
            font.pixelSize: Style.font.displayLarge
          }
        }

        Column {
          width: parent.width - Style.space(68)
          spacing: Style.space(3)

          Text {
            width: parent.width
            text: root.active && root.player.trackTitle ? root.player.trackTitle : "Omasub"
            color: root.bar.foreground
            font.family: root.bar.fontFamily
            font.pixelSize: Style.font.subtitle
            font.bold: true
            elide: Text.ElideRight
          }
          Text {
            width: parent.width
            text: root.active && root.player.trackArtist ? root.player.trackArtist : root.stateLabel()
            color: Qt.darker(root.bar.foreground, 1.3)
            font.family: root.bar.fontFamily
            font.pixelSize: Style.font.bodySmall
            elide: Text.ElideRight
          }
          Text {
            width: parent.width
            visible: text !== ""
            text: root.active ? root.player.trackAlbum : ""
            color: Qt.darker(root.bar.foreground, 1.6)
            font.family: root.bar.fontFamily
            font.pixelSize: Style.font.caption
            elide: Text.ElideRight
          }
        }
      }

      Row {
        anchors.horizontalCenter: parent.horizontalCenter
        spacing: Style.space(6)

        Button {
          iconText: "󰒮"
          foreground: root.bar.foreground
          enabled: root.active && root.player.canGoPrevious
          opacity: enabled ? 1.0 : 0.4
          onClicked: root.runPlayback("previous")
        }
        Button {
          iconText: root.stateIcon
          foreground: root.bar.foreground
          iconSize: Style.font.iconLarge
          enabled: root.active && (root.player.canTogglePlaying || root.player.canPlay || root.player.canPause)
          opacity: enabled ? 1.0 : 0.4
          onClicked: root.runPlayback("toggle")
        }
        Button {
          iconText: "󰒭"
          foreground: root.bar.foreground
          enabled: root.active && root.player.canGoNext
          opacity: enabled ? 1.0 : 0.4
          onClicked: root.runPlayback("next")
        }
        Button {
          iconText: "󰓛"
          foreground: root.bar.foreground
          enabled: root.active && root.player.canControl && root.player.trackTitle !== ""
          opacity: enabled ? 1.0 : 0.4
          onClicked: root.runPlayback("stop")
        }
      }

      Button {
        anchors.horizontalCenter: parent.horizontalCenter
        text: root.active ? "Open Omasub" : "Launch Omasub"
        foreground: root.bar.foreground
        onClicked: root.openOmasub()
      }

      Button {
        anchors.horizontalCenter: parent.horizontalCenter
        text: "Mode: " + root.playbackModeLabel()
        foreground: root.bar.foreground
        enabled: root.active
        opacity: enabled ? 1.0 : 0.4
        onClicked: root.runRemote("mode")
      }

      Button {
        anchors.horizontalCenter: parent.horizontalCenter
        text: "Quit and lock vault"
        foreground: root.bar.foreground
        enabled: root.active
        opacity: enabled ? 1.0 : 0.4
        onClicked: {
          root.popupOpen = false
          root.runRemote("quit")
        }
      }
    }
  }
}
