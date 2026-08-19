.PHONY: check test build dry-run

check: test build

test:
	go test ./...

build:
	mkdir -p build
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags='-s -w' -o build/u60-nr-crossband-probe .

dry-run:
	go run . dry-run \
		-config ../../gossh/app/service/embed/qtrace.cfg \
		-out /tmp/u60-nr-crossband-dry-run \
		-target-band 41 \
		-target-arfcn 504990
