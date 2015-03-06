#!/bin/sh
set -e

distrib_codename="$1"
eval "$(tar -xJOf "$1" ./etc/lsb-release | tr A-Z a-z || :)"
. ./build-info.txt

cat <<EOF
{
  "name": "ubuntu/cloudimg",
  "labels": [
    { "name": "version", "value": "${distrib_release}.${SERIAL}" },
    { "name": "codename", "value": "${distrib_codename}" },
    { "name": "os", "value": "linux" },
    { "name": "arch", "value": "i386" }
  ],
  "app": {
    "exec": ["/bin/login", "-f", "root"],
    "user": "root",
    "group": "root"
  }
}
EOF
