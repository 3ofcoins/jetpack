#!/bin/sh
set -x

idfile="$(mktemp -t jetpack.validator)"
valdir="$(dirname "$0")"

# TODO: prepare vs run, accept all flags
jetpack prepare -saveid="$idfile" -f "$valdir/pod-manifest.json"
jetpack run -destroy `cat $idfile`
