#!/bin/bash
# This file is part of arduino-app-cli.
#
# SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
# SPDX-License-Identifier: GPL-3.0-or-later

# Generates a flat apt index (Packages, Packages.gz, Release) for the .debs in
# a dir. In docker: the board has no dpkg-dev/apt-utils.

set -euo pipefail

if [ "$#" -ne 1 ]; then
  echo "usage: $0 <repo-dir>" >&2
  exit 1
fi

REPO_DIR="$(cd "$1" && pwd)"
IMAGE="arduino-app-cli-aptrepo"

docker build -q -t "$IMAGE" - <<'DOCKERFILE'
FROM debian:bookworm
RUN apt-get update \
  && apt-get install -y --no-install-recommends dpkg-dev apt-utils \
  && rm -rf /var/lib/apt/lists/*
DOCKERFILE

# Output not owned by root.
docker run --rm \
  --user "$(id -u):$(id -g)" \
  --volume "$REPO_DIR:/repo" \
  --workdir /repo \
  "$IMAGE" \
  sh -euc '
    rm -f Packages Packages.gz Release
    dpkg-scanpackages --multiversion . > Packages
    gzip --keep --force Packages
    # Written aside, or it would index itself.
    apt-ftparchive release . > Release.tmp
    mv Release.tmp Release
  '

echo "Apt repository index generated in ${REPO_DIR}."
