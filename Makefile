all: bin/jetpack

.PHONY: bin/jetpack clean distclean sys.destroy sys.init sys.recycle

CC=clang
.export CC

bin/jetpack:
	go build -o $@

clean:
	rm -rf bin/ *.aci

distclean: clean
	rm -rf tmp/ cache/

cache/base.txz:
	mkdir -p cache
	fetch -o ${@} ftp://ftp2.freebsd.org/pub/FreeBSD/releases/amd64/amd64/10.1-RELEASE/base.txz

freebsd-base-current.aci: ./scripts/freebsd-baseimage.sh cache/base.txz
	sudo ./scripts/freebsd-baseimage.sh -u -b $$(pwd)/cache/base.txz
	sudo chown $$(id -u):$$(id -g) *.aci

sys.destroy:
	sudo zfs destroy -R zroot/jetpack

sys.init: bin/jetpack freebsd-base-current.aci
	sudo ./bin/jetpack init
	sudo ./bin/jetpack import ./freebsd-base-current.aci

sys.recycle: sys.destroy sys.init
