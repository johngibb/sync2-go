.PHONY: install test

install:
	go install ./...

test:
	go test -race .
