#!/bin/sh
set -x

idfile="$(mktemp -t jetpack.validator)"
valdir="$(dirname "$0")"
jetpack prepare -saveid="$idfile" -f "$valdir/pod-manifest.json"
"$valdir/run-validator.sh" `cat $idfile`
rv=$?
jetpack destroy `cat $idfile`
rm $idfile
exit $rv
