#!/bin/sh
set -x

jetpack run $1:ace-validator-sidekick &
skpid=$!
sleep 0.5

jetpack run $1:ace-validator-main
mrv=$?

wait $skpid
skrv=$?

exit $(($mrv+$skrv))
