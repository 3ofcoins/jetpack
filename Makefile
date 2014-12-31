all: bin/jetpack bin/stage2 bin/test.integration

.PHONY: bin/jetpack bin/test.integration vendor.refetch dist jetpack.txz clean

CC=clang
GOPATH=$(.CURDIR)/vendor
.export CC GOPATH

bin/jetpack:
	go build -o $@

bin/stage2: stage2/*.go
	cd stage2 && go build -o ../bin/stage2

bin/test.integration:
	cd integration && go test -c -o ../bin/test.integration

vendor.refetch:
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

dist: jetpack.txz
jetpack.txz:
	git archive --format=tar --prefix=jetpack/ HEAD | xz > $@

clean:
	rm -rf bin tmp jetpack.txz
