#!/bin/sh
set -e -x

# https://github.com/tianon/docker-brew-ubuntu-core/blob/d7f2045ad9b08962d9728f6d9910fa252282b85f/trusty/Dockerfile

test -f build-info.txt
. ./build-info.txt

cat >/usr/sbin/policy-rc.d <<EOF
#!/bin/sh
exit 101
EOF
chmod 0755 /usr/sbin/policy-rc.d

dpkg-divert --local --rename --add /sbin/initctl
ln -sfv /bin/true /sbin/initctl

dpkg-divert --local --rename --add /sbin/MAKEDEV

ln -sfv /bin/login /usr/bin/login

cat >/etc/apt/apt.conf.d/jetpack <<EOF
DPkg::Post-Invoke { "rm -f /var/cache/apt/archives/*.deb /var/cache/apt/archives/partial/*.deb /var/cache/apt/*.bin || true"; };
APT::Update::Post-Invoke { "rm -f /var/cache/apt/archives/*.deb /var/cache/apt/archives/partial/*.deb /var/cache/apt/*.bin || true"; };
Dir::Cache::pkgcache ""; Dir::Cache::srcpkgcache "";

Acquire::Languages "none";

Acquire::GzipIndexes "true";
Acquire::CompressionTypes::Order:: "gz";
EOF

sed -i 's/^#\s*\(deb.*universe\)$/\1/g' /etc/apt/sources.list

arch=$(arch)
case $arch in
    i?86)   arch=i386  ;;
    x86_64) arch=amd64 ;;
esac

cat >manifest.json <<EOF
{
  "name": "3ofcoins.net/ubuntu",
  "labels": [
    { "name": "version", "value": "$(lsb_release -sr)-${SERIAL}" },
    { "name": "codename", "value": "$(lsb_release -sc)" },
    { "name": "os", "value": "linux" },
    { "name": "arch", "value": "$arch" }
  ],
  "app": {
    "exec": ["/bin/login", "-f", "root"],
    "user": "root",
    "group": "root"
  }
}
EOF
