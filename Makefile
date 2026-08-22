# SunPanel — bảng điều khiển quản trị máy chủ

BINARY      := sunpanel
CMD         := ./cmd/sunpanel
DIST        := dist
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE  ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

MODULE  := github.com/thanhtinz/sunpanel
LDFLAGS := -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.buildDate=$(BUILD_DATE)

# Các nền tảng được phát hành. Nhờ SQLite thuần Go, không cần cgo nên biên dịch
# chéo toàn bộ danh sách này từ một máy duy nhất.
PLATFORMS := linux/amd64 linux/arm64 linux/arm darwin/amd64 darwin/arm64 windows/amd64

.DEFAULT_GOAL := help

.PHONY: help
help: ## Hiển thị danh sách lệnh
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

.PHONY: frontend
frontend: ## Build giao diện vào web/dist
	cd frontend && npm ci && npm run build

.PHONY: frontend-dev
frontend-dev: ## Chạy máy chủ phát triển của giao diện
	cd frontend && npm run dev

.PHONY: build
build: ## Build binary cho nền tảng hiện tại
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) $(CMD)

.PHONY: all
all: frontend build ## Build cả giao diện lẫn binary

.PHONY: run
run: build ## Build rồi chạy panel
	./bin/$(BINARY) serve

.PHONY: test
test: ## Chạy toàn bộ test
	go test -race ./...

.PHONY: cover
cover: ## Chạy test kèm báo cáo độ phủ
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

.PHONY: lint
lint: ## Kiểm tra mã Go
	gofmt -l . | tee /dev/stderr | (! read)
	go vet ./...
	@command -v golangci-lint >/dev/null && golangci-lint run || \
		echo "bỏ qua golangci-lint (chưa cài)"

.PHONY: typecheck
typecheck: ## Kiểm tra kiểu của giao diện
	cd frontend && npm run typecheck

.PHONY: check
check: lint test ## Chạy lint và test

.PHONY: release
release: frontend ## Build binary cho mọi nền tảng vào dist/
	@rm -rf $(DIST) && mkdir -p $(DIST)
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; arch=$${platform#*/}; \
		out=$(DIST)/$(BINARY)-$$os-$$arch; \
		[ "$$os" = "windows" ] && out=$$out.exe; \
		echo "  build $$os/$$arch"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch \
			go build -trimpath -ldflags "$(LDFLAGS)" -o $$out $(CMD) || exit 1; \
	done
	@cd $(DIST) && sha256sum * > SHA256SUMS
	@ls -lh $(DIST)

# Ứng dụng máy tính là một mô-đun Go riêng vì nó cần cgo (WebKit/WebView2/WKWebView),
# còn binary panel phải giữ CGO_ENABLED=0 để biên dịch chéo được.
.PHONY: desktop
desktop: ## Build ứng dụng máy tính vào bin/sunpanel-desktop
	cd desktop && PKG_CONFIG_PATH=$(CURDIR)/desktop/pkgconfig:$$PKG_CONFIG_PATH \
		go build -trimpath -o $(CURDIR)/bin/sunpanel-desktop$(if $(filter windows,$(GOOS)),.exe,) .

.PHONY: desktop-test
desktop-test: ## Chạy test của ứng dụng máy tính
	cd desktop && PKG_CONFIG_PATH=$(CURDIR)/desktop/pkgconfig:$$PKG_CONFIG_PATH go test ./...

.PHONY: clean
clean: ## Xóa sản phẩm build
	rm -rf bin $(DIST) coverage.out web/dist/assets web/dist/index.html web/dist/favicon.svg

.PHONY: tidy
tidy: ## Dọn go.mod
	go mod tidy
