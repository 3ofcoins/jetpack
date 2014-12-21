#!/bin/sh
set -e

cat <<EOF
Program: $0
Args: $*
I am: $(id)
Work dir: $(pwd)
Hostname: $(hostname)

Environment:
EOF
env | sort

cat <<EOF

Files:
EOF
ls -la . /opt/data/
