.PHONY: build run test docker-build docker-run lint clean

build:
	go build -o bin/server ./cmd/server

run: build
	./bin/server

test:
	go test ./... -v -count=1

docker-build:
	docker build -t webpage-analyzer .

docker-run: docker-build
	docker run -p 8080:8080 webpage-analyzer

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/
