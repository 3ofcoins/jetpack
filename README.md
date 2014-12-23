> **WARNING:** This software is new, experimental, and under heavy
> development. The documentation is lacking, if any. There are almost
> no tests. The CLI commands, on-disk formats, APIs, and source code
> layout can change in any moment. Do not trust it. Use it at your own
> risk.
>
> **You have been warned**

Jetpack
=======

Jetpack is an **experimental and incomplete** implementation of the
[App Container Specification](https://github.com/appc/spec) for
FreeBSD. It uses jails as isolation mechanism, and ZFS for layered
storage.

This document uses some language used in
[Rocket](https://github.com/coreos/rocket), the reference
implementation of the App Container Specification. While the
documentation will be expanded in the future, currently you need to be
familiar at least with Rocket's README to understand everything.

Features, or The Laundry List
-----------------------------

 - Stage0
   - [x] Image import from ACI
   - [x] Image building
   - [x] Clone container from image and run it
   - [ ] Full container lifecycle (Stage0/Stage1 interaction)
   - [ ] Multi-application containers
   - [ ] Image discovery
 - Stage1
   - [x] Isolation via jails
   - [ ] Volumes
   - [ ] Multi-application containers
   - [ ] Firewall integration
   - [ ] Metadata endpoint
   - [ ] Isolators
 - Stage2
   - [x] Main entry point execution
   - [x] Setting UID/GID
   - [x] Setting environment variables
   - [ ] Event Handlers
   - [ ] Isolators
 - CLI
   - [ ] Specify image/container by name & labels, not only UUID
   - [ ] Consistent options for specifying application options (CLI,
         JSON file)

Getting Started
---------------

> TODO

Building Images
---------------

> TODO

Container Life Cycle
--------------------

> TODO
