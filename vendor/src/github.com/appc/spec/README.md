# App Container

[![Build Status](https://travis-ci.org/appc/spec.png?branch=master)](https://travis-ci.org/appc/spec)

This repository contains schema definitions and tools for the App Container (appc) specification.
These include technical details on how an appc image is downloaded over a network, cryptographically verified, and executed on a host.
See [SPEC.md](SPEC.md) for details of the specification itself.

For information on the packages in the repository, see their respective [godocs](http://godoc.org/github.com/appc/spec).

## What is the App Container spec?

App Container (appc) is a well-specified and community developed specification that defines an image format, runtime environment and discovery mechanism for application containers.
An _application container_ is a way of packaging and executing processes on a computer system that isolates the application from the underlying host operating system.
For example, a Python web app packaged as a container would bring its own copy of a Python runtime, shared libraries, and application code, and would not share those packages with the host.

Application containers are useful because they put developers in full control of the exact versions of software dependencies for their applications.
This reduces surprises that can arise because of discrepancies between different environments (like development, test, and production), while freeing the underlying OS from worrying about shipping software specific to the applications it will run.
This decoupling of concerns increases the ability for the OS and application to be serviced for updates and security patches.

For these reasons we want the world to run containers, a world where your application can be packaged once, and run in the environment you choose.

The App Container (appc) spec aims to have the following properties:

- **Composable**. All tools for downloading, installing, and running containers should be well integrated, but independent and composable.
- **Secure**. Isolation should be pluggable, and the cryptographic primitives for strong trust, image auditing and application identity should exist from day one.
- **Decentralized**. Discovery of container images should be simple and facilitate a federated namespace and distributed retrieval. This opens the possibility of alternative protocols, such as BitTorrent, and deployments to private environments without the requirement of a registry.
- **Open**. The format and runtime should be well-specified and developed by a community. We want independent implementations of tools to be able to run the same container consistently.

## What is the promise of the App Container Spec?

By explicitly defining - separate of any particular implementation - how an app is packaged into an image, downloaded over a network, and executed as a container, we hope to enable a community of engineers to build tooling around the fundamental building block of a container.
Some examples of build systems and tools that have been built so far include:

- [goaci](https://github.com/jonboulle/goaci) - ACI builder for Go projects
- [docker2aci](https://github.com/appc/docker2aci) - ACI builder from Docker images
- [actool](https://github.com/appc/spec/tree/master/actool) - ACI builder from root filesystems
- [baci](https://github.com/sgotti/baci) - A generic ACI build project

## What are some implementations of the spec?

The most mature implementation of the spec today is [Rocket](https://github.com/coreos/rocket), but several other implementations are being actively worked on:

- [Jet Pack](https://github.com/3ofcoins/jetpack) - FreeBSD/Go
- [libappc](https://github.com/cdaylward/libappc) - C++ library
- [Nose Cone](https://github.com/cdaylward/nosecone) - Linux/C++
- [Rocket](https://github.com/coreos/rocket) - Linux/Go

## Building ACIs

`actool` can be used to build an Application Container Image from an [Image Layout](SPEC.md#image-layout) - that is, from an Image Manifest and an application root filesystem (rootfs).

For example, to build a simple ACI (in this case consisting of a single binary), one could do the following:
```
$ find /tmp/my-app/
/tmp/my-app/
/tmp/my-app/manifest
/tmp/my-app/rootfs
/tmp/my-app/rootfs/bin
/tmp/my-app/rootfs/bin/my-app
$ cat /tmp/my-app/manifest
{
    "acKind": "ImageManifest",
    "acVersion": "0.5.1",
    "name": "my-app",
    "labels": [
        {"name": "os", "value": "linux"},
        {"name": "arch", "value": "amd64"}
    ],
    "app": {
        "exec": [
            "/bin/my-app"
        ],
        "user": "0",
        "group": "0"
    }
}
$ actool build /tmp/my-app/ /tmp/my-app.aci
```

Since an ACI is simply an (optionally compressed) tar file, we can inspect the created file with simple tools:

```
$ tar tvf /tmp/my-app.aci
drwxrwxr-x 1000/1000         0 2014-12-10 10:33 rootfs
drwxrwxr-x 1000/1000         0 2014-12-10 10:36 rootfs/bin
-rwxrwxr-x 1000/1000   5988728 2014-12-10 10:34 rootfs/bin/my-app
-rw-r--r-- root/root       332 2014-12-10 20:40 manifest
```

and verify that the manifest was embedded appropriately
```
tar xf /tmp/my-app.aci manifest -O | python -m json.tool
{
    "acKind": "ImageManifest",
    "acVersion": "0.5.1",
    "annotations": null,
    "app": {
        "environment": [],
        "eventHandlers": null,
        "exec": [
            "/bin/my-app"
        ],
        "group": "0",
        "isolators": null,
        "mountPoints": null,
        "ports": null,
        "user": "0"
    },
    "dependencies": null,
    "labels": [
        {
            "name": "os",
            "value": "linux"
        },
        {
            "name": "arch",
            "value": "amd64"
        }
    ],
    "name": "my-app",
    "pathWhitelist": null
}
```

## Validating App Container implementations

`actool validate` can be used by implementations of the App Container Specification to check that files they produce conform to the expectations.

### Validating Image Manifests and Pod Manifests

To validate one of the two manifest types in the specification, simply run `actool validate` against the file.

```
$ actool validate ./image.json
$ echo $?
0
```

Multiple arguments are supported, and more output can be enabled with `-debug`:

```
$ actool -debug validate image1.json image2.json
image1.json: valid ImageManifest
image2.json: valid ImageManifest
```

`actool` will automatically determine which type of manifest it is checking (by using the `acKind` field common to all manifests), so there is no need to specify which type of manifest is being validated:
```
$ actool -debug validate /tmp/my_container
/tmp/my_container: valid PodManifest
```

If a manifest fails validation the first error encountered is returned along with a non-zero exit status:
```
$ actool validate nover.json
nover.json: invalid ImageManifest: acVersion must be set
$ echo $?
1
```

### Validating ACIs and layouts

Validating ACIs or layouts is very similar to validating manifests: simply run the `actool validate` subcommmand directly against an image or directory, and it will determine the type automatically:
```
$ actool validate app.aci
$ echo $?
0
$ actool -debug validate app.aci
app.aci: valid app container image
```

```
$ actool -debug validate app_layout/
app_layout/: valid image layout
```

To override the type detection and force `actool validate` to validate as a particular type (image, layout or manifest), use the `--type` flag:

```
actool -debug validate -type appimage hello.aci
hello.aci: valid app container image
```

### Validating App Container Executors (ACEs)

The [`ace`](ace/) package contains a simple go application, the _ACE validator_, which can be used to validate app container executors by checking certain expectations about the environment in which it is run: for example, that the appropriate environment variables and mount points are set up as defined in the specification.

To use the ACE validator, first compile it into an ACI using the supplied `build_aci` script:
```
$ ace/build_aci

You need a passphrase to unlock the secret key for
user: "Joe Bloggs (Example, Inc) <joe@example.com>"
4096-bit RSA key, ID E14237FD, created 2014-03-31

Wrote main layout to      bin/ace_main_layout
Wrote unsigned main ACI   bin/ace_validator_main.aci
Wrote main layout hash    bin/sha512-f7eb89d44f44d416f2872e43bc5a4c6c3e12c460e845753e0a7b28cdce0e89d2
Wrote main ACI signature  bin/ace_validator_main.aci.asc

You need a passphrase to unlock the secret key for
user: "Joe Bloggs (Example, Inc) <joe@example.com>"
4096-bit RSA key, ID E14237FD, created 2014-03-31

Wrote sidekick layout to      bin/ace_sidekick_layout
Wrote unsigned sidekick ACI   bin/ace_validator_sidekick.aci
Wrote sidekick layout hash    bin/sha512-13b5598069dbf245391cc12a71e0dbe8f8cdba672072135ebc97948baacf30b2
Wrote sidekick ACI signature  bin/ace_validator_sidekick.aci.asc

```

As can be seen, the script generates two ACIs: `ace_validator_main.aci`, the main entrypoint to the validator, and `ace_validator_sidekick.aci`, a sidekick application. The sidekick is used to validate that an ACE implementation properly handles running multiple applications in a container (for example, that they share a mount namespace), and hence both ACIs should be run together in a layout to validate proper ACE behaviour. The script also generates detached signatures which can be verified by the ACE.

When running the ACE validator, output is minimal if tests pass, and errors are reported as they occur - for example:

```
preStart OK
main OK
sidekick OK
postStop OK
```

or, on failure:
```
main FAIL
==> file "/prestart" does not exist as expected
==> unexpected environment variable "WINDOWID" set
==> timed out waiting for /db/sidekick
```
