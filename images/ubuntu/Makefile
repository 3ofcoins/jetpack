.MAKEFLAGS: -I${../../share:L:tA}

# Ubuntu base images, starting from cloudimg. Needs `fpm` command (from Ruby gem `fpm`) and archivers/gtar installed.

CODENAME ?=	trusty
RELEASE =	${RELEASE_${CODENAME}}
RELEASE_precise =	12.04
RELEASE_trusty =	14.04
RELEASE_utopic =	14.10
RELEASE_vivid = 	15.04
ALL_CODENAMES = vivid utopic trusty precise

BASEURL =	https://partner-images.canonical.com/core/${CODENAME}/current

CLOUDIMG_URL =	${BASEURL}/ubuntu-${CODENAME}-core-cloudimg-i386-root.tar.gz
CLOUDIMG_SHA256_cmd = awk '/\*${CLOUDIMG_FILE}$$/ { print $$1 }' SHA256SUMS
CLOUDIMG_SHA256 = ${CLOUDIMG_SHA256_cmd:sh}
CLOUDIMG_FILE = ${CLOUDIMG_URL:T}
BUILDINFO_FILE = ubuntu-${CODENAME}-build-info.txt
BASE_MANIFEST_FILE = ubuntu-${CODENAME}.base.manifest.json
BASE_ACI_FILE = ubuntu-${CODENAME}.base.aci
ACI_FILE = ubuntu-${CODENAME}.aci

CLEAN_FILES +=	*.dl SHA256SUMS SHA256SUMS.gpg ubuntu-*-root.tar.gz ubuntu-*-build-info.txt

SHA256SUMS.dl:
	fetch -n -o $@ ${BASEURL}/${@:R}

SHA256SUMS.gpg:
	fetch -n ${BASEURL}/$@

SHA256SUMS: SHA256SUMS.dl SHA256SUMS.gpg
	gpg --verify SHA256SUMS.gpg SHA256SUMS.dl
	ln SHA256SUMS.dl $@

${CLOUDIMG_FILE}.dl:
	fetch -n -o $@ ${CLOUDIMG_URL}

${CLOUDIMG_FILE}: ${CLOUDIMG_FILE}.dl SHA256SUMS
	sha256 -c ${CLOUDIMG_SHA256} ${CLOUDIMG_FILE}.dl
	ln ${CLOUDIMG_FILE}.dl $@

${BUILDINFO_FILE}:
	fetch -o $@ ${BASEURL}/unpacked/build-info.txt

${BASE_MANIFEST_FILE}: base.manifest.json.sh ${BUILDINFO_FILE} ${CLOUDIMG_FILE}
	./base.manifest.json.sh ${CODENAME} > $@

${BASE_ACI_FILE}: ${CLOUDIMG_FILE} ${BASE_MANIFEST_FILE}
	${MAKEACI} ${CLOUDIMG_FILE} ${BASE_MANIFEST_FILE} $@

${BASE_ACI_FILE}.id: ${BASE_ACI_FILE}
	jetpack fetch -saveid=$@ -insecure-allow-no-signature ./${BASE_ACI_FILE}

fetch: ${CLOUDIMG_FILE} ${BUILDINFO_FILE}
base: ${BASE_ACI_FILE}

.for task in fetch base prepare image aci flat-aci
all.${task}:
.for codename in ${ALL_CODENAMES}
	${MAKE} ${task} CODENAME=${codename}
.endfor
.endfor

PARENT_IMAGE =	`cat ${BASE_ACI_FILE}.id`
BUILD_COMMAND =	/bin/sh ./build.sh ${CODENAME}
CLEAN_FILES +=	_bin manifest.json

prepare: udev.deb ${BASE_ACI_FILE}.id

udev.deb:
	rm -rf _bin
	mkdir _bin
	ln -s `which gtar` _bin/tar
	env PATH=${.CURDIR}/_bin:$$PATH fpm -p $@ -t deb -s empty -n ${@:R} --version 0 --epoch 99

.include "jetpack.image.mk"
