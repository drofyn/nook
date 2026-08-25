# AGENTS.md

## Project

Nook is a Go-based LAN collaboration service intended to run on OpenWrt/ImmortalWrt routers.

The project currently contains one application package. The web UI is embedded into the Go binary with `embed.FS`.

## Conventions

- Product name: `Nook`
- Binary/package/service/config name: `nook`
- Keep OpenWrt paths consistent:
  - `/usr/bin/nook`
  - `/etc/init.d/nook`
  - `/etc/config/nook`
- Do not reintroduce `routerdrop`.
- Avoid adding code comments unless explicitly requested.
- Prefer simple standard-library Go where possible.
- Keep the frontend dependency-free unless explicitly requested.

## Validation

After code changes, run:

```sh
go test ./...
```

After build or packaging changes, also run:

```sh
sh scripts/build-openwrt.sh arm64
sh scripts/build-openwrt.sh x86_64
sh scripts/build-packages.sh
```

Packaging uses nFPM and requires the `nfpm` command to be installed. CI installs nFPM with Go 1.25.x. The packaging script removes and recreates `dist/`.

## OpenWrt packaging

Packages are built by `scripts/build-packages.sh` and output:

```text
dist/nook_<version>-1_aarch64_cortex-a53.ipk
dist/nook_<version>-1_x86_64.ipk
dist/nook_<version>-r1_aarch64_cortex-a53.apk
dist/nook_<version>-r1_x86_64.apk
```

OpenWrt service files live in:

```text
packaging/openwrt/files/
```

Expected files:

```text
nook.init
nook.config
```

## Notes

- The WebSocket endpoint is `/ws`.
- The config endpoint is `/config` and includes the UI version and WebRTC config.
- Default listen address is `0.0.0.0:8088`.
- WebRTC uses no ICE servers by default for LAN-only operation.
