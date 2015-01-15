Debootstrapped Linux Images
===========================

Jetpack runs 32-bit Linux images, using Linux emulation layer. To
build the images:

1. On an x86 or x86_64 Linux, run `./debootstrap.sh CODENAME`, where
   `CODENAME` is a Debian or Ubuntu release codename; tested so far on
   wheezy (Debian 7) and Precise (Ubuntu 12.04).
2. Copy the resulting `rootfs.CODENAME.txz` file to the build
   directory on FreeBSD with installed Jetpack.
3. Run `make SUITE=CODENAME` to build the image.
