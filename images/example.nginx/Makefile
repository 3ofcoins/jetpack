.MAKEFLAGS: -I${../../share:L:tA}

PARENT_IMAGE = freebsd-base
BUILD_VARS = DEVEL http_proxy

build:
	env ASSUME_ALWAYS_YES=YES pkg install nginx${DEVEL:D-devel}
	install -v -m 0640 -o root -g www nginx.conf /usr/local/etc/nginx.conf

manifest.json:
	./manifest.json.sh > $@

.include "jetpack.image.mk"
