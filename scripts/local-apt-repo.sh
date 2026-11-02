#!/bin/bash

# Serves the debs in REPO_DIR (default /var/local/local-repo) as an apt repo, for update tests. Run as root.
# Usage: local-apt-repo.sh update [deb...]
#        local-apt-repo.sh teardown

set -eu

dir=$(realpath -m "${REPO_DIR:-/var/local/local-repo}")
list=/etc/apt/sources.list.d/local-repo.list

case "${1:-}" in
update)
    shift
    mkdir -p "$dir"
    echo "deb [trusted=yes] file:$dir ./" >"$list"
    [ $# -eq 0 ] || mv "$@" "$dir"
    cd "$dir"
    for deb in *.deb; do
        [ -e "$deb" ] || continue
        dpkg-deb -f "$deb"
        echo "Filename: ./$deb"
        echo "Size: $(stat -c %s "$deb")"
        echo "SHA256: $(sha256sum "$deb" | cut -d' ' -f1)"
        echo
    done >Packages
    apt-get update
    ;;
teardown)
    rm -rf "$dir" "$list"
    apt-get update
    ;;
*)
    echo "usage: $0 update [deb...]" >&2
    echo "       $0 teardown" >&2
    exit 1
    ;;
esac
