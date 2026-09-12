// cmd/banc — le meme banc que sous node et sous goja, pour comparer ce qui est
// comparable : la meme colonie, la meme absence, le meme moteur.
package main

import (
	"fmt"
	"time"

	"sysb/moteur"
)

func tuile(g *moteur.Genres, brut moteur.Brut) *moteur.Tuile {
	t, err := moteur.ChargerTuile(g, brut)
	if err != nil {
		panic(err)
	}
	return t
}

func main() {
	genres := moteur.CreerGenres(map[string]string{
		"ble": "stock", "pain": "stock", "population": "mobilise", "satisfaction": "indicateur"})

	appro := func(sens string) []any {
		return []any{moteur.Brut{"sens": sens, "cible": "tout", "rayon": 6.0,
			"ressources": []any{"ble"},
			"debit":      moteur.Brut{"navettes": 2.0, "quantite": 50.0},
			"vitesse":    moteur.Brut{"crans": 1.0, "periode_s": 20.0}}}
	}
	ferme := tuile(genres, moteur.Brut{"tid": 1.0, "nom": "ferme", "cycle_minutes": 1.0,
		"production": []any{moteur.Brut{"ressource": "ble", "quantite": 10.0}},
		"stockage":   moteur.Brut{"*": 100000.0}, "appros": appro("envoi")})
	four := tuile(genres, moteur.Brut{"tid": 2.0, "nom": "four", "cycle_minutes": 1.0,
		"utilisation": []any{moteur.Brut{"ressource": "ble", "quantite": 5.0}},
		"production":  []any{moteur.Brut{"ressource": "pain", "quantite": 1.0}},
		"stockage":    moteur.Brut{"*": 100000.0}, "appros": appro("entrant")})
	moteur.Refiger(genres, []*moteur.Tuile{ferme, four})

	banc := func(n, jours int) time.Duration {
		var bats []*moteur.Batiment
		for i := 0; i < n; i++ {
			t := ferme
			if i%2 == 1 {
				t = four
			}
			bats = append(bats, moteur.CreerBatiment(genres,
				moteur.Brut{"x": float64(i % 20), "z": float64(i / 20)}, t))
		}
		p := moteur.CreerPlateau(0, bats, nil, genres, nil, nil)
		t0 := time.Now()
		if err := moteur.Avancer(p, jours*86400); err != nil {
			panic(err)
		}
		return time.Since(t0)
	}

	for _, c := range [][2]int{{10, 1}, {20, 1}, {40, 1}, {60, 1}, {60, 7}, {200, 7}, {200, 30}} {
		fmt.Printf("Go   %3d batiments  %2d jour(s)  ->  %v\n", c[0], c[1], banc(c[0], c[1]).Round(time.Millisecond))
	}
}
