#!/bin/sh
set -e
portname="$(ac-mdc app-annotation port)"
maketarget="$(ac-mdc app-annotation make)"
exec /usr/bin/make -C "/usr/ports/${portname}" "${maketarget}"
