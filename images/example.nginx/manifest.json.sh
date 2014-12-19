#!/bin/sh
set -e

version="$(nginx -v 2>&1)"
version="${version##* nginx/}"

cat <<EOF
{
  "name": "example/nginx",
  "labels": [
    { "name": "version", "val": "${version}" }
  ],
  "app": {
    "exec": [
      "/usr/local/sbin/nginx"
    ],
    "user": "root",
    "group": "root",
    "ports": [
      {
        "name": "http",
        "protocol": "tcp",
        "port": 80
      }
    ]
  }
}
EOF
