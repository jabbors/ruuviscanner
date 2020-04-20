all: build

build:
	go build

pi:
	GOOS=linux GOARCH=arm GOARM=5 go build -o ruuviscanner_linux-arm
