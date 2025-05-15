.PHONY: build clean

build:
	go build -v

clean:
	rm -vf postgres-mcp-go
