Jetpack Images
==============

Fetching pre-made images
------------------------

Images can be imported from files or URLs, using the _fetch_ command:

    jetpack fetch ACI

ACI can be a path, an URL, or a name for discovery.

Building derivative images
--------------------------

An existing image can be used to build a derivative image. The
low-level mechanism is the _build_ command:

    jetpack build [-dir=PATH] [-cp=PATH [-cp-PATH [...]]] BASE-IMAGE COMMAND ARGS...

The build process is:

1. Create a new container (the _build container_) from `BASE-IMAGE`.
2. Copy the _build directory_ (by default it is the current directory,
   but a different one can be provided with the `-dir=PATH` option) to
   a _working directory_ inside the container. The working directory
   is a newly created temporary directory with a unique name, created
   in the container's filesystem root.
3. Copy any additional files (specified by `-cp=PATH` option, which
   can be provided multiple times) to the working directory.
4. Run the build container, executing `COMMAND ARGS...` (the _build
   command_) as root in the work directory.
5. Copy the `manifest.json` file from the work directory as a new
   image's manifest. This way, if the manifest is static, it can be
   simply a part of the build directory - but it can be generated or
   modified by the build command as well. The build command can fill
   in image's version label by inspecting version of the package it
   has installed or add other annotations. For example, the
   `freebsd-base` image uses this to include release patchlevel in the
   version number. The manifest is merged with some defaults
   (`acKind`, `acVersion`, timestamp annotation, `os` and `arch`
   labels from parent image), so the manifest doesn't need to include
   everything.
6. Remove the work directory from the build container.
7. Save the build container's rootfs + manifest from previous step as
   the new image, removing the build container in the process.

Make Macros
-----------

A Make macro file, `jetpack.image.mk`, is provided to automate the
process and wrap the common pieces. It is in `share/` subdirectory of
this repository, and is installed as
`/usr/local/share/jetpack/jetpack.image.mk`. The example images in the
`images/` subdirectory use these macros.

There are three "wildcard targets": `prepare`, `build`, and
`clean`. Each of these is defined, but does not have any commands
associated, so any dependencies or commands can be added in the
Makefile. Each of them will also automatically depend on any task that
starts with its name and a dot. For example, if you define targets
`clean.something` and `clean.something_else`, the `clean` target will
depend on them.

The basic structure of image Makefile is:

    # Add sharedir (/usr/local/share/jetpack for system-wide
    # installation, ./share for in-place build) to the include path.
    .MAKEFLAGS: -I${/path/to/sharedir:L:tA}
    
    VARIABLES = values
    
    targets:
    	commands
    
    # Include Jetpack macros. They need to be at the end.
    .include "jetpack.image.mk"

### Invocation

You can specify `JETPACK=jetpack-command` when running make, to
specify location of the `jetpack` binary if it's not in `$PATH`, or to
call jetpack with sudo (`JETPACK='sudo path/to/jetpack'`).

### Preparation and main target

The default Make target is `image`. It depends on wildcard target
`prepare`. Makefile can define commands and dependencies for the
`prepare` task, or define `prepare.*` tasks that `prepare` will
automatically depend on.

The `image` task itself, after `prepare`, will run `jetpack image build`:

    ${JETPACK} image ${PARENT_IMAGE} build ${BUILD_COMMAND} ${BUILD_ARGS}

If `BUILD_COMMAND` haven't been redefined by user, it is `make
.jetpack.build.`, so the process wraps back into the Makefile. The
`.jetpack.build.` target calls the `build` wildcard target (which
should prepare the image), and then `manifest.json` to ensure that the
manifest file exists, and give Make a chance to build it. The
following variables can be used to modify this process:

 - `BUILD_DIR` can specify a build directory other than `.`.
 - `BUILD_CP` can specify a list of files to copy to the work
   directory. By default, the `jetpack.image.mk` itself is added to this
   list, so that `.include "jetpack.image.mk"` works inside the
   container. To prevent that, set `BUILD_CP_JETPACK_IMAGE_MK=no`.
 - `BUILD_VARS` can specify list of Make (or environment) variables
   that will be added to `BUILD_ARGS`. For example, if `BUILD_VARS`
   includes `http_proxy`, and a `http_proxy` variable is defined,
   `http_proxy=${http_proxy}` will be added to `BUILD_ARGS`.
 - `BUILD_ARGS` can specify additional targets or variables for Make
   (or for the customized `${BUILD_COMMAND}`).
 - `PKG_INSTALL` can specify list of packages to install with `pkg
   install` (if it is defined, a `build..pkg-install` target will be
   defined).

### Cleanup

Run `make clean` to clean up the build directory.

Makefile can define commands for the `clean` target, some extra
`clean.*` targets, or add file names to the `CLEAN_FILES`
variable. This concerns only build directory on host, the work
directory in the container is always removed.

Base Images
-----------

Base (non-derived) images can be made from a rootfs tarball (FreeBSD's
`base.txz`, Ubuntu's cloudimg, or made with debootstrap) and a
manifest. Easiest way is to use `security/fakeroot` in a temporary
directory to run a script like this:

    mkdir rootfs
    tar -C rootfs -xvf /path/to/rootfs-tarball
    cp /path/to/manifest.json manifest
    tar -cJvf ../$NAME.aci manifest rootfs

Be aware that `fakeroot` adds significant filesystem access overhead,
so remember to do parts that don't need root privileges (such as
removing the unpaked files) without it.

A sample script that takes a root filesystem tarball and JSON manifest
file and outputs an ACI archive is provided as
[share/makeaci.sh](share/makeaci.sh).
