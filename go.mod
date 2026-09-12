module sysb

go 1.24

// ⚠️ POCKETBASE EST PINGLE sur la version que sert l'instance — v0.39.2, lue en
// bas de l'admin le 12/09. Ne pas suivre `latest` : un binaire maison sur une
// base qui bouge, c'est un matin ou plus rien ne compile.
//
// `go mod tidy` va chercher le module et ecrit `go.sum`. Si tidy reclame une
// version de Go plus recente que la ligne `go 1.24` ci-dessus, c'est ce chiffre
// qu'il faut remonter (et la ligne `golang:1.27-alpine` du Dockerfile qui suit).
require github.com/pocketbase/pocketbase v0.39.2
