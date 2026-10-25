#!/bin/bash

set -euo pipefail

ROOT_DIR="$(realpath "$(dirname "${BASH_SOURCE[0]}")/..")"
TEMPLATES_FILE="$ROOT_DIR/service/singleton/frontend-templates.yaml"

download_and_extract() {
  local repository="$1"
  local version="$2"
  local targetDir="$3"
  local TMP_DIR

  TMP_DIR="$(mktemp -d)"

  echo "Downloading from repository: $repository, version: $version"

  pushd "$TMP_DIR" || exit

  curl -L -o "dist.zip" "$repository/releases/download/$version/dist.zip"

  unzip -q dist.zip
  # Replace the contents but keep the tracked .gitkeep so the embed directory
  # still exists on a fresh clone (mirrors install_dist in build-frontends.sh).
  if [ -e "$targetDir" ]; then
    rm -rf "${targetDir:?}"/* "${targetDir:?}"/.[!.]* 2>/dev/null || true
    cp -r dist/. "$targetDir"/
    touch "$targetDir/.gitkeep"
  else
    mv dist "$targetDir"
  fi

  rm "dist.zip"
  popd || exit
}

count=$(yq eval '. | length' "$TEMPLATES_FILE")

for i in $(seq 0 $(("$count"-1))); do
  path=$(yq -r ".[$i].path" "$TEMPLATES_FILE")
  repository=$(yq -r ".[$i].repository" "$TEMPLATES_FILE")
  version=$(yq -r ".[$i].version" "$TEMPLATES_FILE")

  if [[ -n $path && -n $repository && -n $version ]]; then
    download_and_extract "$repository" "$version" "$ROOT_DIR/cmd/dashboard/$path"
  fi
done
