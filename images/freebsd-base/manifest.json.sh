#!/bin/sh
set -e

version="$(freebsd-version -u | sed -e 's/-[A-Z][A-Z]*-p/./' -e 's/-RELEASE$/.0/')"
arch="$(uname -m)"

cat <<EOF
{
  "name": "3ofcoins.net/freebsd-base",
  "labels": [
    { "name": "version", "value": "${version}" }
  ]
}
EOF
