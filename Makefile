build:
	@echo "Building..."
	@go build -o bin/go-htmx-chatter main.go

run: build
	@echo "Running..."
	@go run main.go