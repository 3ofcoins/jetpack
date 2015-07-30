Installing Jetpack
==================

At the moment, there is no port, package, or precompiled binaries for
Jetpack. It needs to be compiled from source.

Prerequisites
-------------

 - FreeBSD OS (developed and tested on 10.1 with current updates)
 - Git (to check out this repository)
 - Go (developed and tested on Go 1.4)
 
 To install prerequisites, run:

    # pkg install go git

Building
--------

To build Jetpack, simply run `make` in this directory. This will build
the binary configured for the `/usr/local` installation prefix; to use
a different installation prefix, it must be provided as a parameter,
e.g. `make prefix=/opt/jetpack`.

If you don't intend to install Jetpack and only use it in-place,
prefix is irrelevant.

Installation
------------

To install Jetpack system-wide, into a chosed installation prefix
(`/usr/local` by default), compile the software, and then run `make
install`. Software may be compiled as a regular user, but (most of the
time) needs to be installed as root:

    $ make
    $ sudo make install

Then copy the `$PREFIX/etc/jetpack.conf.sample` as
`$PREFIX/etc/jetpack.conf`, edit it to your liking, and go to the
_Getting Started_ section of the README file.

### Staging Directory

If you need to install into staging directory (e.g. to build a binary
tarball or some kind of a package), add the `DESTDIR` parameter. The
following will install files compiled for `/usr/local` prefix into
`destroot` subdirectory of the current directory, without requiring
root access:

    $ make install DESTDIR=`pwd`/destroot

Running Jetpack without installation
------------------------------------

After building, use script `script/jetpack` in the repository root to
run Jetpack without installing. It can be symlinked to a directory in
`$PATH`, or called with a script that uses `sudo` to run it as root.
To run the metadata service (in foreground), add yourself to the
`_jetpack` group, and run `script/mds`.

When running Jetpack in-place, the configuration file is
`jetpack.conf` in the main source directory (next to this file). Copy
`jetpack.conf.sample` to `jetpack.conf` and edit to your liking.
