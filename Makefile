test:
	@go test ./...

build:
	@go build -o wildgecu .

# lint-install:
# 	@mise install

# lint:
# 	@mise exec -- golangci-lint run ./...

# lint-fix:
# 	@mise exec -- golangci-lint run --fix ./...