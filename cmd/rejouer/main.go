// ============================================================
//  cmd/rejouer — LES VECTEURS, SUR LE MOTEUR Go
//
//      go run ./cmd/rejouer <chemin/scenarios.json>
//
//  ⚠️ MEMES DONNEES, MEME JUGEMENT que `pb_hooks/moteur/cycles/rejeu.js` : ce
//  qui est compare ici, ce n'est pas « est-ce que le Go marche », c'est « est-ce
//  que le Go dit EXACTEMENT ce que dit le JS ».
//
//  ⚠️ ETAPE 1 : seules les familles de TEMPS sont portees. `placement` et
//  `geste` sont comptes a part, pas juges.
// ============================================================

package main

import (
	"fmt"
	"os"

	"sysb/harnais"
	"sysb/moteur"
)

type ecart struct {
	Scenario, Mesure, Titre string
	Attendu, Vu, Tol        float64
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: rejouer <scenarios.json>")
		os.Exit(2)
	}
	d := harnais.LireJson(os.Args[1])
	genres, tuiles, erreurs := harnais.ChargerCatalogue(d)

	var cadences []int
	for _, v := range harnais.Liste(d, "cadences") {
		f, _ := v.(float64)
		cadences = append(cadences, int(f))
	}

	var ecarts []ecart
	var casses []string
	jugees, collees, horsPerimetre, sautes := 0, 0, 0, 0

	for _, sv := range harnais.Liste(d, "scenarios") {
		s, _ := sv.(moteur.Brut)
		nom, _ := s["nom"].(string)
		titre, _ := s["titre"].(string)

		var aJuger []moteur.Brut
		for _, mv := range harnais.Liste(s, "mesures") {
			mb, _ := mv.(moteur.Brut)
			if _, y := mb["attendu"]; y {
				aJuger = append(aJuger, mb)
			}
		}
		if len(aJuger) == 0 {
			sautes++
			continue
		}
		// ─── PLACEMENT : aucune horloge, un verdict et un devis ───────────
		if _, y := s["placement"]; y {
			r := harnais.JouerPlacement(d, tuiles, s)
			for _, m := range aJuger {
				vu, ok := harnais.MesurerPlacement(genres, m, r)
				jugees++
				att, _ := m["attendu"].(float64)
				tol, _ := m["tolerance"].(float64)
				if ok && diff(vu, att) <= tol {
					collees++
					continue
				}
				mn, _ := m["nom"].(string)
				ecarts = append(ecarts, ecart{nom, mn, titre, att, vu, tol})
			}
			continue
		}

		// ─── GESTE : on pose ou on detruit UNE fois, a `maintenant` ────────
		if _, y := s["geste"]; y {
			j, err := harnais.JouerGeste(d, genres, tuiles, erreurs, s)
			if err != nil {
				casses = append(casses, nom+" : "+err.Error())
				continue
			}
			for _, m := range aJuger {
				vu, ok := harnais.MesurerGeste(genres, m, j)
				if !ok {
					// ⚠️ Le defaut du JS : une mesure de geste non reconnue
					// retombe sur la mesure de TEMPS, sur le plateau d'apres.
					vu = mesurer(genres, j.Plateau, m, j.T, false, moteur.NouveauSac(genres.Reg))
				}
				jugees++
				att, _ := m["attendu"].(float64)
				tol, _ := m["tolerance"].(float64)
				if diff(vu, att) <= tol {
					collees++
					continue
				}
				mn, _ := m["nom"].(string)
				ecarts = append(ecarts, ecart{nom, mn, titre, att, vu, tol})
			}
			continue
		}

		duree := 3600
		if v, ok := s["duree"].(float64); ok {
			duree = int(v)
		}
		p, err := harnais.Monter(genres, tuiles, erreurs, s)
		if err != nil {
			casses = append(casses, nom+" : "+err.Error())
			continue
		}
		if err := moteur.Avancer(p, duree); err != nil {
			casses = append(casses, nom+" : "+err.Error())
			continue
		}

		// ⚠️ `apres` s'applique APRES la passe, sur le plateau resolu : c'est ce
		// que le joueur ferait en arrivant.
		debitReussi := false
		perdu := moteur.NouveauSac(genres.Reg)
		if ap, ok := s["apres"].(moteur.Brut); ok {
			if m, ok := ap["debiter"].(moteur.Brut); ok {
				debitReussi, _ = moteur.Debiter(p, harnais.VersSac(genres, m), duree)
			}
			if m, ok := ap["crediter"].(moteur.Brut); ok {
				perdu = moteur.Crediter(p, harnais.VersSac(genres, m), duree)
			}
		}

		for _, m := range aJuger {
			vu := mesurer(genres, p, m, duree, debitReussi, perdu)
			att, _ := m["attendu"].(float64)
			tol, _ := m["tolerance"].(float64)
			jugees++
			if diff(vu, att) <= tol {
				collees++
				continue
			}
			mn, _ := m["nom"].(string)
			ecarts = append(ecarts, ecart{nom, mn, titre, att, vu, tol})
		}

		// ⚠️ L'INVARIANCE AUX CADENCES : le meme intervalle, joue en morceaux,
		// doit rendre le MEME etat. Elle se juge SANS `apres`.
		temoinP, _ := harnais.Monter(genres, tuiles, erreurs, s)
		moteur.Avancer(temoinP, duree)
		temoin := moteur.Photo(temoinP)
		for _, pas := range cadences {
			if pas <= 0 || pas > duree {
				continue
			}
			pp, _ := harnais.Monter(genres, tuiles, erreurs, s)
			for t := pas; t <= duree; t += pas {
				moteur.Avancer(pp, t)
			}
			moteur.Avancer(pp, duree)
			if moteur.Photo(pp) == temoin {
				continue
			}
			ecarts = append(ecarts, ecart{nom,
				fmt.Sprintf("INVARIANCE au pas de %d s", pas), titre, 1, 0, 0})
			break
		}
	}

	fmt.Printf("\n%d mesures jugees par le moteur Go\n", jugees)
	fmt.Printf("  collent        : %d\n", collees)
	fmt.Printf("  ECARTS         : %d\n", len(ecarts))
	_ = horsPerimetre
	fmt.Printf("  scenarios sans aucun `attendu`              : %d\n", sautes)
	if len(casses) > 0 {
		fmt.Printf("\n%d SCENARIOS QUI N'ONT PAS TOURNE :\n", len(casses))
		for _, c := range casses {
			fmt.Println("  " + c)
		}
	}
	if len(ecarts) > 0 {
		fmt.Println("\n=== LES ECARTS, UN PAR LIGNE ===")
		dernier := ""
		for _, e := range ecarts {
			if e.Scenario != dernier {
				fmt.Printf("\n%s — %s\n", e.Scenario, e.Titre)
				dernier = e.Scenario
			}
			fmt.Printf("   %s\n      attendu %g  (tolerance %g)   vu %g   ecart %g\n",
				e.Mesure, e.Attendu, e.Tol, e.Vu, e.Vu-e.Attendu)
		}
	}
}

func diff(a, b float64) float64 {
	if a > b {
		return a - b
	}
	return b - a
}

// mesurer : le pendant Go de `mesurerTemps` (rejeu.js).
func mesurer(g *moteur.Genres, p *moteur.Plateau, m moteur.Brut, t int, debitReussi bool, perdu moteur.Sac) float64 {
	nom, _ := m["ressource"].(string)
	c := g.Reg.Id(nom)
	typ, _ := m["type"].(string)
	switch typ {
	case "reserve":
		return float64(p.Reserve.Get(c))
	case "case":
		b := p.BatimentEn(harnais.Entier(m, "x"), harnais.Entier(m, "z"))
		if b == nil {
			return -1e18 // « null » cote JS : aucune valeur ne collera
		}
		return float64(b.Stock.Get(c))
	case "total":
		s := p.Reserve.Get(c)
		for _, b := range p.Batiments {
			s += b.Stock.Get(c)
		}
		return float64(s)
	case "disponible":
		s := moteur.Disponible(p, t)
		return float64(s.Get(c))
	case "en_attente":
		s := moteur.EnAttente(p, t)
		return float64(s.Get(c))
	case "places":
		s := moteur.Places(p, t)
		return float64(s.Get(c))
	case "capacite":
		s := moteur.Capacite(p, t, nil)
		return float64(s.Get(c))
	case "mobilise":
		s := moteur.Mobilise(p, t)
		return float64(s.Get(c))
	case "libre":
		return float64(moteur.Libre(moteur.Disponible(p, t), moteur.Mobilise(p, t), c))
	case "debit_reussi":
		if debitReussi {
			return 1
		}
		return 0
	case "perdu":
		return float64(perdu.Get(c))
	}
	return -1e18
}
