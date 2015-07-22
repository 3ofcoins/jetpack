.MAKEFLAGS: -I${${JETPACK:Ujetpack} config path.share:L:sh}

PARENT_IMAGE = 3ofcoins.net/freebsd-base
BUILD_VARS = DEVEL http_proxy
PKG_INSTALL = nginx${DEVEL:D-devel}

build:
	install -v -m 0640 -o root -g www nginx.conf /usr/local/etc/nginx.conf

manifest.json:
	./manifest.json.sh > $@

.include "jetpack.image.mk"
