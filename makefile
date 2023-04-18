# 设置变量
BINARY_NAME=auxiliary

build:
	GOOS=linux GOARCH=amd64 go build -o bin/$(BINARY_NAME)_linux_amd64 main.go
	GOOS=windows GOARCH=amd64 go build -o bin/$(BINARY_NAME)_windows_amd64.exe main.go
	GOOS=darwin GOARCH=amd64 go build -o bin/$(BINARY_NAME)_darwin_amd64 main.go

clean:
	rm $(BINARY_NAME)_*

publish:
	./bin/auxiliary_darwin_amd64
	

.PHONY: build clean
