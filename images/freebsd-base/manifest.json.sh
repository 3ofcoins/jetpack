#!/bin/sh
set -e

version="$(freebsd-version -u | sed 's/-[A-Z][A-Z]*-p/./')"
arch="$(uname -m)"

cat <<EOF
{
  "name": "3ofcoins.net/freebsd-base",
  "labels": [
    { "name": "version", "value": "${version}" }
  ]
}
EOF
