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

.PHONY: help web build clean test \
	lab-dirs lab-start lab-stop lab-restart lab-status lab-logs lab-health lab-reset \
	lab-app lab-deploy lab-idle lab-cron lab-node lab-proxyd lab-job

NODE_SAMPLE ?= $(CURDIR)/examples/socket-activation/node-app/server.js
JOB_SAMPLE ?= $(CURDIR)/examples/m2-job/lab-keep.sh

help:
	@echo "Webooza lab Makefile"
	@echo
	@echo "  make web           build Svelte UI into internal/ui/dist (needs Node >= 22.17)"
	@echo "  make build         build Go binaries into $(BIN_DIR) (uses embedded UI)"
	@echo "  make test          go test ./..."
	@echo "  make lab-start     start helper + caddy + panel (systemd-run)"
	@echo "  make lab-app       apply socket-activated app.example.com (needs lab-start/caddy)"
	@echo "  make lab-deploy    binary deploy + failed-probe rollback (needs lab-app)"
	@echo "  make lab-idle      idle-stop (scale to zero) then wake-on-request (needs lab-app)"
	@echo "  make lab-cron      install cron timer + run-now smoke (needs lab-app site user)"
	@echo "  make lab-node      Node sample: wake + idle-stop + deploy/rollback (needs node + lab-start)"
	@echo "  make lab-proxyd    proxyd idle fallback (--exit-idle-time) + wake"
	@echo "  make lab-job       long-running job-as-service smoke"
	@echo "  make lab-stop      stop lab units"
	@echo "  make lab-restart   stop then start"
	@echo "  make lab-status    unit + listen status"
	@echo "  make lab-health    curl panel + sample site"
	@echo "  make lab-logs      recent panel/helper/caddy journals"
	@echo "  make lab-reset     wipe $(LAB_STATE) (new setup token on next start)"
	@echo "  make clean         remove $(BIN_DIR)"

web:
	cd web && npm ci && npm run build
	rm -rf internal/ui/dist
	mkdir -p internal/ui/dist
	cp -a web/build/. internal/ui/dist/
	@test -f internal/ui/dist/200.html
	# SPA mode may not emit index.html; / should still load the fallback shell.
	@if [ ! -f internal/ui/dist/index.html ]; then cp internal/ui/dist/200.html internal/ui/dist/index.html; fi
	@echo "UI embedded assets ready in internal/ui/dist"

build:
	@test -f internal/ui/dist/200.html || { echo "missing embedded UI; run: make web"; exit 1; }
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
	@echo "Panel UI: http://127.0.0.1:8787/  (host via VBox NAT :8787)"
	@echo "Health:   http://127.0.0.1:8787/healthz"
	@echo "Site:     curl -H 'Host: example.com' http://127.0.0.1:8080/"

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

# Milestone 2 slice 1: socket-activated Go sample behind Caddy.
lab-app: build lab-dirs
	go build -o $(BIN_DIR)/spike-socket-go ./cmd/spike-socket-go
	go build -o $(BIN_DIR)/spike-m2-app ./cmd/spike-m2-app
	sudo mkdir -p /run/webooza/sites /var/lib/webooza
	sudo $(BIN_DIR)/spike-m2-app \
		-config internal/config/testdata/lab-app.yaml \
		-caddy-json $(CADDY_JSON) \
		-sample-binary $(BIN_DIR)/spike-socket-go \
		-disable-https
	@echo "--- wake-on-request ---"
	@sudo systemctl stop webooza-app-app-example-com.service 2>/dev/null || true
	@systemctl is-active --quiet webooza-app-app-example-com.socket || sudo systemctl start webooza-app-app-example-com.socket
	@echo -n "socket:  "; systemctl is-active webooza-app-app-example-com.socket || true
	@echo -n "service before: "; systemctl is-active webooza-app-app-example-com.service || true
	@curl -sf -H 'Host: app.example.com' http://127.0.0.1:8080/; echo
	@echo -n "service after:  "; systemctl is-active webooza-app-app-example-com.service || true

# Milestone 2 slice 2: app binary deploy + automatic rollback on probe failure.
lab-deploy: lab-app
	go build -o $(BIN_DIR)/spike-m2-deploy ./cmd/spike-m2-deploy
	sudo $(BIN_DIR)/spike-m2-deploy -domain app.example.com -binary $(BIN_DIR)/spike-socket-go

# Milestone 2 slice 3: cooperative idle-stop (WBZ_IDLE_TIMEOUT) + wake.
lab-idle: lab-app
	@echo "--- idle-stop (lab-app.yaml idle_timeout=3s) ---"
	@echo -n "service after wake: "; systemctl is-active webooza-app-app-example-com.service || true
	@sleep 4
	@echo -n "socket after idle:  "; systemctl is-active webooza-app-app-example-com.socket || true
	@echo -n "service after idle: "; systemctl is-active webooza-app-app-example-com.service || true
	@test "$$(systemctl is-active webooza-app-app-example-com.socket)" = active
	@test "$$(systemctl is-active webooza-app-app-example-com.service)" = inactive
	@echo "--- wake after idle ---"
	@curl -sf -H 'Host: app.example.com' http://127.0.0.1:8080/; echo
	@echo -n "service after wake: "; systemctl is-active webooza-app-app-example-com.service || true
	@test "$$(systemctl is-active webooza-app-app-example-com.service)" = active
	@echo "ok: idle-stop + wake verified"

# Milestone 2 slice 4: per-site cron timer + run-now.
lab-cron: build lab-dirs
	go build -o $(BIN_DIR)/spike-m2-cron ./cmd/spike-m2-cron
	sudo $(BIN_DIR)/spike-m2-cron -domain app.example.com -job lab-tick

# Milestone 2: Node sample on app.example.com (wake, idle-stop, deploy+rollback).
lab-node: build lab-dirs
	@command -v node >/dev/null || { echo "lab-node: node is required (apt install nodejs)"; exit 1; }
	@test -f $(NODE_SAMPLE) || { echo "lab-node: missing $(NODE_SAMPLE)"; exit 1; }
	chmod +x $(NODE_SAMPLE)
	go build -o $(BIN_DIR)/spike-m2-app ./cmd/spike-m2-app
	go build -o $(BIN_DIR)/spike-m2-deploy ./cmd/spike-m2-deploy
	sudo mkdir -p /run/webooza/sites /var/lib/webooza
	sudo $(BIN_DIR)/spike-m2-app \
		-config internal/config/testdata/lab-app.yaml \
		-caddy-json $(CADDY_JSON) \
		-sample-binary $(NODE_SAMPLE) \
		-disable-https
	@echo "--- wake-on-request (node) ---"
	@sudo systemctl stop webooza-app-app-example-com.service 2>/dev/null || true
	@systemctl is-active --quiet webooza-app-app-example-com.socket || sudo systemctl start webooza-app-app-example-com.socket
	@echo -n "socket:  "; systemctl is-active webooza-app-app-example-com.socket || true
	@echo -n "service before: "; systemctl is-active webooza-app-app-example-com.service || true
	@curl -sf -H 'Host: app.example.com' http://127.0.0.1:8080/ | tee /tmp/webooza-lab-node-wake.txt; echo
	@grep -q 'webooza-spike-b-node ok' /tmp/webooza-lab-node-wake.txt
	@echo -n "service after:  "; systemctl is-active webooza-app-app-example-com.service || true
	@echo "--- idle-stop (lab-app.yaml idle_timeout=3s) ---"
	@sleep 4
	@echo -n "socket after idle:  "; systemctl is-active webooza-app-app-example-com.socket || true
	@echo -n "service after idle: "; systemctl is-active webooza-app-app-example-com.service || true
	@test "$$(systemctl is-active webooza-app-app-example-com.socket)" = active
	@test "$$(systemctl is-active webooza-app-app-example-com.service)" = inactive
	@echo "--- wake after idle ---"
	@curl -sf -H 'Host: app.example.com' http://127.0.0.1:8080/; echo
	@echo -n "service after wake: "; systemctl is-active webooza-app-app-example-com.service || true
	@test "$$(systemctl is-active webooza-app-app-example-com.service)" = active
	@echo "--- deploy + rollback (node) ---"
	sudo $(BIN_DIR)/spike-m2-deploy \
		-domain app.example.com \
		-binary $(NODE_SAMPLE) \
		-expect 'webooza-spike-b-node ok'
	@echo "ok: node wake + idle-stop + deploy/rollback verified"

# Milestone 2: proxyd idle fallback (non-LISTEN_FDS backend + --exit-idle-time).
lab-proxyd: build lab-dirs
	go build -o $(BIN_DIR)/spike-socket-backend ./cmd/spike-socket-backend
	go build -o $(BIN_DIR)/spike-m2-app ./cmd/spike-m2-app
	sudo mkdir -p /run/webooza/sites /var/lib/webooza
	sudo $(BIN_DIR)/spike-m2-app \
		-config internal/config/testdata/lab-proxyd.yaml \
		-caddy-json $(CADDY_JSON) \
		-sample-binary $(BIN_DIR)/spike-socket-backend \
		-disable-https
	@echo "--- wake-on-request (proxyd) ---"
	@sudo systemctl stop webooza-app-app-example-com.service 2>/dev/null || true
	@systemctl is-active --quiet webooza-app-app-example-com.socket || sudo systemctl start webooza-app-app-example-com.socket
	@systemctl is-active --quiet webooza-app-app-example-com-backend.service || sudo systemctl start webooza-app-app-example-com-backend.service
	@echo -n "backend: "; systemctl is-active webooza-app-app-example-com-backend.service || true
	@echo -n "proxyd before: "; systemctl is-active webooza-app-app-example-com.service || true
	@curl -sf -H 'Host: app.example.com' http://127.0.0.1:8080/ | tee /tmp/webooza-lab-proxyd.txt; echo
	@grep -q 'webooza-spike-b-proxy-backend ok' /tmp/webooza-lab-proxyd.txt
	@echo -n "proxyd after:  "; systemctl is-active webooza-app-app-example-com.service || true
	@echo "--- idle-stop proxyd (exit-idle-time=3s) ---"
	@sleep 4
	@echo -n "socket after idle: "; systemctl is-active webooza-app-app-example-com.socket || true
	@echo -n "proxyd after idle: "; systemctl is-active webooza-app-app-example-com.service || true
	@echo -n "backend after idle: "; systemctl is-active webooza-app-app-example-com-backend.service || true
	@test "$$(systemctl is-active webooza-app-app-example-com.socket)" = active
	@test "$$(systemctl is-active webooza-app-app-example-com.service)" = inactive
	@test "$$(systemctl is-active webooza-app-app-example-com-backend.service)" = active
	@echo "--- wake after idle ---"
	@curl -sf -H 'Host: app.example.com' http://127.0.0.1:8080/; echo
	@echo -n "proxyd after wake: "; systemctl is-active webooza-app-app-example-com.service || true
	@test "$$(systemctl is-active webooza-app-app-example-com.service)" = active
	@echo "ok: proxyd idle + wake verified"

# Milestone 2: long-running job-as-service.
lab-job: build lab-dirs
	@test -f $(JOB_SAMPLE) || { echo "lab-job: missing $(JOB_SAMPLE)"; exit 1; }
	chmod +x $(JOB_SAMPLE)
	go build -o $(BIN_DIR)/spike-m2-job ./cmd/spike-m2-job
	sudo $(BIN_DIR)/spike-m2-job -domain app.example.com -job lab-keep -script $(JOB_SAMPLE)
	@test -f /srv/sites/app.example.com/tmp/job-alive
	@echo "ok: long-running job marker present"
