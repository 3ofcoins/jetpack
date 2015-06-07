#!/bin/sh
# Based on https://github.com/tianon/docker-brew-ubuntu-core/blob/master/update.sh
set -e -x

. ./ubuntu-$(lsb_release --codename --short)-build-info.txt

install -o root -g root -m 0755 policy-rc.d /usr/sbin/policy-rc.d
dpkg-divert --local --rename --add /sbin/initctl
ln -sfv /bin/true /sbin/initctl
install -o root -g root -m 0644 apt.conf /etc/apt/apt.conf.d/jetpack
dpkg -i udev.deb
sed -i '/verse$/s/^##* *//' /etc/apt/sources.list
ln -sv /bin/login /usr/bin/login

cat >manifest.json <<EOF
{
  "name": "ubuntu",
  "labels": [
    { "name": "version", "value": "$(lsb_release --release --short).${SERIAL}" },
    { "name": "codename", "value": "$(lsb_release --codename --short)" },
    { "name": "os", "value": "linux" },
    { "name": "arch", "value": "i386" }
  ]
}
EOF
