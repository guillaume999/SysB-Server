// harnais — ce que `rejouer`, `photos` et `banc` font pareil : lire le fichier,
// charger le catalogue ENTIER, monter un plateau.
//
// ⚠️ UN SEUL HARNAIS, comme en JS (`rejeu.js` sert la ligne de commande ET la
// route `banc-vecteurs.pb.js`). Deux harnais auraient donne deux facons de
// mesurer, et un ecart qu'on aurait mis des heures a comprendre.
package harnais

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"sysb/moteur"
)

func LireJson(chemin string) moteur.Brut {
	brut, err := os.ReadFile(chemin)
	if err != nil {
		panic(err)
	}
	var d moteur.Brut
	if err := json.Unmarshal(brut, &d); err != nil {
		panic(err)
	}
	return d
}

func Liste(b moteur.Brut, nom string) []any {
	if l, ok := b[nom].([]any); ok {
		return l
	}
	return nil
}

func Entier(b moteur.Brut, nom string) int {
	if f, ok := b[nom].(float64); ok {
		return int(f)
	}
	return 0
}

func VersSac(g *moteur.Genres, b moteur.Brut) moteur.Sac {
	s := moteur.NouveauSac(g.Reg)
	for k, v := range b {
		f, _ := v.(float64)
		s.Set(g.Reg.Inscrire(k), int(f))
	}
	return s
}

// chargerCatalogue — ⚠️⚠️ TOUT LE CATALOGUE D'UN COUP, PUIS `Refiger`.
//
// Le JS chargeait les tuiles A LA DEMANDE, et il pouvait se le permettre : ses
// cles etaient des chaines. Ici tout est indexe par un entier attribue au
// chargement — une tuile chargee avant qu'un code n'existe porterait un tableau
// de stockage trop court, en silence. On ferme donc le catalogue avant de jouer.
//
// ⚠️ Une tuile REFUSEE n'arrete pas le chargement : elle garde son erreur, et
// c'est le scenario qui l'utilise qui tombe — comme en JS, ou le `throw` de
// `tuileDe` etait rattrape scenario par scenario.
func ChargerCatalogue(d moteur.Brut) (*moteur.Genres, map[string]*moteur.Tuile, map[string]error) {
	genresBruts := map[string]string{}
	if g, ok := d["genres"].(moteur.Brut); ok {
		for k, v := range g {
			s, _ := v.(string)
			genresBruts[k] = s
		}
	}
	genres := moteur.CreerGenres(genresBruts)

	brutes, _ := d["tuiles"].(moteur.Brut)
	noms := make([]string, 0, len(brutes))
	for n := range brutes {
		noms = append(noms, n)
	}
	sort.Strings(noms) // ⚠️ ordre stable : il decide l'ordre d'inscription des codes

	tuiles := map[string]*moteur.Tuile{}
	erreurs := map[string]error{}
	var toutes []*moteur.Tuile
	for _, n := range noms {
		tb, _ := brutes[n].(moteur.Brut)
		t, err := moteur.ChargerTuile(genres, tb)
		if err != nil {
			erreurs[n] = err
			continue
		}
		tuiles[n] = t
		toutes = append(toutes, t)
	}
	moteur.Refiger(genres, toutes)
	return genres, tuiles, erreurs
}

func Monter(g *moteur.Genres, tuiles map[string]*moteur.Tuile, erreurs map[string]error, s moteur.Brut) (*moteur.Plateau, error) {
	var bats []*moteur.Batiment
	for _, c := range Liste(s, "cases") {
		cb, _ := c.(moteur.Brut)
		nom, _ := cb["tuile"].(string)
		t, ok := tuiles[nom]
		if !ok {
			if e, y := erreurs[nom]; y {
				return nil, e
			}
			return nil, fmt.Errorf("tuile inconnue « %s »", nom)
		}
		bats = append(bats, moteur.CreerBatiment(g, cb, t))
	}
	// ⚠️ LE `sol` EST OBLIGATOIRE : les forets de S10 n'ont aucun etat, donc
	// aucun batiment. Sans lui la proximite compte zero — le piege du 31/08.
	var sol []moteur.CaseSol
	for _, v := range Liste(s, "sol") {
		sb, _ := v.(moteur.Brut)
		sol = append(sol, moteur.CaseSol{
			X: Entier(sb, "x"), Z: Entier(sb, "z"), Tid: Entier(sb, "tid")})
	}
	return moteur.CreerPlateau(0, bats, nil, g, nil, sol), nil
}
