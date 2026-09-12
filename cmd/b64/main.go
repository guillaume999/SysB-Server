// cmd/b64 — 500 tirages aleatoires encodes par Go, pour que le JS les relise.
// ⚠️ C'est le seul encodage que Unity, PocketBase et les deux moteurs doivent
// lire PAREIL : une divergence ici deplace tout le terrain d'une case.
package main

import (
	"encoding/json"
	"fmt"
	"math/rand"

	"sysb/moteur"
)

func main() {
	r := rand.New(rand.NewSource(20260912))
	type cas struct {
		Octets []int  `json:"octets"`
		B64    string `json:"b64"`
	}
	var out []cas
	for n := 0; n < 500; n++ {
		src := make([]int, r.Intn(300))
		for i := range src {
			src[i] = r.Intn(256)
		}
		out = append(out, cas{src, moteur.OctetsVersBase64(src)})
	}
	b, _ := json.Marshal(out)
	fmt.Println(string(b))
}
