.MAKEFLAGS: -I${../../share:L:tA}

PARENT_IMAGE = freebsd-base/release$(RELEASE)
CLEAN_FILES  = entropy manifest.json
BUILD_VARS   = http_proxy

prepare: entropy

entropy:
	dd if=/dev/random of=entropy bs=4096 count=1

build:
	sed -i '' 's|^Components.*|Components world/base|' /etc/freebsd-update.conf
	install -v -m 0644 rc.conf /etc/rc.conf
	install -v -m 0600 entropy /entropy
	patch /usr/sbin/freebsd-update < freebsd-update.patch
	env PAGER=cat freebsd-update -s update6.freebsd.org fetch install
	rm -rf /var/db/freebsd-update/* /etc/resolv.conf

manifest.json:
	./manifest.json.sh > $@

.include "jetpack.image.mk"
