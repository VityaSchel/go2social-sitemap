# Building go2social-sitemap

Requirements: Go 1.27+

Run `go build -trimpath -ldflags='-s -w'`

## Reproducible builds

```sh
#!/bin/sh
set -eu
export LC_ALL=C
for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64 \
      freebsd/amd64 freebsd/arm64 openbsd/amd64 openbsd/arm64 netbsd/amd64 netbsd/arm64; do
      os=${target%/*} arch=${target#*/} ext=
      if [ "$os" = windows ]; then ext=.exe; fi
      env -i PATH="$PATH" HOME="$HOME" GOENV=off GOTOOLCHAIN=go1.27.1 CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
              go build -trimpath -buildvcs=false -ldflags='-s -w' -o "dist/go2social-sitemap_${os}_${arch}${ext}" .
done
(cd dist && shasum -a 256 go2social-sitemap_* > SHA256SUMS)
```
