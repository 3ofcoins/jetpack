#!/bin/sh
set -e

version="$(freebsd-version -u | sed 's/-[A-Z][A-Z]*-p/./')"
arch="$(uname -m)"
timestamp="$(date -u '+%FT%TZ')"

cat <<EOF
{
  "acKind": "ImageManifest",
  "acVersion": "0.1.1",
  "name": "freebsd-base",
  "labels": [
    { "name": "version", "val": "${version}" },
    { "name": "os", "val": "freebsd" },
    { "name": "arch", "val": "${arch}" }
  ],
  "annotations": {
    "created": "${timestamp}"
  }
}
EOF
