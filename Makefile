.ifdef PREFIX
bindir      ?= $(PREFIX)/bin
libexecdir  ?= $(PREFIX)/libexec/jetpack
sharedir    ?= $(PREFIX)/share/jetpack
sharedir.mk ?= $(sharedir)/mk
examplesdir ?= $(PREFIX)/share/examples/jetpack
.else
bindir      ?= ${.CURDIR}/bin
libexecdir  ?= ${bindir}
sharedir    ?= ${.CURDIR}/share
sharedir.mk ?= ${sharedir}
examplesdir ?= ${.CURDIR}/images
.endif

all: bin/jetpack bin/stage2 bin/test.integration .prefix

.prefix: .PHONY
	echo "${PREFIX:Uin-place usage}" > $@
.if exists(.prefix)
.prefix := ${cat .prefix:L:sh}
.else
.prefix := (no prefix saved)
.endif

CC=clang
GOPATH=$(.CURDIR)/vendor
.export CC GOPATH

bin/jetpack: .PHONY jetpack/const.go integration/const.go
	go build -o $@

bin/stage2: stage2/*.go
	cd stage2 && go build -o ../bin/stage2

bin/test.integration: .PHONY jetpack/const.go
	cd integration && go test -c -o ../bin/test.integration

jetpack/const.go: .PHONY
	echo 'package jetpack; const LibexecPath="$(libexecdir)"; const JetpackImageMkPath="$(sharedir.mk)/jetpack.image.mk"; const Version="'"$$(cat VERSION)"'"; const IsDevelopment=${PREFIX:Dfalse:Utrue}; const BuildTimestamp="'"$$(date -u +%FT%TZ)"'"' \
	    | gofmt > $@

integration/const.go: .PHONY
	echo 'package jetpack_integration; const BinPath="$(bindir)"; const ImagesPath="$(examplesdir)"' \
	    | gofmt > $@

vendor.refetch: .PHONY
	rm -rf vendor
	mkdir -p vendor/src/github.com/3ofcoins
	ln -s ../../../.. vendor/src/github.com/3ofcoins/jetpack
	go get -d
	set -e ; \
	    cd vendor/src ; \
	    for d in code.google.com/p/* ; do \
	        echo "$$d $$(cd $$d ; hg log -l 1 --template '{node|short} {desc|firstline}')" >> $(.CURDIR)/vendor/manifest.txt ; \
	        rm -rf $$d/.hg ; \
	    done ; \
	    for d in github.com/*/* ; do \
	        if test -L $$d ; then \
	            continue ; \
	        fi ; \
	        echo "$$d $$(cd $$d; git log -n 1 --oneline)" >> $(.CURDIR)/vendor/manifest.txt ; \
	        rm -rf $$d/.git ; \
            done

.ifdef PREFIX
install: .PHONY
.if "${.prefix}" != "${PREFIX}"
	@echo 'Cannot install to ${PREFIX}, source was built for ${.prefix}' ; false
.else
	install -m 0755 -d $(DESTDIR)$(bindir) $(DESTDIR)$(libexecdir) $(DESTDIR)$(sharedir.mk) $(DESTDIR)$(examplesdir)
	install -m 0755 -s bin/jetpack $(DESTDIR)$(bindir)/jetpack
	install -m 0755 -s bin/stage2 bin/test.integration $(DESTDIR)$(libexecdir)
	install -m 0644 share/jetpack.image.mk $(DESTDIR)$(sharedir.mk)
	cp -R images/ $(DESTDIR)$(examplesdir)
	sed -i '' -e 's!^\.MAKEFLAGS: *-I.*!.MAKEFLAGS: -I$(sharedir.mk)!' $(DESTDIR)$(examplesdir)/*/Makefile
.endif

uninstall: .PHONY
	rm -rf $(bindir)/jetpack $(libexecdir) $(sharedir) $(examplesdir)


reinstall: .PHONY uninstall .WAIT install
.endif

clean: .PHONY
	rm -rf bin tmp .prefix jetpack/const.go integration/const.go
