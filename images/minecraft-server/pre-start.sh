#!/bin/sh
set -e -x

echo "eula=$(ac-mdc app-annotation eula)" > /opt/minecraft-server/eula.txt
install -v -d -o mcserver -g mcserver /vol/minecraft-server /vol/minecraft-server/logs /vol/minecraft-server/world

