BASE_URL = ftp://ftp2.freebsd.org/pub/FreeBSD/releases/amd64/amd64/10.1-RELEASE/base.txz
BASE_SHA256 = 2b028a894d25711ad496762622a52d74b1e32ee04693ad1cf056e3ddcdc23975
JETPACK = jetpack

build_params =
.ifdef http_proxy
build_params += http_proxy=$(http_proxy)
.endif

all: prepare
	$(JETPACK) import base.txz manifest.json

prepare: base.txz
	sha256 -c $(BASE_SHA256) base.txz

base.txz:
	fetch -o $@ $(BASE_URL)

clean:
	rm -vf base.txz
