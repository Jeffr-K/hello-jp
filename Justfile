set dotenv-load

go_cache := "/tmp/today-japanese-go-cache"

default: check

# Generate gRPC and gRPC-Gateway contracts.
proto:
	buf lint
	buf generate

# Format all Go source files.
fmt:
	GOCACHE={{go_cache}} go fmt ./...

# Run the complete test suite.
test:
	GOCACHE={{go_cache}} go test ./...

# Format and test before running the project.
check: fmt test

# Build the CLI into tmp/.
build:
	mkdir -p tmp
	GOCACHE={{go_cache}} go build -o /tmp/jp-api ./cmd/api
	GOCACHE={{go_cache}} go build -o /tmp/jp-agent ./cmd/agent
	GOCACHE={{go_cache}} go build -o /tmp/jp-batch ./cmd/batch

# Generate today's Japanese learning content.
run topic="운동과 관련된 일본어":
	GOCACHE={{go_cache}} go run ./cmd/agent -topic "{{topic}}"

# Alias for the content generation command.
generate topic="운동과 관련된 일본어":
	GOCACHE={{go_cache}} go run ./cmd/agent -topic "{{topic}}"

# Start the local learning web app.
serve:
	GOCACHE={{go_cache}} go run ./cmd/api

# Start PostgreSQL and the web app with Docker Compose.
docker-up:
	docker compose up -d db app

# Stop Compose services without deleting the database volume.
docker-down:
	docker compose down

# Generate content through the Compose app container.
docker-generate topic="운동과 관련된 일본어":
	 docker compose run --rm --entrypoint /app/agent app -topic "{{topic}}"

# Start Air hot reload development mode.
dev:
	GOCACHE={{go_cache}} air
