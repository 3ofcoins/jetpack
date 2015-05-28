.MAKEFLAGS: -I${../../share:L:tA}

PARENT_IMAGE = freebsd-base/release$(RELEASE)
CLEAN_FILES  = entropy manifest.json
BUILD_VARS   = http_proxy

prepare: entropy appc-metadata-client/ac-mdc

entropy:
	dd if=/dev/random of=entropy bs=4096 count=1

appc-metadata-client/ac-mdc: appc-metadata-client/mdc.go
	${MAKE} -C appc-metadata-client

appc-metadata-client/mdc.go:
	git clone https://github.com/3ofcoins/appc-metadata-client

clean.mdc:
	rm -rf appc-metadata-client

build:
	sed -i '' 's|^Components.*|Components world/base|' /etc/freebsd-update.conf
	install -v -d -m 0755 /usr/local/bin
	install -v -m 0755 appc-metadata-client/ac-mdc /usr/local/bin/ac-mdc
	install -v -m 0644 rc.conf /etc/rc.conf
	install -v -m 0600 entropy /entropy
	patch /usr/sbin/freebsd-update < freebsd-update.patch
	env PAGER=cat freebsd-update -s update6.freebsd.org fetch install
	rm -rf /var/db/freebsd-update/*

manifest.json:
	./manifest.json.sh > $@

.include "jetpack.image.mk"
