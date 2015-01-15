Jetpack Images
==============

Importing pre-made images
-------------------------

Images can be imported from files or URLs, using the _image import_
command:

    jetpack image import AMI
    jetpack image import ROOTFS MANIFEST

The parameters should be either URLs pointing directly at archives, or
filesystem paths. The contents are retrieved using `fetch(1)` command.

`AMI` should point at Application Manifest Image that contains rootfs
and manifest, as described in the Application Container Specification
(caveat: the _dependencies_ are not supported).

Alternatively, `ROOTFS` can point to a tar archive (which may be
compressed with gzip, bzip2, or xz) of the root filesystem, and
MANIFEST can point at raw JSON manifest.

Building derivative images
--------------------------

An existing image can be used to build a derivative image. The
low-level mechanism is the _image build_ command:

    jetpack image BASE-IMAGE build [-dir=PATH] [-cp=PATH [-cp-PATH [...]]] COMMAND ARGS...

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

The `image` task itself, after `prepare`, will:

 - run `${JETPACK} image import …` if `IMPORT_FILE` or `IMPORT_URL` is
   defined (see _Importing Images_ for details), or
 - run `${JETPACK} image ${PARENT_IMAGE} build …` if `PARENT_IMAGE` is
   defined (see _Building Images_ for details), or
 - raise an error if neiter import file/url, nor parent image is
   defined.

### Cleanup

Run `make clean` to clean up the build directory.

Makefile can define commands for the `clean` target, some extra
`clean.*` targets, or add file names to the `CLEAN_FILES`
variable. This concerns only build directory on host, the work
directory in the container is always removed.

### Importing Images

See the `images/freebsd-release.base/Makefile` file for a complete
example. Most of the time, only some variables need to be defined:

 - `IMPORT_FILE` will specify a file to import (AMI or
   rootfs). The file can be provided in the build dir, or can be
   created by Makefile.
 - `IMPORT_MANIFEST` -- if `IMPORT_FILE` is a rootfs tarball, it
   specifies location of the manifest file.
 - `IMPORT_URL` -- if specified, Makefile will download `IMPORT_FILE`
   from that URL. If `IMPORT_URL` is defined, and `IMPORT_FILE` is not
   defined, the `IMPORT_FILE` will be extracted automatically from the
   `IMPORT_URL`.
 - `IMPORT_SHA256` -- a checksum to verify download from `IMPORT_URL`.

The Makefile can also include detailed instructions on how to get the
`IMPORT_FILE`. For example:

    IMPORT_FILE = secret.aci
    CLEAN_FILES = secret.aci.gpg secret.aci
    
    secret.aci: secret.aci.gpg
    	gpg --decrypt --output $@ secret.aci.gpg
    
    secret.aci.gpg:
    	scp secret.login@secret.location:/path/to/secret.aci.gpg $@

> side note: the `jetpack.image.mk` itself uses wildcard targets
> internally. If `IMPORT_URL` is defined: `IMPORT_FILE` is
> automatically defined if not set by user; a `${IMPORT_FILE}` target
> is added that downloads from `${IMPORT_URL}`; a
> `prepare..import_file` target (with double dot to avoid conflicts
> with user's Makefile), depending on `${IMPORT_FILE}`, checks
> `${IMPORT_SHA256}`. The `prepare` target automatically depends on
> `prepare..import_file`.

### Building images

If `PARENT_IMAGE` variable is defined, the `image` target will run:

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
