.ifndef PARENT_IMAGE
.error "PARENT_IMAGE not defined. I don't know what to do!"
.endif

JETPACK ?= jetpack
JETPACK_FLAGS ?=
# BUILD_DIR ?= .
CLEAN_FILES += ${ACI_ID_FILE} ${ACI_FILE} ${FLAT_ACI_FILE}
BUILD_CP ?=
BUILD_CP_JETPACK_IMAGE_MK ?= yes
BUILD_COMMAND ?= /usr/bin/make .jetpack.build.${"${BUILD_CP_JETPACK_IMAGE_MK}" == "yes":? .jetpack.image.mk=./${.jetpack.image.mk.path:T}:}
ACI_FILE ?= image.aci
ACI_ID_FILE ?= ${ACI_FILE}.id
FLAT_ACI_FILE ?= ${ACI_FILE:R}.flat.aci

JETPACK_SHAREDIR := ${.PARSEDIR}
MAKEACI := ${.PARSEDIR}/makeaci.sh

.MAIN: image

.ifdef DEBUG
JETPACK_FLAGS += -debug
.endif

.ifdef BUILD_VARS
BUILD_ARGS += ${BUILD_VARS:@.VAR.@${${.VAR.}:D${.VAR.}=${${.VAR.}:Q}}@}
.endif

.jetpack.image.mk.path := $(.PARSEDIR)/$(.PARSEFILE)
.if ${BUILD_CP_JETPACK_IMAGE_MK} == yes
BUILD_CP += ${.jetpack.image.mk.path}
.endif

image: ${ACI_ID_FILE}
${ACI_ID_FILE}:
	${MAKE} prepare
	$(JETPACK) ${JETPACK_FLAGS} build -saveid=$@ ${BUILD_CP:@.FILE.@-cp=${.FILE.}@} ${BUILD_DIR:D-dir=${BUILD_DIR}} ${PARENT_IMAGE} $(BUILD_COMMAND) $(BUILD_ARGS)

aci: ${ACI_FILE}
${ACI_FILE}: ${ACI_ID_FILE}
	${JETPACK} ${JETPACK_FLAGS} export `cat ${ACI_ID_FILE}` $@

flat-aci: ${FLAT_ACI_FILE}
${FLAT_ACI_FILE}: ${ACI_ID_FILE}
	${JETPACK} ${JETPACK_FLAGS} export -flat `cat ${ACI_ID_FILE}` $@

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
