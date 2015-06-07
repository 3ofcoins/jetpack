Clean Room Port Builder
=======================

Mount `ports` tree (read-only), optionally `distfiles` dir (r/w),
`packages` dir for dependency reuse (r/w), and `dbdir` for port
options (`/var/db/ports`). Perform builds in a clean environment to
catch missing dependencies.

    jetpack pod create -run \
      -v ports:/usr/ports \
      -v distfiles:/usr/ports/distfiles \
      -v packages:/srv/portbuilder.packages \
      3ofcoins.net/port-builder \
      -a port=graphics/imp

Fix, retry from the failure:

    jetpack pod UUID run

Poke around to make sure stuff works:

    jetpack pod UUID console
