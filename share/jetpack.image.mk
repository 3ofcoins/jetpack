JETPACK ?= jetpack
BUILD_COMMAND ?= make build+manifest
BUILD_DIR ?= .
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

prepare.import_file: $(IMPORT_FILE) $(IMPORT_MANIFEST)
.ifdef IMPORT_SHA256
	sha256 -c $(IMPORT_SHA256) $(IMPORT_FILE)
.else
.warn "Download of ${IMPORT_URL} is not validated! Set the IMPORT_SHA256 variable."
.endif

$(IMPORT_FILE):
	fetch -o $@ $(IMPORT_URL)
.endif

.jetpack.image.mk.path := $(.PARSEDIR)/$(.PARSEFILE)
.if ${BUILD_CP_JETPACK_IMAGE_MK} == yes
BUILD_CP += ${.jetpack.image.mk.path}
.endif

image: .PHONY prepare
.ifdef IMPORT_FILE
	$(JETPACK) import $(IMPORT_FILE) $(IMPORT_MANIFEST)
.else
	$(JETPACK) build ${BUILD_CP:@.FILE.@-cp=${.FILE.}@} $(PARENT_IMAGE) $(BUILD_DIR) $(BUILD_COMMAND) $(BUILD_ARGS)
.endif

.ifdef PKG_INSTALL
build.pkg-install: .PHONY
	env ASSUME_ALWAYS_YES=YES pkg install ${PKG_INSTALL}
.endif

.if !empty(CLEAN_FILES)
clean.files:
	rm -rf $(CLEAN_FILES)
.endif

build+manifest: .PHONY build .WAIT manifest.json

prepare: .PHONY ${.ALLTARGETS:Mprepare.*}
build:   .PHONY ${.ALLTARGETS:Mbuild.*}
clean:   .PHONY ${.ALLTARGETS:Mclean.*}
