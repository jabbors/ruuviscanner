all: build

build:
	go build

arm32:
	GOOS=linux GOARCH=arm GOARM=6 CGO_ENABLED=0 go build -ldflags "-s -w" -o ruuviscanner.linux-arm32

arm64:
	GOOS=linux GOARCH=arm64 GOARM=6 CGO_ENABLED=0 go build -ldflags "-s -w" -o ruuviscanner.linux-arm64

amd64:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-s -w" -o ruuviscanner.linux-amd64

release: arm32 arm64 amd64
