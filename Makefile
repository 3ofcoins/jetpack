prefix	?= /usr/local

all:
	gb build -ldflags "-X lib/jetpack.prefix ${prefix}" bin/jetpack ${echo src/libexec/*:L:sh:S/^src\///} github.com/appc/spec/actool

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
