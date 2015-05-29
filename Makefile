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

const.integration = \
	BinPath="${bindir}" \
	ImagesPath="${examplesdir}"

const.go := src/lib/jetpack/const.go src/test/integration/const.go

all: .prefix ${const.go}
	gb build ${PREFIX:D-r }bin/jetpack ${libexec}

.prefix: .PHONY
	echo "${PREFIX:Udevelopment}" > $@
.if exists(.prefix)
.prefix := ${cat .prefix:L:sh}
.else
.prefix := (no prefix saved)
.endif

src/lib/jetpack/const.go: .PHONY
	echo 'package jetpack ${const.jetpack:@.CONST.@; const ${.CONST.}@}' | gofmt > $@

src/test/integration/const.go: .PHONY
	echo 'package jetpack_integration ${const.integration:@.CONST.@; const ${.CONST.}@}' | gofmt > $@

bin/jetpack: .PHONY ${const.go}
	gb build bin/jetpack ${libexec}

libexec = ${echo src/libexec/*:L:sh:S/^src\///}
libexec += github.com/appc/spec/actool

.for libexec1 in ${libexec}
libexec.bin += ${libexec1:T}
bin/${libexec1:T}: .PHONY ${const.go}
	gb build ${libexec1}
.endfor

bin/test.integration: .PHONY ${const.go}
	cd integration && go test -c -o ../bin/test.integration

APPC_SPEC_VERSION=v0.5.2

vendor.refetch: .PHONY
	rm -rf vendor
	go get -d
	cd ${.CURDIR}/vendor/src/github.com/appc/spec && git checkout ${APPC_SPEC_VERSION}
	set -e ; \
	    cd ${.CURDIR}/vendor/src ; \
	    for d in code.google.com/p/* ; do \
	        echo "$$d $$(cd $$d ; hg log -l 1 --template '{node|short} {desc|firstline}')" >> $(.CURDIR)/vendor/manifest.txt ; \
	        rm -rf $$d/.hg ; \
	    done ; \
	    for d in github.com/*/* golang.org/x/* ; do \
	        if test -L $$d ; then \
	            continue ; \
	        fi ; \
	        echo "$$d $$(cd $$d; git log -n 1 --oneline --decorate)" >> $(.CURDIR)/vendor/manifest.txt ; \
	        rm -rf $$d/.git ; \
            done

.ifdef PREFIX
install: .PHONY
.if "${.prefix}" != "${PREFIX}"
	@echo 'Cannot install to ${PREFIX}, source was built for ${.prefix}' ; false
.else
	install -m 0755 -d $(DESTDIR)$(bindir) $(DESTDIR)$(libexecdir) $(DESTDIR)$(sharedir) $(DESTDIR)$(examplesdir)
	install -m 0755 -s bin/jetpack $(DESTDIR)$(bindir)/jetpack
	install -m 0755 -s bin/stage2 bin/mds bin/test.integration $(DESTDIR)$(libexecdir)
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

clean: .PHONY
	rm -rf bin pkg tmp .prefix ${const.go}

# development helpers
cloc:
	cloc --exclude-dir=vendor .

ack:
	ack --type=go --ignore-dir=vendor -w ${q}
