prefix	?= /usr/local

# The old syntax for the `-X` argumant to go compiler's ldflags has
# been deprecated at Go 1.5.
go_version := ${go version | grep -o '[1-9][0-9]*\.[0-9]*':L:sh}
.if ${go_version} >= 1.5
go_ldflags="-X lib/jetpack.prefix=${prefix}"
.else
go_ldflags="-X lib/jetpack.prefix ${prefix}"
.endif

all: bin/jetpack bin/mds bin/actool bin/stage2

bin/jetpack bin/mds bin/actool: .gb.build.
.PHONY: .gb.build.
.gb.build.:
	gb build -ldflags ${go_ldflags} bin/jetpack libexec/mds github.com/appc/spec/actool

bin/stage2: stage2.c
	${CC} ${CFLAGS} ${LDFLAGS} -o $@ stage2.c

install: .PHONY bin/jetpack
	set -e -x ; \
	    prefix=$$(./bin/jetpack -config=/dev/null config path.prefix) ; \
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
spec = ${.CURDIR}/vendor/src/github.com/appc/spec

validator-aci: ${spec}/bin/ace-validator-main.aci ${spec}/bin/ace-validator-sidekick.aci
${spec}/bin/ace-validator-main.aci ${spec}/bin/ace-validator-sidekick.aci:
	sed -i~ s/linux/freebsd/ ${spec}/ace/*.json
	cd ${spec} && bash ./build && env NO_SIGNATURE=1 bash ./ace/build_aci

clean: .PHONY
	rm -rf bin pkg tmp vendor/bin vendor/pkg ${spec}/bin ${spec}/gopath
