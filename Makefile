build-win:
	GOOS=windows GOARCH=amd64 go build -o dedecms-checker-win ./cmd/main.go

build-linux:
	GOOS=linux GOARCH=amd64 go build -o dedecms-checker ./cmd/main.go

build-mac:
	GOOS=darwin GOARCH=amd64 go build -o dedecms-checker-mac ./cmd/main.go


build: build-win build-linux build-mac
