install:
	go get github.com/Masterminds/glide
	go get github.com/golang/lint/golint
	go get github.com/gordonklaus/ineffassign
	go get github.com/client9/misspell/cmd/misspell
	go get github.com/jteeuwen/go-bindata/...
	glide install

templates-debug:
	go-bindata \
		-o ./pkg/templates/bindata.go \
		-pkg templates \
		-debug \
		-prefix "bindata" \
		bindata/...

templates-release:
	go-bindata \
		-o ./pkg/templates/bindata.go \
		-pkg templates \
		-prefix "bindata" \
		bindata/...

test:
	ineffassign ./
	misspell -error README.md ./pkg/**/*
	gofmt -d -s -e ./pkg/
	go test ./pkg/...

format:
	gofmt -d -w -s -e ./pkg/

build-linux: templates-release
	GOOS=linux GOARCH=amd64 go build -ldflags "-w" -o ./pavlik ./main.go

docker: build-linux
	docker build -t pavlik:1 .

.PHONY: install templates-debug templates-release test format build-linux docker
