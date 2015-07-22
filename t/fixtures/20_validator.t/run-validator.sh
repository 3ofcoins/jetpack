#!/bin/sh
set -x

jetpack run -app=ace-validator-sidekick $1 &
skpid=$!
sleep 0.5

jetpack run -app=ace-validator-main $1
mrv=$?

wait $skpid
skrv=$?

exit $(($mrv+$skrv))
