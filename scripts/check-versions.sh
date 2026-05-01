#!/bin/bash
# Check that all version files are in sync
# Run this before committing version bumps

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

# Get the canonical version from version.go
CANONICAL=$(grep 'Version = ' cmd/tl/version.go | sed 's/.*"\(.*\)".*/\1/')

if [ -z "$CANONICAL" ]; then
    echo -e "${RED}❌ Could not read version from cmd/tl/version.go${NC}"
    exit 1
fi

echo "Canonical version (from version.go): $CANONICAL"
echo ""

MISMATCH=0

check_version() {
    local file=$1
    local version=$2
    local description=$3

    if [ "$version" != "$CANONICAL" ]; then
        echo -e "${RED}❌ $description: $version (expected $CANONICAL)${NC}"
        MISMATCH=1
    else
        echo -e "${GREEN}✓ $description: $version${NC}"
    fi
}

MODULE=$(go list -m)
if [ "$MODULE" != "task-ledger" ]; then
    echo -e "${RED}❌ Go module path: $MODULE (expected task-ledger)${NC}"
    MISMATCH=1
else
    echo -e "${GREEN}✓ Go module path: $MODULE${NC}"
fi

if [ $MISMATCH -eq 1 ]; then
    echo -e "${RED}❌ Version mismatch detected!${NC}"
    echo ""
    echo "Manually update the mismatched files."
    exit 1
else
    echo -e "${GREEN}✓ All versions match: $CANONICAL${NC}"
fi
