# Build the Go binary (assumes verbs.db is already generated)
build:
    go build -ldflags="-s -w" -o conj main.go

# Cross-compile a static Linux aarch64 binary if your second machine runs Linux
build-linux-arm64:
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o conj-linux-arm64 main.go

# Regenerate verbs.db from scratch using Python + verbecc
regen-db:
    uv venv .venv
    uv pip install --python .venv/bin/python verbecc
    ./.venv/bin/python build_db.py
