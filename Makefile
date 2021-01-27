linux:
	GOOS=linux GOARCH=amd64 go build -o asdiffconf-linux-amd64 ./

darwin:
	GOOS=darwin GOARCH=amd64 go build -o asdiffconf-darwin-amd64 ./

.PHONY: linux darwin
