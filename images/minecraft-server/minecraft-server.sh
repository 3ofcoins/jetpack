#!/bin/sh
set -x
exec java $(ac-mdc app-annotation java-opts) -jar ./minecraft-server.jar
