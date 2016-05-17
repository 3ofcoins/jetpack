PARENT_IMAGE = 3ofcoins.net/freebsd-base

build:
	install -d /usr/local/bin
	install -d -o root -g guest -m 0775 /opt/data
	install -m 0755 showenv.sh /usr/local/bin/showenv

.include <${.jetpack.image.mk:U${${JETPACK:Ujetpack} config path.share:L:sh}/jetpack.image.mk}>
