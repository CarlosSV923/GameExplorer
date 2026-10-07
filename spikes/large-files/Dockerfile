# Throwaway image for the phase 0.5 spike.
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /spike .

FROM alpine:3
RUN apk add --no-cache 7zip ca-certificates
COPY --from=build /spike /usr/local/bin/spike
EXPOSE 8090
ENTRYPOINT ["/usr/local/bin/spike"]
