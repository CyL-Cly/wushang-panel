#!/bin/bash

# Build the admin and user frontends from source and drop the artifacts into
# cmd/dashboard/{admin,user}-dist, which cmd/dashboard/main.go embeds.
#
# Unlike fetch-frontends.sh this does not download prebuilt releases: it builds
# from source, which carries the performance patches (route-level lazy
# loading, no catch-all vendor chunk).
#
# The admin frontend is vendored in this repository at ./admin-frontend.
# The user frontend is an optional local checkout; when it is absent this
# script leaves user-dist alone, and fetch-frontends.sh can supply the
# prebuilt release instead.
#
# The admin build uses npm (package-lock.json); the user frontend uses pnpm,
# pinned by its "packageManager" field and pnpm-lock.yaml.

set -euo pipefail

ROOT_DIR="$(realpath "$(dirname "${BASH_SOURCE[0]}")/..")"
ADMIN_SRC="${ADMIN_SRC:-$ROOT_DIR/admin-frontend}"
USER_SRC="${USER_SRC:-$ROOT_DIR/../nezha-dash-v2}"
NPM_REGISTRY="${NPM_REGISTRY:-https://registry.npmmirror.com}"

require_dir() {
  if [ ! -d "$1" ]; then
    echo "error: $1 not found" >&2
    exit 1
  fi
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "error: $1 is required but not installed" >&2
    exit 1
  fi
}

install_dist() {
  local src="$1" target="$2"
  # Keep the tracked .gitkeep so the embed directory still exists on a fresh
  # clone; only the built assets are replaced.
  rm -rf "${target:?}"/* "${target:?}"/.[!.]* 2>/dev/null || true
  cp -r "$src/dist"/. "$target"/
  touch "$target/.gitkeep"
  echo "installed $(basename "$target") from $src/dist"
}

require_dir "$ADMIN_SRC"
require_cmd node
require_cmd npm

echo "==> building admin frontend"
pushd "$ADMIN_SRC" >/dev/null
npm ci --no-audit --no-fund --registry="$NPM_REGISTRY"
npm run build
popd >/dev/null
install_dist "$ADMIN_SRC" "$ROOT_DIR/cmd/dashboard/admin-dist"

if [ -d "$USER_SRC" ]; then
  echo "==> building user frontend"
  require_cmd pnpm
  pushd "$USER_SRC" >/dev/null
  pnpm install --frozen-lockfile --registry="$NPM_REGISTRY"
  pnpm build
  popd >/dev/null
  install_dist "$USER_SRC" "$ROOT_DIR/cmd/dashboard/user-dist"
else
  echo "==> user frontend source not found at $USER_SRC; keeping existing user-dist"
  echo "    run script/fetch-frontends.sh to download the prebuilt release instead"
fi

echo "done; rebuild the dashboard binary to embed the new dists"
