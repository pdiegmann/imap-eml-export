#!/bin/bash
# release.sh – bump the semver version and create a signed git tag.
#
# Usage:  ./scripts/release.sh [major|minor|patch]
#
# The semver component to increment defaults to "patch".
# Example: if the latest tag is v0.1.1 then
#   ./scripts/release.sh          → v0.1.2
#   ./scripts/release.sh minor    → v0.2.0
#   ./scripts/release.sh major    → v1.0.0
set -euo pipefail

PART="${1:-patch}"

case "$PART" in
  major|minor|patch) ;;
  *)
    echo "error: invalid semver part '$PART'. Use major, minor, or patch." >&2
    exit 1
    ;;
esac

# Find the latest semver tag (e.g. v1.2.3 or 1.2.3).
LATEST=$(git tag --list 'v*' | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | sort -V | tail -n 1)

if [ -z "$LATEST" ]; then
  # No existing tag – start from v0.0.0 and apply the bump below.
  MAJOR=0; MINOR=0; PATCH=0
else
  # Strip leading 'v'.
  VERSION="${LATEST#v}"
  IFS='.' read -r MAJOR MINOR PATCH <<< "$VERSION"
fi

case "$PART" in
  major) MAJOR=$((MAJOR + 1)); MINOR=0; PATCH=0 ;;
  minor) MINOR=$((MINOR + 1)); PATCH=0 ;;
  patch) PATCH=$((PATCH + 1)) ;;
esac

NEW_TAG="v${MAJOR}.${MINOR}.${PATCH}"

echo "Current version : ${LATEST:-<none>}"
echo "Bumping         : $PART"
echo "New tag         : $NEW_TAG"

# Confirm before tagging.
read -r -p "Create and push tag $NEW_TAG? [y/N] " CONFIRM
case "$CONFIRM" in
  [yY][eE][sS]|[yY]) ;;
  *)
    echo "Aborted."
    exit 0
    ;;
esac

git tag -a "$NEW_TAG" -m "Release $NEW_TAG"
git push origin "$NEW_TAG"
echo "Tagged and pushed $NEW_TAG"
