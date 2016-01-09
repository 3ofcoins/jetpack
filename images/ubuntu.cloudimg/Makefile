.MAKEFLAGS: -I${../../share:L:tA}

CODENAME ?=	trusty
RELEASE ?=	${RELEASE_${CODENAME}}
ARCH ?=			amd64
RELEASE_precise =	12.04
RELEASE_trusty =	14.04
RELEASE_utopic =	14.10
RELEASE_vivid = 	15.04
RELEASE_wily =		15.10
RELEASE_xenial =	16.04

BASEURL =	https://partner-images.canonical.com/core/${CODENAME}/current

CLOUDIMG_ACI =		ubuntu-cloudimg-${CODENAME}-${ARCH}.base.aci
CLOUDIMG_TARBALL =	ubuntu-${CODENAME}-core-cloudimg-${ARCH}-root.tar.gz
CLOUDIMG_SHA256_cmd =	awk '/\*${CLOUDIMG_TARBALL}$$/ { print $$1 }' SHA256SUMS
CLOUDIMG_SHA256 =	${CLOUDIMG_SHA256_cmd:sh}
CLOUDIMG_MANIFEST =	${CLOUDIMG_ACI:R}.manifest.json

# 3ofcoins.net/ubuntu-cloudimg

base-aci: ${CLOUDIMG_ACI}
base-import: ${CLOUDIMG_ACI}.id

all-the-acis:
.for build_codename in precise trusty utopic vivid wily xenial
.for build_arch in amd64 i386
	${MAKE} CODENAME=${build_codename} ARCH=${build_arch} flat-aci
.endfor
.endfor

${CLOUDIMG_ACI}.id: ${CLOUDIMG_ACI}
	jetpack import -saveid=$@ ./${CLOUDIMG_ACI}

${CLOUDIMG_ACI}: ${CLOUDIMG_TARBALL} ${CLOUDIMG_MANIFEST}
	${MAKEACI} ${CLOUDIMG_TARBALL} ${CLOUDIMG_MANIFEST} $@

${CLOUDIMG_MANIFEST}: ${CLOUDIMG_TARBALL} build-info.txt manifest.json.sh
	./manifest.json.sh ${RELEASE} ${CODENAME} ${ARCH} > $@

${CLOUDIMG_TARBALL}: SHA256SUMS
	fetch -o $@.dl ${BASEURL}/$@
	sha256 -c ${CLOUDIMG_SHA256} $@.dl
	touch $@.dl
	mv -v $@.dl $@

# TODO: include public GPG key to verify, use local keyring
SHA256SUMS:
	fetch -o $@.dl ${BASEURL}/$@
	fetch ${BASEURL}/$@.gpg
	gpg --verify $@.gpg $@.dl
	touch $@.dl
	mv -v $@.dl $@

build-info.txt:
	fetch -o $@ ${BASEURL}/unpacked/build-info.txt

clean.base:
	rm -rfv \
	    SHA256SUMS SHA256SUMS.gpg \
	    *.dl \
	    *-root.tar.gz \
	    *.manifest.json \
	    build-info.txt

##############################################################################

PARENT_IMAGE =	`cat ${CLOUDIMG_ACI}.id`
BUILD_COMMAND =	/bin/sh ./build.sh

.include "jetpack.image.mk"
