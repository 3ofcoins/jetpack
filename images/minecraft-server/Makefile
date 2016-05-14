.MAKEFLAGS: -I${../../share:L:tA}

PARENT_IMAGE = 3ofcoins.net/freebsd-base

SERVER_VERSION = 1.9.4
SERVER_JAR = minecraft_server.${SERVER_VERSION}.jar
SERVER_JAR_URL = https://s3.amazonaws.com/Minecraft.Download/versions/${SERVER_VERSION}/${SERVER_JAR}
SERVER_JAR_SHA256 = 13fea7aa10d804dd14ed7ebde2493dc64c7d3c8173369309bd7f6ea4c0ea40ad

PKG_INSTALL = openjdk8

prepare: ${SERVER_JAR}
	sha256 -c ${SERVER_JAR_SHA256} ${SERVER_JAR}

${SERVER_JAR}:
	fetch ${SERVER_JAR_URL}

# build:

build..pkg-install: build..pkg-latest
build..pkg-latest: .PHONY
	sed -i~ s/quarterly/latest/g /etc/pkg/FreeBSD.conf
	rm /etc/pkg/FreeBSD.conf~

build:
	pw groupadd mcserver -g 199
	pw useradd mcserver -u 199 -g 199 -c 'Minecraft Server' -d /nonexistent -s /bin/sh
	install -d /opt/minecraft-server
	install ${SERVER_JAR} /opt/minecraft-server
	ln -sv ${SERVER_JAR} /opt/minecraft-server/minecraft-server.jar
	install -m 0755 pre-start.sh minecraft-server.sh /opt/minecraft-server
	for tgt in banned-ips.json banned-players.json ops.json usercache.json whitelist.json world logs debug server.properties ; do \
	    ln -sfv /vol/minecraft-server/$$tgt /opt/minecraft-server/$$tgt ; done
	install -d -o mcserver -g mcserver /vol/minecraft-server
	rm -rf /var/cache/pkg/* 

.include "jetpack.image.mk"
