#!/bin/sh
set -e

if [ $# -lt 3 ]; then
    echo "Usage: $0 RELEASE CODENAME ARCH" >&2
    exit 1
fi

test -f build-info.txt
. ./build-info.txt

cat <<EOF
{
  "name": "ubuntu-cloudimg-base",
  "labels": [
    { "name": "version", "value": "$1.${SERIAL}" },
    { "name": "codename", "value": "$2" },
    { "name": "os", "value": "linux" },
    { "name": "arch", "value": "$3" }
  ],
  "app": {
    "exec": ["/bin/login", "-f", "root"],
    "user": "root",
    "group": "root"
  }
}
EOF
