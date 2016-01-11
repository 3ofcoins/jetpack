prefix?=	/usr/local

gopath =	gopath
gopkg =		github.com/3ofcoins/jetpack
goenv =		env -u GOBIN GOPATH=${gopath:tA} GO15VENDOREXPERIMENT=1 CC=clang

gopkg_path =	${gopath}/src/${gopkg}

all: bin/jetpack bin/mds bin/stage2

${gopkg_path}:
	mkdir -p ${gopkg_path:H}
	if test -e ${gopkg_path} ; then rm -v ${gopkg_path} ; fi
	ln -sv ${.CURDIR:tA} ${gopkg_path}

bin/jetpack bin/mds: .go.build.
.PHONY: .go.build.
.go.build.: ${gopkg_path}
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
	        install -m 0755 -d ${DESTDIR}$${prefix}/share/man/man$${section} ; \
	        install -m 0644 man/*.$${section} ${DESTDIR}$${prefix}/share/man/man$${section} ; \
	    done ; \
	    install -m 0644 jetpack.conf.sample ${DESTDIR}$${prefix}/etc/jetpack.conf.sample

# appc/spec stuff
spec = vendor/github.com/appc/spec

validator-aci: ${spec}/bin/ace-validator-main.aci ${spec}/bin/ace-validator-sidekick.aci
${spec}/bin/ace-validator-main.aci ${spec}/bin/ace-validator-sidekick.aci:
	sed -i~ s/linux/freebsd/ ${spec}/ace/*.json
	cd ${spec} && bash ./build && env NO_SIGNATURE=1 bash ./ace/build_aci


# gvt for dependency management

${gopath}/bin/gvt: ${gopkg_path}
	${goenv} go get -u github.com/FiloSottile/gvt

gvt: ${gopath}/bin/gvt
	${goenv} ${gopath}/bin/gvt ${CMD}

clean: .PHONY
	rm -rf ${gopath} bin tmp ${spec}/bin ${spec}/gopath
