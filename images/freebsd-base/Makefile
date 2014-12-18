BASE_URL = ftp://ftp2.freebsd.org/pub/FreeBSD/releases/amd64/amd64/10.1-RELEASE/base.txz
BASE_SHA256 = 2b028a894d25711ad496762622a52d74b1e32ee04693ad1cf056e3ddcdc23975
JETPACK = jetpack

all: prepare
	$(JETPACK) build \
	    -tarball=base.txz \
	    . \
	    make build

clean:
	rm -vf base.txz entropy manifest.json

prepare: base.txz
	sha256 -c $(BASE_SHA256) base.txz
	dd if=/dev/random of=entropy bs=4096 count=1
	cp /etc/resolv.conf .

base.txz:
	fetch -o $@ $(BASE_URL)

build:
	sed -i~ 's|^Components.*|Components world/base|' /etc/freebsd-update.conf
	rm -v /etc/freebsd-update.conf~
	install -v -m 0644 rc.conf resolv.conf /etc/
	install -v -m 0600 entropy /entropy
	PAGER=cat http_proxy=127.0.0.1:3128 freebsd-update fetch install
	rm -rf /var/db/freebsd-update/* /etc/resolv.conf
	./manifest.json.sh > manifest.json
