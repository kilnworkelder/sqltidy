.PHONY: build test vet fmt clean

BINARY := sqltidy

build:
	go build -o bin/$(BINARY) ./cmd/$(BINARY)

test:
	go test -count=1 ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

clean:
	rm -rf bin/
