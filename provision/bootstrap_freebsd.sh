#!/bin/sh
#
# Bootstrap script for setting up FreeBSD as a vagrant .box

# Install ansible dependencies
sudo pkg install -y python27 && \
sudo ln -sf /usr/local/bin/python2 /usr/bin/python && \
echo "Pre-provisioning completed successfully"
exit 0
