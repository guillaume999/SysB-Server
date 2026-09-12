module sysb

go 1.25.0

// ⚠️ POCKETBASE EST PINGLE sur la version que sert l'instance — v0.39.2, lue en
// bas de l'admin le 12/09. Ne pas suivre `latest` : un binaire maison sur une
// base qui bouge, c'est un matin ou plus rien ne compile.
//
// ⚠️ CE FICHIER EST ECRIT PAR `go mod tidy`, PAS A LA MAIN. Le bloc `indirect`
// plus bas et la ligne `go 1.25.0` viennent de lui. Le 12/09, une version
// abregee de ce fichier est partie sur GitHub avec le `go.sum` complet : les
// deux ne se correspondaient plus et la construction de l'image a echoue en
// cinq secondes. `go.mod` et `go.sum` voyagent ENSEMBLE, toujours.
require github.com/pocketbase/pocketbase v0.39.2

require (
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2 // indirect
	github.com/disintegration/imaging v1.6.2 // indirect
	github.com/domodwyer/mailyak/v3 v3.6.2 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/fatih/color v1.19.0 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.13 // indirect
	github.com/ganigeorgiev/fexpr v0.5.0 // indirect
	github.com/go-ozzo/ozzo-validation/v4 v4.3.0 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.22 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/pocketbase/dbx v1.12.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/spf13/cobra v1.10.2 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	golang.org/x/crypto v0.52.0 // indirect
	golang.org/x/image v0.41.0 // indirect
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/oauth2 v0.36.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/text v0.37.0 // indirect
	modernc.org/libc v1.72.3 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.11.0 // indirect
	modernc.org/sqlite v1.52.0 // indirect
)
