all: bin/jetpack

.PHONY: bin/jetpack
bin/jetpack:
	go build -o $@
