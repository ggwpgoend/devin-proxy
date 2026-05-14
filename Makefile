.PHONY: build run test clean windows

build:
	go build -o devin-proxy ./cmd/devin-proxy

run: build
	./devin-proxy

test:
	go test ./... -v

clean:
	rm -f devin-proxy devin-proxy.exe devin-proxy.db .master_key

windows:
	GOOS=windows GOARCH=amd64 go build -o devin-proxy.exe ./cmd/devin-proxy
