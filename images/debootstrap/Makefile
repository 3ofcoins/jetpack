.MAKEFLAGS: -I${../../share:L:tA}

SUITE ?= wheezy
IMPORT_FILE = rootfs.${SUITE}.txz
IMPORT_MANIFEST = manifest.${SUITE}.json

${IMPORT_MANIFEST}: ${IMPORT_FILE} manifest.json.sh
	./manifest.json.sh ${SUITE} > $@

CLEAN_FILES = rootfs.*.txz manifest.*.json

.include "jetpack.image.mk"
