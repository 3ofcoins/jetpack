#!/bin/sh
set -e
suite="$1"
mirror="$2"
if [ -z "$2" ]; then
    case "$suite" in
    hamm|slink|potato|woody|sarge|etch|lenny)
        mirror="http://archive.debian.org/debian-archive/debian/"
        ;;
    squeeze|wheezy|jessie)
        mirror="http://ftp2.de.debian.org/debian/"
        ;;
    warty|hoary|breezy|dapper|edgy|feisty|gutsy|hardy|intrepid|jaunty|karmic|lucid|maverick|natty|oneiric|precise|quantal|raring|saucy|trusty|utopic|vivid)
        mirror="http://archive.ubuntu.com/ubuntu/"
        ;;
    *)
        echo "Don't know the mirror for $suite" >&2
        exit 1
    esac
fi

set -x
rm -rf rootfs.$suite
debootstrap --arch=i386 --variant=minbase $suite ./rootfs.$suite $mirror
tar -C rootfs.$suite -cJf rootfs.$suite.txz .
rm -rf rootfs.$suite
