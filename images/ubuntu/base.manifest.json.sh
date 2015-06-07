#!/bin/sh
set -e

distrib_codename="$1"
eval "$(tar -xzOf "ubuntu-${distrib_codename}-core-cloudimg-i386-root.tar.gz" ./etc/lsb-release | tr A-Z a-z || :)"
. ./ubuntu-${distrib_codename}-build-info.txt

cat <<EOF
{
  "name": "base.ubuntu-${distrib_codename}",
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
