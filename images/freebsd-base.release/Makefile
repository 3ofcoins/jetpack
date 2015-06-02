SUDO = fakeroot
PV := ${which pv || echo cat:L:sh}

BASE_URL = ftp://ftp2.freebsd.org/pub/FreeBSD/releases/amd64/amd64/10.1-RELEASE/base.txz
BASE_SHA256 = 2b028a894d25711ad496762622a52d74b1e32ee04693ad1cf056e3ddcdc23975

freebsd-base_release-10.1-freebsd-amd64.aci: base.txz manifest.json
	sha256 -c ${BASE_SHA256} base.txz
	set -e -x ; \
	    tmpdir=$$(mktemp -d ./freebsd-base_release.aci.XXXXXXXX) ; \
	    ${PV} base.txz | ${SUDO} ./build.sh $$tmpdir | xz -z -c > $@ ; \
	    rm -rf $$tmpdir

base.txz:
	fetch -o $@ ${BASE_URL}

clean:
	rm -rf base.txz freebsd-base_release-*.aci  freebsd-base_release.aci.*
