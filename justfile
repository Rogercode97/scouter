default:
    @just --list

# Run the analyzer
run target="":
    go run ./cmd/scouter {{target}}

# Build the binary
build:
    go build -o bin/scouter ./cmd/scouter

# Run tests
test:
    go test -v ./...

# Format code
fmt:
    go fmt ./...

# Sincroniza los cambios a GitHub
sync msg="chore: sync repository via just":
    git add .
    git commit -m "{{msg}}" || true
    git push
