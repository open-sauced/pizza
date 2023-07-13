ROOT_DIR:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

all: lint test build

lint:
	docker run \
		-t \
		--rm \
		-v "${ROOT_DIR}/:/app" \
		-w /app \
		golangci/golangci-lint:v1.53.3 \
		golangci-lint run -v

test:
	go test -v ./...

run:
	go run main.go

build:
	docker build . -t pizza-oven:latest

setup-test-env:
	./hack/setup.sh

teardown-test-env:
	kind delete cluster --name opensauced-pizza
