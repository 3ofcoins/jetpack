Installing JetPack
==================

At the moment, there is no port, package, or precompiled binaries for
JetPack. It needs to be compiled from source.

Prerequisites
-------------

 - FreeBSD OS (developed and tested on 10.1 with current updates)
 - Git (to check out this repository)
 - Go (developed and tested on Go 1.4)
 - [gb](http://getgb.io/)
 
 To install prerequisites, run:

    # pkg install go git

Then set up your `$GOPATH` and `$GOBIN`, and run:

    $ go get github.com/constabulary/gb/...

Installing system-wide
----------------------

To install JetPack system-wide, choose installation prefix
(`/usr/local` is suggested), compile the software, and
install. Software may be compiled as a regular user, but (most of the
time) needs to be installed as root:

    $ make PREFIX=/usr/local
    $ sudo make install PREFIX=/usr/local

Then copy the `$PREFIX/etc/jetpack.conf.sample` as
`$PREFIX/etc/jetpack.conf`, edit it to your liking, and go to the
_Getting Started_ section of the README file.

Look into `$PREFIX/share/examples/jetpack` for example image
Makefiles, and to `$PREFIX/share/jetpack/jetpack.image.mk` for the
details on the used macros.

### Staging Directory

If you need to install into staging directory (e.g. to build a binary
tarball or some kind of a package), add the `DESTDIR` parameter. The
following will install files compiled for `/usr/local` prefix into
`destroot` subdirectory of the current directory, without requiring
root access:

    $ make install PREFIX=/usr/local DESTDIR=`pwd`/destroot

Using in-place
--------------

For ongoing development, it is more convenient to compile JetPack and
use it from the source checkout. If the `PREFIX` variable is not
provided to `make`, binaries can be run without a system-wide
installation.

> Side note: binaries compiled to run in-place can't be installed
> system-wide without recompilation. Paths to helper binaries,
> configuration defaults, and others are compiled into the binaries,
> based on the `PREFIX` parameter (or lack of it) passed to
> Make. The Makefile includes protection, and `make install` compares
> its prefix to prefix of recent `make`.

To use binaries in-place, simply run `make` and run `./bin/jetpack`
(it still needs to run as root to manipulate jails and ZFS datasets. I
have a `$HOME/bin/jetpack` script in my path that does
`exec sudo ~/Projects/jetpack/bin/jetpack "${@}"`, so I can simply run
`jetpack whatever`).

To customize the configuration, copy the `jetpack.conf.sample` as
`jetpack.conf` and edit it. Image Makefiles are in `images/`
subdirectory of this repository, and the `jetpack.image.mk` include is
in `share/`.
