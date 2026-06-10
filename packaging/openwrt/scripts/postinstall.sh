#!/bin/sh
set -eu

if [ -x /etc/init.d/nook ] && /etc/init.d/nook running >/dev/null 2>&1; then
  /etc/init.d/nook restart >/dev/null 2>&1 || true
fi
