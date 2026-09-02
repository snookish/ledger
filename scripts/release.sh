#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "Usage: $0 [v0.1.0]"
  echo "  Creates and pushes an annotated tag. Version is auto-derived if not given."
  exit 1
}

derive_version() {
  local latest
  latest="$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")"
  # strip leading v and any suffix after -
  latest="${latest%%-*}"
  latest="${latest#v}"
  IFS='.' read -r major minor patch <<< "$latest"
  major=${major:-0}
  minor=${minor:-0}
  patch=${patch:-0}
  patch=$((patch + 1))
  echo "v${major}.${minor}.${patch}"
}

if [ $# -eq 0 ]; then
  VERSION="$(derive_version)"
  echo "Derived version: $VERSION"
elif [ $# -eq 1 ]; then
  if [ "$1" = "-h" ] || [ "$1" = "--help" ]; then
    usage
  fi
  VERSION="$1"
else
  usage
fi

if ! [[ "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-.*)?$ ]]; then
  echo "error: Version must look like v0.1.0, got $VERSION" >&2
  exit 1
fi

if [ -n "$(git status --porcelain)" ]; then
  echo "error: Working tree not clean. Please commit or stash first" >&2
  git status --porcelain
  exit 1
fi

BRANCH="$(git rev-parse --abbrev-ref HEAD)"
if [ "$BRANCH" != "main" ]; then
  echo "error: Must be on main, currently on $BRANCH" >&2
  exit 1
fi

if git rev-parse "$VERSION" >/dev/null 2>&1; then
  echo "error: Tag $VERSION already exists" >&2
  exit 1
fi

echo "==> Running Checks"
make vet
make test >/dev/null
echo "Checks Passed"

echo "==> Creating tag $VERSION"
git tag -a "$VERSION" -m "release $VERSION"

echo "==> Pushing Tag $VERSION to Origin"
git push origin "$VERSION"

echo "Done: $VERSION"
