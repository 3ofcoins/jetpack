.ifdef PREFIX
bindir      ?= $(PREFIX)/bin
libexecdir  ?= $(PREFIX)/libexec/jetpack
sharedir    ?= $(PREFIX)/share/jetpack
examplesdir ?= $(PREFIX)/share/examples/jetpack
sysconfdir  ?= $(PREFIX)/etc
.else
bindir      ?= ${.CURDIR}/bin
libexecdir  ?= ${bindir}
sharedir    ?= ${.CURDIR}/share
examplesdir ?= ${.CURDIR}/images
sysconfdir  ?= ${.CURDIR}
.endif

version := ${cat VERSION:L:sh}

.if exists(.git)
revision := ${git describe --always --long --dirty='*':L:sh}
.else
revision := (unknown)
.endif

const.jetpack = \
	LibexecPath="${libexecdir}" \
	DefaultConfigPath="${sysconfdir}/jetpack.conf" \
	SharedPath="${sharedir}" \
	Version="${version}" \
	IsDevelopment=${PREFIX:Dfalse:Utrue} \
	BuildTimestamp="${%FT%TZ:L:gmtime}" \
	Revision="${revision}"

const.go := src/lib/jetpack/const.go

libexec = ${echo src/libexec/*:L:sh:S/^src\///}
libexec += github.com/appc/spec/actool

GB ?= vendor/bin/gb

all: ${GB} .prefix ${const.go}
	${GB} build ${PREFIX:D-r }bin/jetpack ${libexec}

${const.go}: .PHONY
	echo 'package jetpack ${const.jetpack:@.CONST.@; const ${.CONST.}@}' | gofmt > $@

.prefix: .PHONY
	echo "${PREFIX:Udevelopment}" > $@
.if exists(.prefix)
.prefix := ${cat .prefix:L:sh}
.else
.prefix := (no prefix saved)
.endif

# Convenience

bin/jetpack: .PHONY ${const.go} ${GB}
	${GB} build bin/jetpack ${libexec}

.for libexec1 in ${libexec}
libexec.bin += ${libexec1:T}
bin/${libexec1:T}: .PHONY ${const.go} ${GB}
	${GB} build ${libexec1}
.endfor

${GB}:
	env GOBIN=${GB:H} GOPATH=${.CURDIR}/vendor go install github.com/constabulary/gb/...

.ifdef PREFIX
install: .PHONY
.if "${.prefix}" != "${PREFIX}"
	@echo 'Cannot install to ${PREFIX}, source was built for ${.prefix}' ; false
.else
	install -m 0755 -d $(DESTDIR)$(bindir) $(DESTDIR)$(libexecdir) $(DESTDIR)$(sharedir) $(DESTDIR)$(examplesdir) $(DESTDIR)$(sysconfdir)
	install -m 0755 -s bin/jetpack $(DESTDIR)$(bindir)/jetpack
	install -m 0755 -s bin/stage2 bin/mds $(DESTDIR)$(libexecdir)
	install -m 0644 share/* $(DESTDIR)$(sharedir)
	install -m 0644 jetpack.conf.sample $(DESTDIR)$(sysconfdir)/jetpack.conf.sample
	cp -R images/ $(DESTDIR)$(examplesdir)
	sed -i '' -e 's!^\.MAKEFLAGS: *-I.*!.MAKEFLAGS: -I$(sharedir)!' $(DESTDIR)$(examplesdir)/*/Makefile
.endif

uninstall: .PHONY
	rm -rf \
	    $(DESTDIR)$(bindir)/jetpack \
	    $(DESTDIR)$(libexecdir) \
	    $(DESTDIR)$(sharedir) \
	    $(DESTDIR)$(examplesdir) \
	    $(DESTDIR)$(sysconfdir)/jetpack.conf.sample

reinstall: .PHONY uninstall .WAIT install
.endif

# appc/spec stuff
spec = ${.CURDIR}/vendor/src/github.com/appc/spec

validator-aci: ${spec}/bin/ace-validator-main.aci ${spec}/bin/ace-validator-sidekick.aci
${spec}/bin/ace-validator-main.aci ${spec}/bin/ace-validator-sidekick.aci:
	sed -i~ s/linux/freebsd/ ${spec}/ace/*.json
	cd ${spec} && bash ./build && env NO_SIGNATURE=1 bash ./ace/build_aci

clean: .PHONY
	rm -rf bin pkg tmp vendor/bin vendor/pkg .prefix ${const.go} ${spec}/bin ${spec}/gopath

# development helpers
cloc:
	cloc --exclude-dir=vendor .

ack:
	ack --type=go --ignore-dir=vendor -w ${q}
