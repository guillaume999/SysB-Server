# ============================================================
#  L'IMAGE POCKETBASE DE SysB
#
#  Construite par GitHub Actions, pas a la main : voir
#  `.github/workflows/image.yml`. Le resultat atterrit sur
#  `ghcr.io/guillaume999/sysb-pocketbase:latest`, et Portainer le tire — comme
#  il tirait `ghcr.io/muchobien/pocketbase:latest` avant.
#
#  ⚠️ POCKETBASE EST PINGLE a v0.39.2 dans `go.mod` — la version que sert
#  l'instance. Ne pas suivre `latest` : un binaire maison sur une base qui
#  bouge, c'est un matin ou plus rien ne compile.
#
#  ⚠️ `--platform=$BUILDPLATFORM` + `GOARCH=$TARGETARCH` : Go compile pour une
#  autre architecture sans rien emuler. L'image amd64 et l'image arm64 sortent
#  toutes les deux a la vitesse d'une compilation normale — pas de QEMU, pas de
#  vingt minutes d'attente. C'est gratuit uniquement parce que CGO est coupe.
# ============================================================

FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# CGO_ENABLED=0 : PocketBase utilise `modernc.org/sqlite`, du SQLite en Go pur.
# Rien a lier, donc image finale minuscule ET compilation croisee triviale.
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build \
    -tags pocketbase -trimpath -ldflags="-s -w" -o /out/sysb ./cmd/sysb

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=build /out/sysb /usr/local/bin/sysb
# ⚠️ `pb_data` est un VOLUME : la base ne vit pas dans l'image. C'est ce qui
# permet de revenir a l'ancienne image sans rien perdre.
VOLUME ["/pb_data"]
EXPOSE 8090
ENTRYPOINT ["/usr/local/bin/sysb"]
CMD ["serve", "--http=0.0.0.0:8090", "--dir=/pb_data"]
