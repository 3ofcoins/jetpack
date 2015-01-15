#!/bin/sh
set -e

distrib_id="debian"
distrib_codename="$1"
distrib_release="$(tar -xjOf "rootfs.${distrib_codename}.txz" ./etc/debian_version)"
eval "$(tar -xJOf "rootfs.${distrib_codename}.txz" ./etc/lsb-release | tr A-Z a-z || :)"

cat <<EOF
{
  "name": "${distrib_id}",
  "labels": [
    { "name": "version", "value": "${distrib_release}" },
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
