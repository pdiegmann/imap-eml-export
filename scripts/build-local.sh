#!/bin/bash
set -e
VERSION=${1:-dev}
go build -ldflags "-X main.version=${VERSION}" -o imap-eml-export ./cmd/imap-eml-export
echo "Built: imap-eml-export (${VERSION})"
