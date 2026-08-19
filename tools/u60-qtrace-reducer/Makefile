.PHONY: all build check clean dry-run

OUTPUT := build/u60-qtrace-reducer
CONFIG := testdata/qtrace-original-25frames.cfg

all: check build

build:
	mkdir -p build
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags '-s -w' -o $(OUTPUT) .

check:
	@test -z "$$(gofmt -l *.go)" || { gofmt -l *.go; exit 1; }
	go test ./...
	go vet ./...

dry-run:
	go run . dry-run -config $(CONFIG) -out build/dry-run

clean:
	rm -rf build
