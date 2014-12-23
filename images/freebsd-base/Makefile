RELEASE_LABELS = 
JETPACK = jetpack

build_params =
.ifdef http_proxy
build_params += http_proxy=$(http_proxy)
.endif

all: prepare
	$(JETPACK) build \
	    freebsd-base/release$(RELEASE_LABELS) \
	    . \
	    make build $(build_params)

clean:
	rm -vf entropy manifest.json

prepare:
	dd if=/dev/random of=entropy bs=4096 count=1

build:
	sed -i~ 's|^Components.*|Components world/base|' /etc/freebsd-update.conf
	rm -v /etc/freebsd-update.conf~
	install -v -m 0644 rc.conf /etc/rc.conf
	install -v -m 0600 entropy /entropy
	PAGER=cat freebsd-update -s update6.freebsd.org fetch install
	rm -rf /var/db/freebsd-update/* /etc/resolv.conf
	./manifest.json.sh > manifest.json
