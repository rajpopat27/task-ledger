#!/bin/bash
set -e

echo "🔧 Building task-ledger from source..."
go build -o tl ./cmd/tl

echo "📦 Installing tl globally..."
sudo mv tl /usr/local/bin/tl
sudo chmod +x /usr/local/bin/tl

echo "✅ Verifying tl installation..."
tl version

echo "🎯 Initializing tl (non-interactive)..."
if [ ! -d .task-ledger/issues ] && [ ! -f .task-ledger/metadata.json ]; then
  tl init --quiet
else
  echo "tl already initialized"
fi

echo "📚 Installing Go dependencies..."
go mod download

echo "✨ Development environment ready!"
echo "Run 'tl ready' to see available tasks"
