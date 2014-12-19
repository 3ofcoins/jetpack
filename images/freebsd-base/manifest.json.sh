#!/bin/sh
set -e

version="$(freebsd-version -u | sed 's/-[A-Z][A-Z]*-p/./')"
arch="$(uname -m)"

cat <<EOF
{
  "name": "freebsd-base",
  "labels": [
    { "name": "version", "val": "${version}" },
    { "name": "os", "val": "freebsd" },
    { "name": "arch", "val": "${arch}" }
  ]
}
EOF
