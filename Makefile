all: bin/jetpack

.PHONY: bin/jetpack
bin/jetpack:
	go build -o $@

suclean:
	sudo chflags -R noschg tmp
	sudo rm -rf tmp
