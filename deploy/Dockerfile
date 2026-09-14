FROM node:22-alpine AS assets
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --ignore-scripts
COPY web/ ./
RUN npm run build

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
COPY --from=assets /src/web/static/ ./web/static/
RUN go run github.com/a-h/templ/cmd/templ@v0.3.1020 generate
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/mgp-server ./cmd/mgp-server

FROM alpine:3.22
RUN addgroup -S mgp && adduser -S -G mgp mgp && apk add --no-cache ca-certificates wget
COPY --from=build /out/mgp-server /usr/local/bin/mgp-server
RUN mkdir -p /var/lib/mgp/documents && chown -R mgp:mgp /var/lib/mgp
USER mgp
ENV MGP_ADDR=:8080 MGP_DOCUMENT_DIR=/var/lib/mgp/documents
VOLUME ["/var/lib/mgp/documents"]
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --retries=5 CMD wget -qO- http://127.0.0.1:8080/health/live || exit 1
ENTRYPOINT ["/usr/local/bin/mgp-server"]
