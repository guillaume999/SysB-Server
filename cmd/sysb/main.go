//go:build pocketbase

// ============================================================
//  cmd/sysb — LE BINAIRE POCKETBASE DE SysB
//
//  ⚠️ JAMAIS COMPILE DANS UNE SESSION CLAUDE — voir l'en-tete de
//  `pb/adaptateur.go`. Chez toi :
//
//      cd SysB/serveur-go
//      go mod tidy                       # va chercher PocketBase
//      go build -tags pocketbase -o sysb ./cmd/sysb
//      ./sysb serve --http=0.0.0.0:8090
//
//  ⚠️ `pb_data` NE BOUGE PAS. C'est le meme SQLite que l'image
//  `ghcr.io/muchobien/pocketbase` utilisait : rien a migrer, et on peut
//  revenir a l'ancienne image en reposant le compose d'avant.
//
//  ⚠️ CE QUI CHANGE, EN REVANCHE : les routes `/api/sysb/*` ne viennent plus de
//  `pb_hooks/*.pb.js` mais de ce binaire. **Sors les anciens hooks du dossier
//  monte** — deux routes du meme nom, c'est la meme classe de panne que deux
//  ecrivains sur `plateaux`.
// ============================================================

package main

import (
	"log"

	"github.com/pocketbase/pocketbase"

	"sysb/pb"
)

func main() {
	app := pocketbase.New()
	pb.Brancher(app)
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
