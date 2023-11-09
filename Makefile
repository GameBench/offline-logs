VERSION := "1.0"

build:
	go build -ldflags "-X main.version=$(VERSION)" -o gb-offline-logs
