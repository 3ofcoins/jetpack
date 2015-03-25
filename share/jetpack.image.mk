JETPACK ?= jetpack
BUILD_COMMAND ?= /usr/bin/make .jetpack.build.
# BUILD_DIR ?= .
CLEAN_FILES ?=
# IMPORT_FILE
# IMPORT_URL
# IMPORT_SHA256
# IMPORT_MANIFEST
BUILD_CP ?=
BUILD_CP_JETPACK_IMAGE_MK ?= yes

.MAIN: image

.ifdef BUILD_VARS
BUILD_ARGS += ${BUILD_VARS:@.VAR.@${${.VAR.}:D${.VAR.}=${${.VAR.}:Q}}@}
.endif

.ifdef IMPORT_URL
IMPORT_FILE ?= ${IMPORT_URL:C%^.*/%%:C%\?.*$%%}
CLEAN_FILES += $(IMPORT_FILE)

prepare..import_file: $(IMPORT_FILE) $(IMPORT_MANIFEST)
.ifdef IMPORT_SHA256
	sha256 -c $(IMPORT_SHA256) $(IMPORT_FILE)
.else
.warn "Download of ${IMPORT_URL} is not validated! Set the IMPORT_SHA256 variable."
.endif

$(IMPORT_FILE):
	fetch -o $@ $(IMPORT_URL)
.endif

.ifdef IMPORT_MANIFEST
prepare: ${IMPORT_MANIFEST}
.endif

.jetpack.image.mk.path := $(.PARSEDIR)/$(.PARSEFILE)
.if ${BUILD_CP_JETPACK_IMAGE_MK} == yes
BUILD_CP += ${.jetpack.image.mk.path}
.endif

image: .PHONY prepare
.ifdef IMPORT_FILE
	$(JETPACK) image import $(IMPORT_FILE) $(IMPORT_MANIFEST)
.elifdef PARENT_IMAGE
	$(JETPACK) image $(PARENT_IMAGE) build ${BUILD_CP:@.FILE.@-cp=${.FILE.}@} ${BUILD_DIR:D-dir=${BUILD_DIR}} $(BUILD_COMMAND) $(BUILD_ARGS)
.else
.error "Define either IMPORT_FILE/IMPORT_URL, or PARENT_IMAGE. I don't know what to do!"
.endif

.ifdef PKG_INSTALL
build..pkg-install: .PHONY
	env ASSUME_ALWAYS_YES=YES pkg install ${PKG_INSTALL}
.endif

.if !empty(CLEAN_FILES)
clean..files:
	rm -rf $(CLEAN_FILES)
.endif

.jetpack.build.: .PHONY build .WAIT manifest.json

prepare: .PHONY ${.ALLTARGETS:Mprepare.*}
build:   .PHONY ${.ALLTARGETS:Mbuild.*}
clean:   .PHONY ${.ALLTARGETS:Mclean.*}
