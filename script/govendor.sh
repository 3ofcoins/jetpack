#!/bin/sh
set -e

repo_root="$(git rev-parse --show-toplevel)"
subdir="$(pwd)"
subdir="${subdir#$repo_root}"

export GOPATH="$(make -C "${repo_root}" -V gopath)"
cd "${GOPATH}/src/$(make -C "${repo_root}" -V gopkg)${subdir}"
exec govendor "${@}"
