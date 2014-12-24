.ifndef DATASET
.error DATASET= parameter must be set
.endif

.ifdef VERBOSE
test_args += -test.v
.endif

test_args += DATASET=$(DATASET)
.ifdef QUICK
test_args += quick
.endif
.ifdef KEEPFS
test_args += keepfs
.endif
.ifdef SAVE
test_args += save
.endif
.ifdef http_proxy
test_args += http_proxy=$(http_proxy)
.endif

all: test

test: test.bin
	sudo ./test.bin $(test_args)

.PHONY: test.bin
test.bin:
	go test -c -o $@

clean:
	rm test.bin

mrproper: clean
	rm -f dataset.zfs
