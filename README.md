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
and compiled with Go 1.4. Earlier FreeBSD releases are not supported.

Getting Started
---------------
### VM with vagrant
To spin up a pre configured FreeBSD VM with [Vagrant](https://www.vagrantup.com)

Make sure you have [ansible](http://docs.ansible.com/intro_installation.html#getting-ansible) installed on the host system.

Then boot and provision the VM by running `$ vagrant up` in the root directory of this repository.
Run `$ vagrant ssh` to ssh into the machine. 
The code is mounted under `/vagrant`.

### Configuring the system

First, build Jetpack and install it system-wide or in-place. The
[INSTALL.md](INSTALL.md) document contains the installation
instructions.

You will obviously need a ZFS pool for Jetpack's datasets. By default,
Jetpack will create a `zroot/jetpack` dataset and mount it at
`/var/jetpack`. If your zpool is not named _zroot_, or if you prefer
different locations, these defaults can be modified in the
`jetpack.conf` file.

You will need a user and group to own the runtime status files and
avoid running the metadata service as root. If you stay with default
settings, the username and group should be `_jetpack`:

    pw useradd _jetpack -d /var/jetpack -s /usr/sbin/nologin

> **Note:** If you are upgrading from an earlier revision of Jetpack,
> you will need to change ownership of files and directories:
> `chgrp _jetpack /var/jetpack/pods/* /var/jetpack/images/*
> /var/jetpack/*/*/manifest && chmod 0440 /var/jetpack/*/*/manifest`

You will also need a network interface that the jails will use, and
this interface should have Internet access. By default, Jetpack uses
`lo1`, but this can be changed in the `jetpack.conf` file. To create
the interface, run the following command as root:

    ifconfig lo1 create inet 172.23.0.1/16

To have the `lo1` interface created at boot time, add the following
lines to `/etc/rc.conf`:

    cloned_interfaces="lo1"
    ipv4_addrs_lo1="172.23.0.1/16"

The main IP address of the interface will be used as the host
address. Remaining addresses within its IP range (in this case,
172.23.0.2 to 172.23.255.254) will be assigned to the pods. IPv6
is currently not supported.

The simplest way to provide internet access to the jails is to NAT the
loopback interface. A proper snippet of PF firewall configuration
would be:

    set skip on lo1
    nat pass on $ext_if from lo1:network to any -> $ext_if

where `$ext_if` is your external network interface. A more
sopihisticated setup can be desired to limit pods'
connectivity. In the long run, Jetpack will probably manage its own
`pf` anchor.

### Using Jetpack

Run `jetpack` without any arguments to see available commands. Use
`jetpack help COMMAND` to see detailed help on individual commands.

To initialize the ZFS datasets and directory structure, run `jetpack
init`.

To get a console, run:

    jetpack run 3ofcoins.net/freebsd.base

This will fetch our signing GPG key, then fetch the FreeBSD base ACI,
and finally run a pod and drop you into its console. After you exit
the shell, run `jetpack list` to see the pod, and `jetpack destroy
UUID` to remove id.

Run `jetpack images` to list available images.

You create pods from images, then run the pods:

    jetpack prepare 3ofcoins.net/freebsd-base

Note the pod UUID printed by the above command (no user-friendly pod
names yet) or get it from the pod list (run `jetpack list` to see the
list). Then run the pod:

    jetpack run $UUID

The above command will drop you into root console of the pod. After
you're finished, you can run the pod again. Once you're done with the
pod, you can destroy it:

    jetpack destroy $UUID

You can also look at the "showenv" example:

    make -C images/example.showenv
    jetpack prepare example/showenv
    jetpack run $UUID

To poke inside a pod that, like the "showenv" example, runs a useful
command instead of a console, use the `console` subcommand:

    jetpack console $UUID

Run `jetpack help` to see info on remaining available commands, and if
something needs clarification, create an issue at
https://github.com/3ofcoins/jetpack/ and ask the question. If
something is not clear, it's a bug in the documentation!

#### Running the Metadata Service

To start the metadata service as a daemon, run `jetpack mds`. The
metadata service will be started automatically if needed.

You can also start the service in foreground logging to standard
output, but you're going to need to use `sudo` or `su` to run it:

    # sudo -H -u _jetpack $LIBEXECDIR/mds

Building Images
---------------

See the [IMAGES.md](IMAGES.md) file for details.  Some example image
build scripts (including the published `3ofcoins.net/freebsd-base`
image) are provided in the `images/` directory.

Features, or The Laundry List
-----------------------------

 - Stage0
   - [x] Image import from ACI
   - [x] Image building
   - [x] Clone pod from image and run it
   - [ ] Full pod lifecycle (Stage0/Stage1 interaction)
   - [x] Multi-application pods
   - [x] Image discovery
 - Stage1
   - [x] Isolation via jails
   - [x] Volumes
   - [x] Multi-application pods
   - [ ] Firewall integration
   - [x] Metadata endpoint
   - [ ] Isolators
 - Stage2
   - [x] Main entry point execution
   - [x] Setting UID/GID
   - [x] Setting environment variables
   - [x] Event Handlers
   - [ ] Isolators
 - CLI
   - [X] Specify image/pod by name & labels, not only UUID
   - [x] Consistent options for specifying application options (CLI,
         JSON file)
 - General TODO
   - [x] Refactor the Thing/ThingManager/Host sandwich to use embedded
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
   - [ ] `/etc/rc.d/jetpack` (`/etc/rc.d/jetpack_` for individual
         pods?) to start pods at boot time, and generally
         manage them as services
   - [ ] Port to install Jetpack system-wide
   - If/when we get enough live runtime data to make it complicated,
     maybe a centralized indexed storage, like SQLite? This could also
     solve some locking issues for long-running processes…
