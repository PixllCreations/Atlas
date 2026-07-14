.DEFAULT_GOAL := help

WEB := ./web

.PHONY: help up down logs cluster cluster-down test web-dev require-env

help:
	@echo "Atlas (public by default)"
	@echo ""
	@echo "  up            k3d cluster + Atlas + Cloudflare Tunnel"
	@echo "  down          stop stack and delete k3d cluster"
	@echo "  logs          follow api / postgres / tunnel logs"
	@echo "  cluster       create or refresh k3d + kubeconfig"
	@echo "  cluster-down  delete k3d cluster"
	@echo "  test          run Go tests"
	@echo "  web-dev       Vite dev server (API must be on :8080)"

require-env:
	@if [ ! -f .env ]; then \
		echo "Copy .env.example to .env and set CLOUDFLARE_TUNNEL_TOKEN." >&2; \
		exit 1; \
	fi
	@set -a; . ./.env; set +a; \
	if [ -d "$$HOME/.kube/config" ]; then \
		echo "$$HOME/.kube/config is a directory — run: rmdir $$HOME/.kube/config" >&2; \
		exit 1; \
	fi; \
	if [ ! -f "$$HOME/.kube/config" ] || [ ! -f "$$HOME/.kube/atlas-docker.yaml" ]; then \
		echo "Missing kubeconfig — run: make cluster" >&2; \
		exit 1; \
	fi; \
	if [ -z "$$CLOUDFLARE_TUNNEL_TOKEN" ]; then \
		echo "Set CLOUDFLARE_TUNNEL_TOKEN in .env (see README → Cloudflare)." >&2; \
		exit 1; \
	fi; \
	if ! docker network inspect "$${ATLAS_K3D_NETWORK:-k3d-atlas}" >/dev/null 2>&1; then \
		echo "k3d network missing — run: make cluster" >&2; \
		exit 1; \
	fi

cluster:
	bash hack/cluster-up.sh

cluster-down:
	bash hack/cluster-down.sh

up: cluster require-env
	docker compose up -d --build

down:
	docker compose down
	$(MAKE) cluster-down

logs:
	docker compose logs -f api postgres tunnel

test:
	go test ./...

web-dev:
	cd $(WEB) && npm install && npm run dev
