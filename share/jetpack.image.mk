.ifndef PARENT_IMAGE
.error "PARENT_IMAGE not defined. I don't know what to do!"
.endif

JETPACK ?= jetpack
BUILD_COMMAND ?= /usr/bin/make .jetpack.build.
# BUILD_DIR ?= .
CLEAN_FILES += image.aci.id image.aci image.flat.aci
BUILD_CP ?=
BUILD_CP_JETPACK_IMAGE_MK ?= yes

JETPACK_SHAREDIR := ${.PARSEDIR}
MAKEACI := ${.PARSEDIR}/makeaci.sh

.MAIN: image

.ifdef BUILD_VARS
BUILD_ARGS += ${BUILD_VARS:@.VAR.@${${.VAR.}:D${.VAR.}=${${.VAR.}:Q}}@}
.endif

.jetpack.image.mk.path := $(.PARSEDIR)/$(.PARSEFILE)
.if ${BUILD_CP_JETPACK_IMAGE_MK} == yes
BUILD_CP += ${.jetpack.image.mk.path}
.endif

image: image.aci.id
image.aci.id:
	${MAKE} prepare
	$(JETPACK) image $(PARENT_IMAGE) build -saveid=$@ ${BUILD_CP:@.FILE.@-cp=${.FILE.}@} ${BUILD_DIR:D-dir=${BUILD_DIR}} $(BUILD_COMMAND) $(BUILD_ARGS)

aci: image.aci
image.aci: image.aci.id
	jetpack image `cat image.aci.id` export $@

flat-aci: image.flat.aci
image.flat.aci: image.aci.id
	jetpack image `cat image.aci.id` export -flat $@

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
