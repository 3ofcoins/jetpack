.MAKEFLAGS: -I${../../share:L:tA}

PARENT_IMAGE = `cat base.aci.id`
TIDY_FILES   = manifest.json base.aci.id 
CLEAN_FILES  = ${TIDY_FILES} base.manifest.json base.txz base.aci appc-metadata-client
BUILD_VARS   = http_proxy

# Download base.txz and roll a base ACI off it. Needs security/fakeroot, is nicer with deskutils/pv

BASE_VERSION = 10.2
BASE_URL = ftp://ftp2.freebsd.org/pub/FreeBSD/releases/amd64/amd64/${BASE_VERSION}-RELEASE/base.txz
BASE_SHA256 = 75b0d1872b39c35b0d423c352c96d41cc2d7e7d536b78ecdb506c2fbf8072a0b

base.txz:
	fetch -o $@ ${BASE_URL}

base.aci: base.txz
	sha256 -c ${BASE_SHA256} base.txz
	./base.manifest.json.sh ${BASE_VERSION} > ./base.manifest.json
	${MAKEACI} ./base.txz ./base.manifest.json $@

base.aci.id: base.aci
	jetpack import -saveid=$@ ./base.aci

prepare.base: base.aci.id

# Clone and compile metadata client

appc-metadata-client/mdc.go:
	git clone https://github.com/3ofcoins/appc-metadata-client

appc-metadata-client/ac-mdc: appc-metadata-client/mdc.go
	${MAKE} -C appc-metadata-client

prepare.ac-mdc: appc-metadata-client/ac-mdc

# Real build: update base, add some settings, install metadata client

build:
	sed -i '' 's|^Components.*|Components world/base|' /etc/freebsd-update.conf
	install -v -d -m 0755 /usr/local/bin
	install -v -m 0755 appc-metadata-client/ac-mdc /usr/local/bin/ac-mdc
	install -v -m 0644 rc.conf /etc/rc.conf
	env PAGER=cat freebsd-update -s update6.freebsd.org fetch install
	rm -rf /var/db/freebsd-update/*

manifest.json:
	./manifest.json.sh > $@

tidy:
	rm -rf ${TIDY_FILES}

.include "jetpack.image.mk"
