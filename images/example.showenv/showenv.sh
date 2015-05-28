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

Mounts:
EOF
mount

cat <<EOF

Files:
EOF
ls -la

echo

echo -n 'date.txt: '
cat /opt/data/date.txt

echo -n 'pre-start id: '
cat /opt/data/pre-start-id.txt

if test -f /opt/data/post-stop-id.txt ; then
    echo -n 'post-stop id: '
    cat /opt/data/post-stop-id.txt
else
    echo 'post-stop id: NONE (first run?)'
fi
