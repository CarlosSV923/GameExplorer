# Throwaway image for the phase 0.5 spike.
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /spike .

# Official 7-Zip build: distro packages (Alpine, Debian main) ship 7zz WITHOUT
# the RAR codec for licensing reasons, and most game archives are RAR5.
FROM alpine:3 AS sevenzip
ARG SEVENZIP_VERSION=2601
ARG SEVENZIP_SHA256=8ea0fc8a135e7b848e80a4116fe22dff56c8c4518dde1f43cce67f4e340b437a
RUN apk add --no-cache xz tar \
 && wget -qO /tmp/7z.tar.xz "https://www.7-zip.org/a/7z${SEVENZIP_VERSION}-linux-x64.tar.xz" \
 && echo "${SEVENZIP_SHA256}  /tmp/7z.tar.xz" | sha256sum -c - \
 && tar -xJf /tmp/7z.tar.xz -C /tmp 7zzs License.txt

FROM alpine:3
RUN apk add --no-cache ca-certificates
COPY --from=sevenzip /tmp/7zzs /usr/local/bin/7zz
COPY --from=sevenzip /tmp/License.txt /usr/share/licenses/7zip/License.txt
COPY --from=build /spike /usr/local/bin/spike
EXPOSE 8090
ENTRYPOINT ["/usr/local/bin/spike"]
