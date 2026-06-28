.PHONY: dev build test css clean

dev:
	@echo "Starting DB Studio development environment..."
	@npm run dev:css &
	@sleep 1
	@air

build:
	@echo "Building DB Studio..."
	@npm run build:css
	@go build -o bin/db-studio .

test:
	@go clean -testcache
	@go test ./... -v -count=1

css:
	@npm run build:css

clean:
	@rm -rf bin/
	@rm -f web/static/css/output.css
	@rm -rf tmp/
