BINARY        := streamdeck-go
HELPER        := streamdeck-helper
PREFIX        ?= $(HOME)/.local
BIN_DIR       := $(PREFIX)/bin
SYS_BIN       := /usr/local/bin
CONFIG_DIR    := $(HOME)/.config/streamdeck-go
SYSTEMD_USER  := $(HOME)/.config/systemd/user
SYSTEMD_SYS   := /etc/systemd/system
ETC_DIR       := /etc/streamdeck-go
UDEV_RULE     := /etc/udev/rules.d/99-streamdeck.rules
GROUP         := streamdeck

.PHONY: build build-helper install install-helper uninstall udev

# ── Build ─────────────────────────────────────────────────────────────────────

build:
	go build -o $(BINARY) ./cmd/streamdeck/

build-helper:
	go build -o $(HELPER) ./cmd/streamdeck-helper/

# ── Install ───────────────────────────────────────────────────────────────────

install: build udev _install-user
	@echo ""
	@echo "Done. Edit $(CONFIG_DIR)/config.yaml to configure your keys."
	@echo "To enable privileged commands (suspend, reboot, etc): make install-helper"

_install-user:
	install -Dm755 $(BINARY) $(BIN_DIR)/$(BINARY)
	mkdir -p $(CONFIG_DIR)/icons
	@if [ ! -f $(CONFIG_DIR)/config.yaml ]; then \
		install -Dm644 config.example.yaml $(CONFIG_DIR)/config.yaml; \
		echo "Default config written to $(CONFIG_DIR)/config.yaml"; \
	else \
		echo "Config already exists — not overwriting"; \
	fi
	install -Dm644 systemd/streamdeck-go.service $(SYSTEMD_USER)/streamdeck-go.service
	systemctl --user daemon-reload
	systemctl --user enable --now streamdeck-go.service

# Install the privileged helper (requires sudo).
# Creates a 'streamdeck' group, adds the current user to it, installs the
# helper binary as root, and enables it as a system service.
install-helper: build-helper
	# Create group and add current user.
	@if ! getent group $(GROUP) > /dev/null; then \
		sudo groupadd $(GROUP); \
		echo "Created group '$(GROUP)'"; \
	fi
	sudo usermod -aG $(GROUP) $(USER)
	# Install helper binary (owned root, not world-executable by others).
	sudo install -Dm750 $(HELPER) $(SYS_BIN)/$(HELPER)
	sudo chown root:$(GROUP) $(SYS_BIN)/$(HELPER)
	# Install whitelist config (root-owned, not world-writable).
	sudo mkdir -p $(ETC_DIR)
	@if [ ! -f $(ETC_DIR)/privileged.yaml ]; then \
		sudo install -Dm640 config/privileged.example.yaml $(ETC_DIR)/privileged.yaml; \
		sudo chown root:$(GROUP) $(ETC_DIR)/privileged.yaml; \
		echo "Whitelist written to $(ETC_DIR)/privileged.yaml"; \
	else \
		echo "Whitelist already exists — not overwriting"; \
	fi
	# Install and enable system service.
	sudo install -Dm644 systemd/streamdeck-go-helper.service $(SYSTEMD_SYS)/streamdeck-go-helper.service
	sudo systemctl daemon-reload
	sudo systemctl enable --now streamdeck-go-helper.service
	@echo ""
	@echo "Helper installed. Edit $(ETC_DIR)/privileged.yaml (as root) to add commands."
	@echo "NOTE: log out and back in for group membership to take effect,"
	@echo "      or run: newgrp $(GROUP)"

udev:
	@if [ ! -f $(UDEV_RULE) ]; then \
		echo 'KERNEL=="hidraw*", ATTRS{idVendor}=="0fd9", MODE="0666"' \
			| sudo tee $(UDEV_RULE); \
		sudo udevadm control --reload; \
		sudo udevadm trigger; \
		echo "udev rule installed."; \
	else \
		echo "udev rule already exists."; \
	fi

# ── Uninstall ─────────────────────────────────────────────────────────────────

uninstall:
	systemctl --user disable --now streamdeck-go.service || true
	rm -f $(BIN_DIR)/$(BINARY)
	rm -f $(SYSTEMD_USER)/streamdeck-go.service
	systemctl --user daemon-reload
	@echo "Uninstalled. Config at $(CONFIG_DIR) preserved."

uninstall-helper:
	sudo systemctl disable --now streamdeck-go-helper.service || true
	sudo rm -f $(SYS_BIN)/$(HELPER)
	sudo rm -f $(SYSTEMD_SYS)/streamdeck-go-helper.service
	sudo systemctl daemon-reload
	@echo "Helper uninstalled. Whitelist at $(ETC_DIR)/privileged.yaml preserved."
