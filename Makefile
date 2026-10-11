BIN := xplan

.PHONY: all web build test vet run dev clean

all: build

# vite empties dist/ on every build; .gitkeep keeps go:embed happy on fresh clones.
web:
	cd web && npx tsc -b && npx vite build
	touch web/dist/.gitkeep

build: web
	go build -o $(BIN) ./cmd/xplan

test:
	go test ./...

vet:
	go vet ./...

run: build
	./$(BIN) serve --open

# Backend on :7788 plus the Vite dev server (proxies /api) on :5178.
dev:
	go run ./cmd/xplan serve --no-schedule & cd web && npx vite

clean:
	rm -f $(BIN)
	find web/dist -mindepth 1 ! -name .gitkeep -delete
