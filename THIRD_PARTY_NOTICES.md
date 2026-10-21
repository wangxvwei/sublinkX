# CloudflareSpeedTest

The Docker image includes the unmodified CloudflareSpeedTest v2.3.5 command-line
program by XIU2, invoked as a separate process for CDN measurements.

Project: https://github.com/XIU2/CloudflareSpeedTest/tree/v2.3.5

Its GNU GPL version 3 license is included at
`/usr/share/doc/cloudflare-speed-test/LICENSE`. Its corresponding source,
including vendored Go dependencies and their licenses, is included in the same
image at `/usr/share/doc/cloudflare-speed-test/source.tar.gz`.

Extract that archive and build with Go 1.22.2:

```sh
CGO_ENABLED=0 go build -mod=vendor -trimpath -ldflags="-s -w -X main.version=v2.3.5" -o cfst .
```

No modifications have been made to CloudflareSpeedTest. The exact image build
steps are recorded in this repository's Dockerfile.
