JETPACK = jetpack

all:
	$(JETPACK) build freebsd-base . $(MAKE) build

build:
	install -d /usr/local/bin
	install -m 0755 showenv.sh /usr/local/bin/showenv
