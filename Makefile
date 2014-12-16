all: bin/jetpack

.PHONY: bin/jetpack
bin/jetpack:
	go build -o $@

suclean:
	sudo chflags -R noschg tmp
	sudo rm -rf tmp

.PHONY: sys.destroy sys.init sys.recycle

sys.destroy:
	sudo zfs destroy -R zroot/jetpack

sys.init: bin/jetpack
	sudo ./bin/jetpack init
	sudo ./bin/jetpack import \
	    https://s3.eu-central-1.amazonaws.com/3ofcoins-aci/freebsd-base-10.1-p1-freebsd-amd64.aci

sys.recycle: sys.destroy sys.init
