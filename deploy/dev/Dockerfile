# Development/test image: the Go toolchain plus the official 7-Zip (with RAR).
# Built by `task dev:image`; used by compose.dev.yaml and the Taskfile's Go tasks.
FROM golang:1.27

RUN apt-get update \
 && apt-get install -y --no-install-recommends xz-utils \
 && rm -rf /var/lib/apt/lists/*

COPY scripts/install-7zip.sh /tmp/install-7zip.sh
RUN sh /tmp/install-7zip.sh && rm /tmp/install-7zip.sh

# Tests that need 7zz fail instead of being skipped inside this image.
ENV REQUIRE_7ZZ=1
