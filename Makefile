prefix?=	/usr/local/

gopath =	${.CURDIR:tA}/gopath
gopkg =		github.com/3ofcoins/jetpack
goenv =		env -u GOBIN GOPATH=${gopath} GO15VENDOREXPERIMENT=1 CC=clang

all: bin/jetpack bin/mds bin/stage2

.gopath = ${gopath}/.sentinel
${.gopath}:
	mkdir -p ${gopath}/src/${gopkg:H}
	ln -svf ${.CURDIR:tA} ${gopath}/src/${gopkg}
	touch $@

bin/jetpack bin/mds: .go.build.
.PHONY: .go.build.
.go.build.: ${.gopath}
	${goenv} GOBIN=${.CURDIR:tA}/bin go install ${gopkg}/cmd/jetpack ${gopkg}/cmd/mds

bin/stage2: stage2.c
	-mkdir -p bin
	${CC} ${CFLAGS} ${LDFLAGS} -o $@ stage2.c

install: .PHONY bin/jetpack bin/stage2
	set -e -x ; \
	    prefix=$$(bin/jetpack -config=/dev/null config path.prefix) ; \
	    install -m 0755 -d ${DESTDIR}$${prefix}/bin ${DESTDIR}$${prefix}/libexec/jetpack ${DESTDIR}$${prefix}/share/jetpack ${DESTDIR}$${prefix}/etc ; \
	    install -m 0755 -s bin/jetpack ${DESTDIR}$${prefix}/bin/jetpack ; \
	    install -m 0755 -s bin/stage2 bin/mds ${DESTDIR}$${prefix}/libexec/jetpack/ ; \
	    install -m 0644 share/*[^~] ${DESTDIR}$${prefix}/share/jetpack/ ; \
	    for section in 5 ; do \
	        install -m 0755 -d ${DESTDIR}$${prefix}/man/man$${section} ; \
	        install -m 0644 man/*.$${section} ${DESTDIR}$${prefix}/man/man$${section} ; \
	    done ; \
	    install -m 0644 jetpack.conf.sample ${DESTDIR}$${prefix}/etc/jetpack.conf.sample

# appc/spec stuff
spec = vendor/github.com/appc/spec

validator-aci: ${spec}/bin/ace-validator-main.aci ${spec}/bin/ace-validator-sidekick.aci
${spec}/bin/ace-validator-main.aci ${spec}/bin/ace-validator-sidekick.aci:
	cd ${spec} && ./build.sh && env NO_SIGNATURE=1 ./scripts/build-ace-validator-acis


clean: .PHONY
	rm -rf ${gopath} bin tmp ${spec}/bin ${spec}/gopath
