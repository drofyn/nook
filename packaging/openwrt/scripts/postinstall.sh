#!/bin/sh
set -eu

DEFAULT_CONF=/usr/share/nook/nook.config

[ -f "$DEFAULT_CONF" ] || exit 0

[ -f /etc/config/nook ] || cp "$DEFAULT_CONF" /etc/config/nook

while IFS= read -r line; do
	case "$line" in
		option*)
			key=$(printf '%s\n' "$line" | sed -n 's/^[[:space:]]*option \([^ ]*\) .*/\1/p')
			val=$(printf '%s\n' "$line" | sed -n "s/^[[:space:]]*option \([^ ]*\) '\(.*\)'.*/\2/p")
			[ -n "$key" ] || continue
			if [ -z "$(uci -q get "nook.main.$key")" ]; then
				uci set "nook.main.$key=$val"
			fi
			;;
	esac
done < "$DEFAULT_CONF"

uci commit nook 2>/dev/null || true

rm -f /etc/config/nook-opkg /etc/config/nook.apk-new

if [ -x /etc/init.d/nook ] && /etc/init.d/nook running >/dev/null 2>&1; then
	/etc/init.d/nook restart >/dev/null 2>&1 || true
fi
