PARENT_IMAGE = 3ofcoins.net/freebsd-base
PKG_INSTALL = dialog4ports

build:
	install -v -d /usr/ports /usr/ports.distfiles /usr/ports.wrkdir /usr/ports.packages /usr/local/bin
	install -v make.conf /etc/make.conf
	install -v -m 0755 portbuild.sh portbuild.pre-start.sh /usr/local/bin/

.include <${.jetpack.image.mk:U${${JETPACK:Ujetpack} config path.share:L:sh}/jetpack.image.mk}>
