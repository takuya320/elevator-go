.PHONY: help run build test vet lint fmt gen web check all

help:
	@echo "Targets:"
	@echo "  run    - go run . (:8080 で起動)"
	@echo "  build  - go build ./..."
	@echo "  test   - go test ./..."
	@echo "  vet    - go vet ./..."
	@echo "  lint   - golangci-lint run ./..."
	@echo "  fmt    - golangci-lint fmt ./..."
	@echo "  gen    - go generate ./... (OpenAPI 再生成)"
	@echo "  web    - cd web && pnpm run build"
	@echo "  check  - build + vet + test (完了報告前の確認)"
	@echo "  all    - web + gen + check"

run:
	go run .

build:
	go build ./...

test:
	go test ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./...

fmt:
	golangci-lint fmt ./...

gen:
	go generate ./...

web:
	cd web && pnpm run build

check: build vet test

all: web gen check
