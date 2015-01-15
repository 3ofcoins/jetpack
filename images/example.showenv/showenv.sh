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

date > /opt/data/date.txt

cat <<EOF

Files:
EOF
ls -la

echo
cat /opt/data/date.txt
