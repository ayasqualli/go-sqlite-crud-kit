.PHONY: run test fmt vet check demo

run:
	go run ./cmd/demo-api

test:
	go test ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

vet:
	go vet ./...

check:
	@test -z "$$(gofmt -l $$(find . -name '*.go' -not -path './vendor/*'))" || (echo 'Run make fmt first'; exit 1)
	go vet ./...
	go test -race ./...

demo:
	./scripts/demo.sh
