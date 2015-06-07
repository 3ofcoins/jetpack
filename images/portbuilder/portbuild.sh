#!/bin/sh
set -e
portname="$(ac-mdc app-annotation port)"
exec /usr/bin/make -C "/usr/ports/${portname}" install
