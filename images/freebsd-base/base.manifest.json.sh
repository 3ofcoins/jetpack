#!/bin/sh
cat <<EOF 
{
  "acKind": "ImageManifest",
  "acVersion": "0.5.2",
  "name": "base",
  "labels": [
    { "name": "version", "value": "$1" },
    { "name": "os", "value": "freebsd" },
    { "name": "arch", "value": "amd64" }
  ],
  "annotations": [
    { "name": "authors", "value": "Maciej Pasternacki <maciej@3ofcoins.net>" },
    { "name": "homepage", "value": "https://github.com/3ofcoins/jetpack/" },
    { "name": "created", "value": "$(TZ=UTC date +%FT%TZ)" }
  ]
}
EOF
