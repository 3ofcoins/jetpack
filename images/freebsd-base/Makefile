.MAKEFLAGS: -I${../../share:L:tA}

PARENT_IMAGE = `cat base.aci.id`
TIDY_FILES   = manifest.json base.aci.id 
CLEAN_FILES  = ${TIDY_FILES} base.manifest.json base.txz base.txz.dl base.aci appc-metadata-client
BUILD_VARS   = http_proxy NO_UPDATE
NO_UPDATE   ?= 0

# Download base.txz and roll a base ACI off it. Needs security/fakeroot, is nicer with deskutils/pv

BASE_VERSION = 10.3
BASE_URL = ftp://ftp2.freebsd.org/pub/FreeBSD/releases/amd64/amd64/${BASE_VERSION}-RELEASE/base.txz
BASE_SHA256 = e7c4c961694e34b2b60598d9668a185b5de1c6241bedfc4891cd9ea8fbff2dc4

base.txz:
	rm -f $@.dl
	fetch -o $@.dl ${BASE_URL}
	sha256 -c ${BASE_SHA256} $@.dl
	mv -v $@.dl $@

base.aci: base.txz
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
.if ${NO_UPDATE} == 0 
	BOOTFILE=`sysctl -n kern.bootfile` ; mkdir -v $${BOOTFILE%/kernel}
	env PAGER=cat freebsd-update -s update6.freebsd.org fetch install
	BOOTFILE=`sysctl -n kern.bootfile` ; rmdir -v $${BOOTFILE%/kernel}
	rm -rf /var/db/freebsd-update/*
.endif

manifest.json:
	./manifest.json.sh > $@

tidy:
	rm -rf ${TIDY_FILES}

.include "jetpack.image.mk"


