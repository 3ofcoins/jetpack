.MAKEFLAGS: -I${../../share:L:tA}

CODENAME ?=	trusty
RELEASE ?=	${RELEASE_${CODENAME}}
RELEASE_lucid =		10.04
RELEASE_maverick =	10.10
RELEASE_natty =		11.04
RELEASE_oneiric =	11.10
RELEASE_precise =	12.04
RELEASE_quantal =	12.10
RELEASE_raring =	13.04
RELEASE_saucy =		13.10
RELEASE_trusty =	14.04
RELEASE_utopic =	14.10

BASEURL =	https://partner-images.canonical.com/core/${CODENAME}/current

IMPORT_URL =	${BASEURL}/ubuntu-${CODENAME}-core-cloudimg-i386-root.tar.gz
IMPORT_SHA256_cmd = awk '/\*${IMPORT_FILE}$$/ { print $$1 }' SHA256SUMS
IMPORT_SHA256 = ${IMPORT_SHA256_cmd:sh}
IMPORT_MANIFEST = manifest.json

CLEAN_FILES +=	SHA256SUMS SHA256SUMS.gpg *-root.tar.gz build-info.txt manifest.json

prepare..import_file:	SHA256SUMS

SHA256SUMS:
	fetch ${BASEURL}/$@ ${BASEURL}/$@.gpg
	if ! gpg --verify $@.gpg $@; then mv -v $@ $@~CORRUPT ; false ; fi

build-info.txt:
	fetch -o $@ ${BASEURL}/unpacked/build-info.txt

manifest.json: ${IMPORT_FILE} build-info.txt manifest.json.sh
	./manifest.json.sh ${IMPORT_FILE} > $@

.include "jetpack.image.mk"
