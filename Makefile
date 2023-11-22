VERSION := "0.1.0"
PREFIX := "gb-offline-logs"

build:
	GOOS=darwin GOARCH=amd64 go build -ldflags "-X main.version=$(VERSION)" -trimpath -o $(PREFIX)-darwin-amd64 main.go
	GOOS=linux GOARCH=amd64 go build -ldflags "-X main.version=$(VERSION)" -trimpath -o $(PREFIX)-linux-amd64 main.go
	GOOS=windows GOARCH=amd64 go build -ldflags "-X main.version=$(VERSION)" -trimpath -o $(PREFIX)-windows-amd64.exe main.go
