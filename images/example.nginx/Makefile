JETPACK = jetpack
DEVEL =

all:
	$(JETPACK) build -from=freebsd-base . $(MAKE) build DEVEL=$(DEVEL) http_proxy=172.23.0.1:3128

ASSUME_ALWAYS_YES=YES
.export ASSUME_ALWAYS_YES

.if x$(DEVEL) == x
PACKAGE = nginx
.else
PACKAGE = nginx-devel
.endif

build:
	pkg install $(PACKAGE)
	install -v -m 0640 -o root -g www nginx.conf /usr/local/etc/nginx.conf
	./manifest.json.sh > manifest.json
