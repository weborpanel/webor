# Webooza lab helpers — intended to run on the Debian VM checkout.
# Usage: make help

SHELL := /bin/bash
export GOTOOLCHAIN ?= auto
export PATH := /usr/local/go/bin:$(PATH)

BIN_DIR    ?= $(CURDIR)/bin
LAB_STATE  ?= /tmp/webooza-lab
LAB_CONFIG ?= $(LAB_STATE)/etc/config.yaml
CADDY_JSON ?= /etc/webooza/caddy.json
SOCKET     ?= /run/webooza/helper.sock
LISTEN     ?= 0.0.0.0:8787
HTTP_LISTEN ?= :8080

HELPER_UNIT ?= webooza-helper-lab
PANEL_UNIT  ?= webooza-panel-lab
CADDY_UNIT  ?= webooza-caddy-lab

BINS := webooza webooza-helper wbz

.PHONY: help build clean test \
	lab-dirs lab-start lab-stop lab-restart lab-status lab-logs lab-health lab-reset

help:
	@echo "Webooza lab Makefile"
	@echo
	@echo "  make build         build webooza, webooza-helper, wbz into $(BIN_DIR)"
	@echo "  make test          go test ./..."
	@echo "  make lab-start     start helper + caddy + panel (systemd-run)"
	@echo "  make lab-stop      stop lab units"
	@echo "  make lab-restart   stop then start"
	@echo "  make lab-status    unit + listen status"
	@echo "  make lab-health    curl panel + sample site"
	@echo "  make lab-logs      recent panel/helper/caddy journals"
	@echo "  make lab-reset     wipe $(LAB_STATE) (new setup token on next start)"
	@echo "  make clean         remove $(BIN_DIR)"

build:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/webooza ./cmd/webooza
	go build -o $(BIN_DIR)/webooza-helper ./cmd/webooza-helper
	go build -o $(BIN_DIR)/wbz ./cmd/wbz
	@ls -lh $(addprefix $(BIN_DIR)/,$(BINS))

test:
	go test ./...

clean:
	rm -rf $(BIN_DIR)

lab-dirs:
	sudo mkdir -p /run/webooza /var/lib/webooza /etc/webooza /srv/sites
	# Lab state lives under /tmp and must stay user-writable (not root).
	@if [ -d "$(LAB_STATE)" ] && [ ! -w "$(LAB_STATE)" ]; then \
		sudo chown -R "$$USER:$$USER" "$(LAB_STATE)"; \
	fi
	mkdir -p $(LAB_STATE)/state $(LAB_STATE)/etc
	@if [ ! -f $(CADDY_JSON) ]; then \
		sudo cp internal/providers/caddy/testdata/good.json $(CADDY_JSON); \
	fi
	@if [ ! -f $(LAB_CONFIG) ]; then \
		cp internal/config/testdata/lab-static.yaml $(LAB_CONFIG); \
	fi

lab-start: build lab-dirs
	@sudo systemctl stop $(HELPER_UNIT) $(PANEL_UNIT) $(CADDY_UNIT) 2>/dev/null || true
	@sudo systemctl reset-failed $(HELPER_UNIT) $(PANEL_UNIT) $(CADDY_UNIT) 2>/dev/null || true
	sudo systemd-run --unit=$(HELPER_UNIT) --collect \
		$(BIN_DIR)/webooza-helper -socket $(SOCKET) -state-dir /var/lib/webooza
	@sleep 0.4
	sudo chmod 666 $(SOCKET)
	sudo systemd-run --unit=$(CADDY_UNIT) --collect \
		/usr/bin/caddy run --config $(CADDY_JSON)
	sudo systemd-run --unit=$(PANEL_UNIT) --collect \
		$(BIN_DIR)/webooza \
		-listen $(LISTEN) \
		-state-dir $(LAB_STATE)/state \
		-config $(LAB_CONFIG) \
		-disable-https \
		-caddy-json $(CADDY_JSON) \
		-http-listen $(HTTP_LISTEN)
	@sleep 0.6
	@$(MAKE) --no-print-directory lab-status
	@$(MAKE) --no-print-directory lab-health
	@echo
	@echo "Setup token (if first run):"
	@sudo journalctl -u $(PANEL_UNIT) -n 30 --no-pager | grep -F 'setup token' || true
	@echo "Panel:  http://127.0.0.1:8787/healthz  (host via VBox NAT)"
	@echo "Site:   curl -H 'Host: example.com' http://127.0.0.1:8080/"

lab-stop:
	sudo systemctl stop $(PANEL_UNIT) $(HELPER_UNIT) $(CADDY_UNIT) 2>/dev/null || true
	sudo systemctl reset-failed $(PANEL_UNIT) $(HELPER_UNIT) $(CADDY_UNIT) 2>/dev/null || true
	@echo "lab stopped"

lab-restart: lab-stop lab-start

lab-status:
	@echo "units:"
	@systemctl is-active $(HELPER_UNIT) $(CADDY_UNIT) $(PANEL_UNIT) 2>/dev/null || true
	@echo "listen:"
	@ss -ltn | grep -E ':(8787|8080|2019)\s' || echo "(none)"

lab-health:
	@curl -sf http://127.0.0.1:8787/healthz; echo
	@curl -sS -H 'Host: example.com' http://127.0.0.1:8080/ | head -c 200; echo

lab-logs:
	sudo journalctl -u $(PANEL_UNIT) -u $(HELPER_UNIT) -u $(CADDY_UNIT) -n 80 --no-pager

lab-reset: lab-stop
	rm -rf $(LAB_STATE)
	@echo "wiped $(LAB_STATE); run make lab-start for a fresh setup token"
