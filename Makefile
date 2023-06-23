all: lint test build

lint:
	docker run \
		-t \
		--rm \
		-v ./:/app \
		-w /app \
		golangci/golangci-lint:v1.53.3 \
		golangci-lint run -v

test:
	go test -v ./...

build:
	docker build . -t pizza-oven:latest

