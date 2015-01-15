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

Compatibility
-------------

Jetpack is developed and tested on an up-to-date FreeBSD 10.1 system,
and compiled with Go 1.4.

Getting Started
---------------

### Configuring the system

First, build JetPack and install it system-wide or in-place. The
`INSTALL.md` document contains the installation instructions.

You will obviously need a ZFS pool for the files. By default, JetPack
uses `zroot/jetpack` dataset mounted at `/var/jetpack`. These settings
can be changed in the `jetpack.conf` file.

You will also need a network interface that the jails will use, and
this interface should have Internet access. By default, Jetpack uses
`lo1` is used, but this can be changed in the `jetpack.conf` file. To
create the interface, run the following command as root:

    ifconfig lo1 create inet 172.23.0.1/16

To have the `lo1` interface created at boot time, add the following
lines to `/etc/rc.conf`:

    cloned_interfaces="lo1"
    ipv4_addrs_lo1="172.23.0.1/16"

The main IP address of the interface will be used as the host
address. Remaining addresses within its IP range (in this case,
172.23.0.2 to 172.23.255.254) will be assigned to the containers. IPv6
is currently not supported.

The simplest way to provide internet access to the jails is to NAT the
loopback interface. A proper snippet of PF firewall configuration
would be:

    set skip on lo1
    nat pass on $ext_if from lo1:network to any -> $ext_if

where `$ext_if` is your external network interface. A more
sopihisticated setup can be desired to limit containers'
connectivity. In the long run, JetPack will probably manage its own
`pf` anchor.

Currently, JetPack copies `/etc/resolv.conf` file from host to
containers. In future, it will be possible to configure custom DNS
servers (like a local unbound or dnsmasq).

### Using JetPack

Run `jetpack` without any arguments to see available commands.

To initialize the ZFS datasets and directory structure, run `jetpack
init`.

To see the general information, run `jetpack info`.

To build images, run `make` in the example image directories
(`/usr/local/share/examples/jetpack/*` in system-wide installation;
`./images/*` if you use in-place). You will probably want to build
`freebsd-base.release` image (pure FreeBSD-10.1 system from `base.txz`
distfile), and then `freebsd-base` (which runs `freebsd-update` on the
previous one). After that, you can build `example.showenv`, which runs
a basic smoke test (shows details of its container's inside).

You create containers from images, then run the containers:

    jetpack container create freebsd-base

Note the container UUID printed by the above command (no user-friendly
container names yet), then run the container:

    jetpack container $UUID run

The above command will drop you into root console of the
container. After you're finished, you can run the container
again. Once you're done with the container, you can destroy it:

    jetpack container $UUID destroy

You can also look at the "showenv" example:

    jetpack container create example/showenv
    jetpack container $UUID run

To poke inside a container that, like the "showenv" example, runs a
useful command instead of a console, use the `console` subcommand:

    jetpack container $UUID console

Run `jetpack help` to see info on remaining available commands, and if
something needs clarification, create an issue at
https://github.com/3ofcoins/jetpack/ and ask the question. If
something is not clear, it's a bug in the documentation!

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
   - [X] Specify image/container by name & labels, not only UUID
   - [ ] Consistent options for specifying application options (CLI,
         JSON file)
 - General TODO
   - [ ] Refactor the Thing/ThingManager/Host sandwich to use embedded
     fields
   - [ ] CLI-specified types.App fields for custom exec, maybe build
         parameters too?
   - [ ] Live, movable "tags" or "bookmarks", to mark e.g. latest
         version of an image without need to modify its
         manifest. Possible search syntax: `name@tag1,tag2,…`, where a
         tag is an ACName, so it may be also a key/value pair like
         `environment/production`.
         - [ ] Maybe some variant of tags that would be unique per
               name?
   - If/when we get enough live runtime data to make it complicated,
     maybe a centralized indexed storage, like SQLite? This could also
     solve some locking issues for long-running processes…

Building Images
---------------

> TODO

Container Life Cycle
--------------------

> TODO
