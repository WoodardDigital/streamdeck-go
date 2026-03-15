# streamdeck-go

> [!IMPORTANT]
> This is a sloppy utility that's designed to just work. [406.fail](https://406.fail)


A lightweight, dotfile-style controller for the **Elgato Stream Deck XL** on Linux.
No Elgato software required — communicates directly with the device over USB HID.

---

## Features

- Configure keys with a single YAML file (Designed for dotfiles compatability)
- PNG and JPEG icons, automatically scaled to key size
- Animated GIF support — frames pre-encoded at startup, cycled at the GIF's native rate
- Runs any shell command on key press
- **Status/toggle keys** — poll any shell command on an interval, swap icons based on output; icon updates on press
- **Live config reload** — save your config and the deck updates instantly, no restart needed
- Privileged command helper — run whitelisted root commands via a Unix socket; supports polkit auth dialogs
- Automatic reconnect — survives USB unplug, KVM switches, and suspend/resume
- Runs as a systemd user service, starts automatically with your desktop session
- No Stream Deck app, no Node.js, no Electron

**Planned:** text/label overlays on keys, multi-page layouts, AUR package — see [Roadmap](#roadmap)

---

## Architecture

```
streamdeck-go/
├── cmd/
│   └── streamdeck/
│       └── main.go              # Entry point, config watcher, event loop
├── internal/
│   ├── config/
│   │   └── config.go            # YAML parsing with XDG-aware defaults
│   └── device/
│       └── streamdeck.go        # USB HID communication, image encoding, button reads
├── systemd/
│   └── streamdeck-go.service    # Systemd user service unit
├── config.example.yaml          # Starter config (copied to ~/.config on install)
├── Makefile                     # build / install / uninstall
├── go.mod
└── go.sum
```

### How it works

```
~/.config/streamdeck-go/config.yaml
    │
    ├── fsnotify watcher ──── file saved? ──▶ cancel ctx → reload config → restart run()
    │
    └──▶ run(ctx)
              │
              ├── static icon ──▶ device.SetKeyImage()   (scale → flip → JPEG → HID)
              │
              ├── animated GIF ──▶ device.EncodeFrame()  (pre-encode all frames once)
              │                        └──▶ goroutine/key: loop frames, sleep per-frame delay
              │                             (cancelled via ctx on reload)
              │
              ├── poll goroutine/key ──▶ exec poll command every interval
              │                            └──▶ match in stdout? swap icon_true / icon_false
              │                                 (also triggered immediately on button press)
              │
              └── event loop ──▶ device.ReadButtons()    (250 ms timeout, checks ctx)
                                      └──▶ key-down: exec.Command("sh", "-c", command)
```

HID output reports are mutex-guarded so concurrent animation goroutines never
interleave partial image data across keys.

---

## Dependencies

### Go packages

| Package | Purpose |
|---|---|
| [`github.com/sstallion/go-hid`](https://github.com/sstallion/go-hid) | Bindings for `libhidapi` — USB HID read/write |
| [`github.com/fsnotify/fsnotify`](https://github.com/fsnotify/fsnotify) | Config file watching for live reload |
| [`golang.org/x/image`](https://pkg.go.dev/golang.org/x/image) | Bi-linear image scaling |
| [`gopkg.in/yaml.v3`](https://pkg.go.dev/gopkg.in/yaml.v3) | YAML config parsing |
| Go stdlib `image/gif`, `image/jpeg`, `image/png` | Image decoding and JPEG encoding |

### System library

`libhidapi` must be present at runtime:

| Distro | Command |
|---|---|
| Arch / Manjaro | `sudo pacman -S hidapi` |
| Debian / Ubuntu | `sudo apt install libhidapi-hidraw0` |
| Fedora / RHEL | `sudo dnf install hidapi` |
| openSUSE | `sudo zypper install libhidapi-hidraw0` |

---

## Installation

### Option A — `make install` (recommended)

An interactive installer that handles everything, including optional dotfiles
directory integration.

```bash
# Prerequisites
sudo pacman -S go hidapi   # Arch; adjust for your distro (see table above)

git clone https://github.com/WoodardDigital/streamdeck-go
cd streamdeck-go
make install
```

The installer walks you through the whole setup:

```
  ❯ Building streamdeck-go...
  ✓ Build complete

  ❯ Checking udev rule...
  ✓ udev rule already installed — skipping

  Config location
  ─────────────────────────────────────────────

  · streamdeck-go stores its config and icons in a single directory.
  · You can keep that directory inside your dotfiles repo and symlink it
  · into ~/.config — the same pattern used by Hyprland, Waybar, etc.

  ❯ Use a dotfiles directory? [Y/n]
  ❯ Path to dotfiles repo [~/dotfiles]:

  · Will create:
  ·   ~/dotfiles/.config/streamdeck-go/
  ·   ~/dotfiles/.config/streamdeck-go/config.yaml
  ·   ~/dotfiles/.config/streamdeck-go/icons/

  · Will symlink:
  ·   ~/.config/streamdeck-go
  ·   └─▶ ~/dotfiles/.config/streamdeck-go

  ❯ Confirm? [Y/n]
```

The resulting structure inside your dotfiles repo mirrors everything else in `.config`:

```
~/dotfiles/
└── .config/
    ├── hypr/
    ├── waybar/
    └── streamdeck-go/       ← lives here, symlinked to ~/.config/streamdeck-go
        ├── config.yaml
        └── icons/
```

After install, edit and save `config.yaml` — the deck reloads live, no restart needed:

```bash
$EDITOR ~/.config/streamdeck-go/config.yaml
```

To remove:

```bash
make uninstall   # stops the service and removes the binary; config is preserved
```

---

### Option B — manual / dev setup

Use this if you want to run directly from the repo (e.g. while developing).

**1. udev rule** (one-time, needed once regardless of install method):

```bash
echo 'KERNEL=="hidraw*", ATTRS{idVendor}=="0fd9", MODE="0666"' \
  | sudo tee /etc/udev/rules.d/99-streamdeck.rules
sudo udevadm control --reload
sudo udevadm trigger
```

**2. Build and run:**

```bash
git clone https://github.com/WoodardDigital/streamdeck-go
cd streamdeck-go

# Run with the repo's config.yaml (created from the example if absent):
cp config.example.yaml config.yaml
go run ./cmd/streamdeck/

# Or point at any config file:
go run ./cmd/streamdeck/ -config ~/.config/streamdeck-go/config.yaml
```

When no `-config` flag is given, the binary always checks
`~/.config/streamdeck-go/config.yaml` first (respecting `$XDG_CONFIG_HOME`).
The repo's `config.yaml` is gitignored — it's your local scratchpad.

---

### Option C — AUR (Arch Linux)

> AUR package coming soon. Until then, use `make install` above.

---

## Systemd service

The service file lives at `systemd/streamdeck-go.service` in the repo and is
installed to `~/.config/systemd/user/streamdeck-go.service` by `make install`.

Useful commands:

```bash
systemctl --user status streamdeck-go      # check if running
systemctl --user restart streamdeck-go     # restart manually
systemctl --user stop streamdeck-go        # stop
journalctl --user -u streamdeck-go -f      # follow logs
```

To use a custom config path with the service, edit the unit after install:

```bash
systemctl --user edit streamdeck-go
```

Add:

```ini
[Service]
ExecStart=
ExecStart=%h/.local/bin/streamdeck-go -config %h/.config/streamdeck-go/config.yaml
```

---

## Configuration

Config lives at `~/.config/streamdeck-go/config.yaml` (or wherever `-config` points).
**Editing and saving the file reloads the deck live** — icons update, animations restart,
no service restart required.

```yaml
icons_dir: ~/.config/streamdeck-go/icons  # default; can be any path
brightness: 70                             # 0–100

# USB IDs — defaults match Stream Deck XL v2
# Run: lsusb | grep Elgato
device:
  vendor_id: 0x0fd9
  product_id: 0x00ba

# Keys are 0-indexed, left-to-right, top-to-bottom.
# Stream Deck XL layout (8 columns × 4 rows):
#
#  0  1  2  3  4  5  6  7
#  8  9 10 11 12 13 14 15
# 16 17 18 19 20 21 22 23
# 24 25 26 27 28 29 30 31

keys:
  0:
    icon: ghostty.png     # PNG, JPEG, or GIF — relative to icons_dir
    command: ghostty
  1:
    icon: firefox.png
    command: firefox
  8:
    icon: loading.gif     # animated — cycles at the GIF's native frame rate
    command: ""
```

### Status / toggle keys

A key can poll any shell command on an interval and show one of two icons based on the result. Pressing the button runs `command` as usual, and the icon re-checks ~400 ms later so it reflects the new state immediately.

```yaml
keys:
  3:
    command: pactl set-source-mute @DEFAULT_SOURCE@ toggle
    icon_true:  mic-muted.png   # shown when poll output contains match
    icon_false: mic-active.png  # shown when poll output does not contain match
    poll:
      command:  pactl get-source-mute @DEFAULT_SOURCE@
      interval: 2s              # how often to check (default: 2s)
      match: "yes"              # substring to find in stdout → true
```

**How matching works:**

| `match` set? | True condition | False condition |
|---|---|---|
| Yes | stdout contains the string | stdout does not contain it |
| No (omitted) | command exits 0 | command exits non-zero |

**More examples:**

```yaml
# VPN status (exit-code match — no match string needed)
4:
  command: nmcli connection up my-vpn
  icon_true:  vpn-on.png
  icon_false: vpn-off.png
  poll:
    command: nmcli connection show --active my-vpn
    interval: 5s

# Systemd service toggle
5:
  command: systemctl --user toggle my-service
  icon_true:  service-running.png
  icon_false: service-stopped.png
  poll:
    command:  systemctl --user is-active my-service
    interval: 3s

# Speaker mute
6:
  command: pactl set-sink-mute @DEFAULT_SINK@ toggle
  icon_true:  speaker-muted.png
  icon_false: speaker-on.png
  poll:
    command:  pactl get-sink-mute @DEFAULT_SINK@
    interval: 2s
    match: "yes"
```

---

### Terminal & SSH commands

Any shell command works — including launching terminals and SSH sessions.

**Open a terminal on key press:**

```yaml
keys:
  0:
    icon: ghostty.png
    command: ghostty
  1:
    icon: terminal.png
    command: alacritty          # or kitty, wezterm, foot, etc.
```

**SSH — open an interactive session in a terminal:**

```yaml
keys:
  5:
    icon: homeserver.png
    command: "ghostty -e ssh user@homeserver"
  6:
    icon: pi.png
    command: "alacritty -e ssh pi@raspberrypi.local"
```

This is the recommended pattern for SSH — the terminal handles the TTY,
resize events, and any passphrase prompt.

**SSH — run a remote command silently (no terminal):**

```yaml
keys:
  7:
    icon: deploy.png
    command: "ssh user@host 'cd /app && git pull && systemctl restart app'"
```

Works as long as SSH key auth is set up and the key has no passphrase (or the agent is available — see note below).

> **SSH agent & the systemd service**
>
> The service starts before your desktop session fully initialises, so
> `SSH_AUTH_SOCK` (used for passphrase-protected keys) may not be in its
> environment. Fix by importing it from your session startup:
>
> ```bash
> # Add to ~/.config/fish/config.fish, ~/.bashrc, or session init:
> systemctl --user import-environment SSH_AUTH_SOCK
> ```
>
> Or hardcode the socket path in the service:
>
> ```bash
> systemctl --user edit streamdeck-go
> ```
>
> ```ini
> [Service]
> Environment=SSH_AUTH_SOCK=%t/keyring/ssh
> ```
>
> Run `echo $SSH_AUTH_SOCK` in a terminal to find the correct path for your
> desktop environment.

**Common terminal flags:**

| Terminal | Flag to run a command |
|---|---|
| ghostty | `ghostty -e <cmd>` |
| alacritty | `alacritty -e <cmd>` |
| kitty | `kitty <cmd>` |
| foot | `foot <cmd>` |
| wezterm | `wezterm start -- <cmd>` |

---

### Supported icon formats

| Format | Notes |
|---|---|
| PNG | Recommended for static icons |
| JPEG | Good for photos / complex images |
| GIF | Animated — all frames pre-encoded at startup, cycled in a background goroutine |
| SVG | Rasterised to 96×96 at startup via `oksvg` |

Icons are scaled to 96×96 px using bi-linear filtering. The XL renders images
mirrored, so they are pre-flipped before sending — your icons will appear the right
way round.

---

## Supported Devices

| Model | Product ID | Keys | Status |
|---|---|---|---|
| Stream Deck XL v2 | `0x00ba` | 32 | tested |
| Stream Deck XL v1 | `0x006c` | 32 | untested |
| Stream Deck MK.2  | `0x006d` | 15 | untested |

Run `lsusb | grep Elgato` to find your device's product ID.
To add a model, edit the `models` map in [internal/device/streamdeck.go](internal/device/streamdeck.go).

---

## Roadmap

### Text / label overlay on icons

Render dynamic text directly onto a key image at runtime — useful for showing
live state like volume level, a clock, a counter, or the current git branch.
Icons would be composited with a text layer before being sent to the device, so
no pre-made image is needed for every possible value.

Example config (proposed):

```yaml
keys:
  4:
    icon: volume.png
    label: "$(pactl get-sink-volume @DEFAULT_SINK@ | awk '{print $5}')"
    label_position: bottom   # top | center | bottom
    refresh: 5s              # re-evaluate and redraw every 5 seconds
```

### Multi-page layouts

Support more than 32 actions by organising keys into named pages. A designated
key (or key combination) switches between pages. The deck reloads instantly with
the new page's icons when switching.

Example config (proposed):

```yaml
pages:
  default:
    0:
      icon: apps.png
      command: "page:apps"    # switch to the 'apps' page
    1:
      icon: firefox.png
      command: firefox

  apps:
    0:
      icon: back.png
      command: "page:default"
    1:
      icon: ghostty.png
      command: ghostty
    2:
      icon: obsidian.png
      command: obsidian
```

### AUR package

A `PKGBUILD` for Arch Linux so the full install (binary, services, udev rule,
config skeleton) is handled by `yay` or `paru` like any other package.

```bash
yay -S streamdeck-go   # coming soon
```
