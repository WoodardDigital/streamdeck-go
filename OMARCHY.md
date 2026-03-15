# streamdeck-go — Omarchy Integration

Omarchy-specific commands and config examples for [streamdeck-go](README.md).
All commands run via `sh -c` on key press unless marked `priv:` (requires the
privileged helper — see [README § Privileged commands](README.md)).

---

## Terminal (Ghostty)

```yaml
keys:
  0:
    icon: ghostty.png
    command: ghostty

  # Open a new window with a specific command
  1:
    icon: ssh.png
    command: "ghostty -e ssh user@homeserver"

  # Open in a specific directory
  2:
    icon: projects.png
    command: "ghostty --working-directory=~/projects"
```

---

## Walker (app launcher)

```yaml
keys:
  3:
    icon: walker.png
    command: walker

  # Open walker directly in a specific mode
  4:
    icon: calc.png
    command: "walker --modules calc"
```

---

## Hyprland

Hyprland exposes everything via `hyprctl dispatch`. No privileges needed.

### Workspaces

```yaml
keys:
  0:
    icon: ws1.png
    command: "hyprctl dispatch workspace 1"
  1:
    icon: ws2.png
    command: "hyprctl dispatch workspace 2"
  2:
    icon: ws3.png
    command: "hyprctl dispatch workspace 3"

  # Move active window to a workspace
  8:
    icon: move.png
    command: "hyprctl dispatch movetoworkspace 2"

  # Toggle floating on the active window
  9:
    icon: float.png
    command: "hyprctl dispatch togglefloating"

  # Fullscreen
  10:
    icon: fullscreen.png
    command: "hyprctl dispatch fullscreen 0"

  # Kill active window
  11:
    icon: close.png
    command: "hyprctl dispatch killactive"
```

### Multi-monitor

```yaml
keys:
  16:
    icon: monitor-left.png
    command: "hyprctl dispatch focusmonitor l"
  17:
    icon: monitor-right.png
    command: "hyprctl dispatch focusmonitor r"

  # Move window to other monitor
  18:
    icon: move-monitor.png
    command: "hyprctl dispatch movewindow mon:next"
```

### Hyprlock (screen lock)

```yaml
keys:
  31:
    icon: lock.png
    command: hyprlock
```

### Hypridle (idle inhibitor)

```yaml
keys:
  30:
    icon: idle-off.png
    # Toggle idle inhibitor — stops screen from locking during presentations
    command: "pkill hypridle || hypridle &"
```

### Hyprpaper (wallpaper)

```yaml
keys:
  24:
    icon: wallpaper.png
    # Cycle to a specific wallpaper on the active monitor
    command: "hyprctl hyprpaper wallpaper 'eDP-1,~/wallpapers/mountain.jpg'"
```

---

## Waybar

```yaml
keys:
  7:
    icon: waybar-reload.png
    # Reload Waybar config and style without restarting
    command: "pkill -SIGUSR2 waybar"

  15:
    icon: waybar-toggle.png
    # Show/hide the bar
    command: "pkill -SIGUSR1 waybar"

  # Full restart (if SIGUSR2 isn't enough after big config changes)
  23:
    icon: waybar-restart.png
    command: "pkill waybar; waybar &"
```

---

## Mako (notifications)

```yaml
keys:
  5:
    icon: notif-dismiss.png
    # Dismiss all notifications
    command: "makoctl dismiss --all"

  6:
    icon: notif-invoke.png
    # Invoke the default action on the last notification
    command: "makoctl invoke"

  # Toggle do-not-disturb
  13:
    icon: dnd.png
    command: "makoctl set-mode $([ $(makoctl get-mode) = 'do-not-disturb' ] && echo default || echo do-not-disturb)"
```

---

## Audio (PipeWire / wpctl)

```yaml
keys:
  16:
    icon: vol-up.png
    command: "wpctl set-volume @DEFAULT_AUDIO_SINK@ 5%+"
  17:
    icon: vol-down.png
    command: "wpctl set-volume @DEFAULT_AUDIO_SINK@ 5%-"
  18:
    icon: mute.png
    command: "wpctl set-mute @DEFAULT_AUDIO_SINK@ toggle"

  # Mic mute (useful for calls)
  19:
    icon: mic-mute.png
    command: "wpctl set-mute @DEFAULT_AUDIO_SOURCE@ toggle"
```

---

## Brightness (brightnessctl)

```yaml
keys:
  20:
    icon: bright-up.png
    command: "brightnessctl set +10%"
  21:
    icon: bright-down.png
    command: "brightnessctl set 10%-"
  22:
    icon: bright-max.png
    command: "brightnessctl set 100%"
```

---

## Screenshots (grimblast / hyprshot)

Omarchy ships with `grimblast`. Output goes to `~/Pictures/Screenshots/` by default.

```yaml
keys:
  8:
    icon: screenshot.png
    # Fullscreen screenshot
    command: "grimblast save screen"

  9:
    icon: screenshot-area.png
    # Select area
    command: "grimblast save area"

  10:
    icon: screenshot-window.png
    # Active window
    command: "grimblast save active"

  # Copy to clipboard instead of saving
  11:
    icon: screenshot-clip.png
    command: "grimblast copy area"
```

---

## Night light (hyprsunset / wlsunset)

```yaml
keys:
  25:
    icon: nightlight.png
    command: "pkill hyprsunset; hyprsunset -t 3500 &"
  26:
    icon: nightlight-off.png
    command: "pkill hyprsunset"
```

---

## Power (privileged — requires helper)

These use the `priv:` prefix and must be added to `/etc/streamdeck-go/privileged.yaml`.

```yaml
# /etc/streamdeck-go/privileged.yaml
commands:
  suspend:  "systemctl suspend"
  hibernate: "systemctl hibernate"
  reboot:   "systemctl reboot"
  poweroff: "systemctl poweroff"
```

```yaml
# ~/.config/streamdeck-go/config.yaml
keys:
  28:
    icon: suspend.png
    command: "priv:suspend"
  29:
    icon: reboot.png
    command: "priv:reboot"
  30:
    icon: poweroff.png
    command: "priv:poweroff"
```

---

## Media (playerctl)

Works with Spotify, Firefox, mpv, and anything that exposes MPRIS.

```yaml
keys:
  20:
    icon: prev.png
    command: "playerctl previous"
  21:
    icon: playpause.png
    command: "playerctl play-pause"
  22:
    icon: next.png
    command: "playerctl next"
  23:
    icon: stop.png
    command: "playerctl stop"
```

---

## Useful hyprctl one-liners

| What | Command |
|---|---|
| List all windows | `hyprctl clients` |
| Active window info | `hyprctl activewindow` |
| List monitors | `hyprctl monitors` |
| Reload Hyprland config | `hyprctl reload` |
| Toggle special workspace | `hyprctl dispatch togglespecialworkspace` |
| Focus next window | `hyprctl dispatch cyclenext` |
| Rotate layout | `hyprctl dispatch layoutmsg orientationcycle` |
