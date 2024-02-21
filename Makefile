build:
	@echo "Building..."
	@go build -o bin/go-htmx-chatter

run: build
	@echo "Running..."
	@go run .