#!/bin/bash
# This file is part of arduino-app-cli.
#
# SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
# SPDX-License-Identifier: GPL-3.0-or-later

# Turns a directory containing .deb files into a flat apt repository, by
# generating the package index (`Packages`, `Packages.gz`, `Release`) next to
# them.
#
# The index is generated here and not on the board, because `dpkg-dev` and
# `apt-utils` are not part of the board image, and installing them there would
# alter the very system we are about to test. Docker is already required to
# build the package, so it is used to provide those tools.

set -euo pipefail

if [ "$#" -ne 1 ]; then
  echo "usage: $0 <repo-dir>" >&2
  exit 1
fi

REPO_DIR="$(cd "$1" && pwd)"
IMAGE="arduino-app-cli-aptrepo"

# Cached by docker after the first run.
docker build -q -t "$IMAGE" - <<'DOCKERFILE'
FROM debian:bookworm
RUN apt-get update \
  && apt-get install -y --no-install-recommends dpkg-dev apt-utils \
  && rm -rf /var/lib/apt/lists/*
DOCKERFILE

# Run as the current user, so the generated files are not owned by root.
docker run --rm \
  --user "$(id -u):$(id -g)" \
  --volume "$REPO_DIR:/repo" \
  --workdir /repo \
  "$IMAGE" \
  sh -euc '
    rm -f Packages Packages.gz Release
    dpkg-scanpackages --multiversion . > Packages
    gzip --keep --force Packages
    # Written aside and moved in place, so that the index does not end up
    # describing a half-written copy of itself.
    apt-ftparchive release . > Release.tmp
    mv Release.tmp Release
  '

echo "Apt repository index generated in ${REPO_DIR}."
