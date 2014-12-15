#!/bin/sh
set -e

usage() {
    echo "Usage: [sudo|fakeroot] $0 [-v VERSION] [-a ARCH] [-n NAME] [-b url://to/base.txz] [-u]" >&2
    exit 99
}

baseurl='ftp://ftp2.freebsd.org/pub/FreeBSD/releases/amd64/amd64/10.1-RELEASE/base.txz'
version=''
arch=''
name='3ofcoins-aci.s3.eu-central-1.amazonaws.com/freebsd-base'
update=0

while getopts 'v:a:n:b:u' option ; do
    case $option in
        v)
            version="$OPTARG"
            ;;
        a)
            arch="$OPTARG"
            ;;
        n)
            name="$OPTARG"
            ;;
        b)
            baseurl="$OPTARG"
            ;;
        u)
            update=1
            ;;
        *)
            usage
    esac
done

if [ $(id -u) -ne 0 ]; then
    echo "Run this script as root, or with fakeroot(1)" >&2
    exit 1
fi

set -x

mkdir -p tmp
workdir="$(mktemp -d tmp/freebsd-baseimage.XXXXXX)"
cd $workdir

fetch -m -l -o base.txz "$baseurl"
mkdir rootfs
tar -C rootfs -xf base.txz

sed -i~ 's/^Components.*/Components world/' rootfs/etc/freebsd-update.conf
rm rootfs/etc/freebsd-update.conf~

cat > rootfs/etc/rc.conf <<EOF
sendmail_submit_enable="NO"
sendmail_outbound_enable="NO"
sendmail_msp_queue_enable="NO"
cron_enable="NO"
devd_enable="NO"
syslogd_enable="NO"
EOF
chmod 0644 rootfs/etc/rc.conf

dd if=/dev/random of=rootfs/entropy bs=4096 count=1
chmod 0600 rootfs/entropy

if [ $update == 1 ]; then
    cp /etc/resolv.conf rootfs/etc/
    chroot rootfs freebsd-update fetch install
    rm -rf rootfs/var/db/freebsd-update/* rootfs/etc/resolv.conf
fi

if [ -z "$version" ]; then
    version="$(./rootfs/bin/freebsd-version -u | sed 's/-[A-Z][A-Z]*//')"
fi

if [ -z "$arch" ]; then
    # FIXME: no better idea now
    arch=$(uname -m)
fi

cat > manifest <<EOF
{
  "acKind": "ImageManifest",
  "acVersion": "0.1.0",
  "name": "$name",
  "labels": [
    { "name": "version", "val": "$version" },
    { "name": "os", "val": "freebsd" },
    { "name": "arch", "val": "$arch" }
  ],
  "annotations": {
    "created": "$(date -u '+%FT%TZ')"
  }
}
EOF

filename="$(basename "$name")-$version-freebsd-$arch"
echo "sha256-$(tar -cf - manifest rootfs | tee image.tar | sha256 -q)" > ../$filename.id
xz -c image.tar > ../$filename.aci

wd="$(pwd)"
cd ..
chflags -R noschg "$wd"
rm -rf "$wd"
