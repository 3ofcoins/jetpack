#!/bin/sh
set -e -x

install -d -o root -g wheel -m 0755 $1/rootfs
tar -C $1/rootfs -xf -

cat <<EOF >$1/manifest
{
  "acKind": "ImageManifest",
  "acVersion": "0.5.2",
  "name": "freebsd-base/release",
  "labels": [
    { "name": "version", "value": "10.1" },
    { "name": "os", "value": "freebsd" },
    { "name": "arch", "value": "amd64" }
  ],
  "annotations": [
    { "name": "authors", "value": "Maciej Pasternacki <maciej@3ofcoins.net>" },
    { "name": "homepage", "value": "https://github.com/3ofcoins/docker/" },
    { "name": "created", "value": "$(TZ=UTC date +%FT%TZ)" }
  ]
}
EOF

chown root:wheel $1/manifest
chmod 0444 $1/manifest

tar -C $1 -cvf - manifest rootfs
