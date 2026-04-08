#!/usr/bin/env bash
# ---------------------------------------------------------------
# release.sh — Create a semver release tag (local + optional push)
#
# Usage:
#   ./scripts/release.sh <major|minor|patch> [--push] [--remote]
#
# Options:
#   --push    Push the tag to origin after creating it
#   --remote  Push tag AND trigger the GitHub Actions workflow
#             (equivalent to --push; CI fires on tag push)
#
# The script:
#   1. Reads the current version from internal/version/version.go
#   2. Bumps major, minor, or patch
#   3. Updates version.go with the new version
#   4. Commits the version bump
#   5. Creates an annotated tag v<new-version>
#   6. Optionally pushes tag + commit to origin
#
# Examples:
#   ./scripts/release.sh patch          # 0.1.0 -> 0.1.1 (local only)
#   ./scripts/release.sh minor --push   # 0.1.1 -> 0.2.0 (push to origin)
#   ./scripts/release.sh major --remote # 0.2.0 -> 1.0.0 (push, triggers CI)
# ---------------------------------------------------------------
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
VERSION_FILE="${ROOT_DIR}/internal/version/version.go"

# ---------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------
die()  { echo "ERROR: $*" >&2; exit 1; }
info() { echo "==> $*"; }

# ---------------------------------------------------------------
# Parse args
# ---------------------------------------------------------------
BUMP_TYPE=""
PUSH=false

for arg in "$@"; do
  case "${arg}" in
    major|minor|patch) BUMP_TYPE="${arg}" ;;
    --push|--remote)   PUSH=true ;;
    -h|--help)
      sed -n '2,/^# ----/{ /^# ----/d; s/^# \?//; p }' "$0"
      exit 0
      ;;
    *) die "Unknown argument: ${arg}" ;;
  esac
done

[[ -n "${BUMP_TYPE}" ]] || die "Usage: $0 <major|minor|patch> [--push|--remote]"

# ---------------------------------------------------------------
# Read current version
# ---------------------------------------------------------------
CURRENT=$(grep -oP 'Version\s*=\s*"\K[^"]+' "${VERSION_FILE}")
[[ -n "${CURRENT}" ]] || die "Could not read version from ${VERSION_FILE}"

IFS='.' read -r MAJOR MINOR PATCH <<< "${CURRENT}"

# ---------------------------------------------------------------
# Bump
# ---------------------------------------------------------------
case "${BUMP_TYPE}" in
  major) MAJOR=$((MAJOR + 1)); MINOR=0; PATCH=0 ;;
  minor) MINOR=$((MINOR + 1)); PATCH=0 ;;
  patch) PATCH=$((PATCH + 1)) ;;
esac

NEW_VERSION="${MAJOR}.${MINOR}.${PATCH}"
TAG="v${NEW_VERSION}"

info "Bumping: ${CURRENT} -> ${NEW_VERSION} (${BUMP_TYPE})"

# ---------------------------------------------------------------
# Check for clean working tree (except the version file change we're about to make)
# ---------------------------------------------------------------
if ! git -C "${ROOT_DIR}" diff --quiet HEAD 2>/dev/null; then
  die "Working tree has uncommitted changes. Commit or stash them first."
fi

# ---------------------------------------------------------------
# Check tag doesn't already exist
# ---------------------------------------------------------------
if git -C "${ROOT_DIR}" rev-parse "${TAG}" >/dev/null 2>&1; then
  die "Tag ${TAG} already exists"
fi

# ---------------------------------------------------------------
# Update version.go
# ---------------------------------------------------------------
sed -i "s/Version = \"${CURRENT}\"/Version = \"${NEW_VERSION}\"/" "${VERSION_FILE}"
info "Updated ${VERSION_FILE}"

# ---------------------------------------------------------------
# Commit + tag
# ---------------------------------------------------------------
git -C "${ROOT_DIR}" add "${VERSION_FILE}"
git -C "${ROOT_DIR}" commit -m "chore: bump version to ${NEW_VERSION}"
git -C "${ROOT_DIR}" tag -a "${TAG}" -m "Release ${TAG}"

info "Created commit and tag ${TAG}"

# ---------------------------------------------------------------
# Push
# ---------------------------------------------------------------
if [[ "${PUSH}" == "true" ]]; then
  info "Pushing commit and tag to origin..."
  git -C "${ROOT_DIR}" push origin HEAD
  git -C "${ROOT_DIR}" push origin "${TAG}"
  info "Pushed. GitHub Actions will handle the release."
else
  info "Tag created locally. To push:"
  echo "  git push origin HEAD && git push origin ${TAG}"
fi

info "Done!"
