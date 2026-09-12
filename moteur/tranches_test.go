package moteur

import "testing"

// ⚠️⚠️ LA PROPRIETE QUI REND LE RATTRAPAGE PAR TRANCHES LICITE : rejouer le meme
// intervalle en N appels budgetes doit rendre EXACTEMENT le meme etat qu'en un
// seul appel. Si ce n'est pas vrai, s'arreter en chemin change le jeu, et la
// reponse au §11.1 ne vaut rien.
//
// C'est le cousin de l'invariance aux cadences (deja verte sur les 28
// scenarios), mais il ne la remplace pas : la cadence decoupe en CHOISISSANT
// des instants ronds, le budget s'arrete a une rupture quelconque, au milieu
// d'un vol de navette.
func TestRattrapageParTranchesRendLeMemeEtat(t *testing.T) {
	const jours = 1
	const jusqua = jours * 86400

	monter := func() *Plateau {
		g := CreerGenres(map[string]string{
			"ble": "stock", "pain": "stock", "population": "mobilise", "satisfaction": "indicateur"})
		appro := func(sens string) []any {
			return []any{Brut{"sens": sens, "cible": "tout", "rayon": 6.0,
				"ressources": []any{"ble"},
				"debit":      Brut{"navettes": 2.0, "quantite": 50.0},
				"vitesse":    Brut{"crans": 1.0, "periode_s": 20.0}}}
		}
		ferme, err := ChargerTuile(g, Brut{"tid": 1.0, "nom": "ferme", "cycle_minutes": 1.0,
			"production": []any{Brut{"ressource": "ble", "quantite": 10.0}},
			"stockage":   Brut{"*": 100000.0}, "appros": appro("envoi")})
		if err != nil {
			t.Fatal(err)
		}
		four, err := ChargerTuile(g, Brut{"tid": 2.0, "nom": "four", "cycle_minutes": 1.0,
			"utilisation": []any{Brut{"ressource": "ble", "quantite": 5.0}},
			"production":  []any{Brut{"ressource": "pain", "quantite": 1.0}},
			"stockage":    Brut{"*": 100000.0}, "appros": appro("entrant")})
		if err != nil {
			t.Fatal(err)
		}
		Refiger(g, []*Tuile{ferme, four})
		var bats []*Batiment
		for i := 0; i < 24; i++ {
			tu := ferme
			if i%2 == 1 {
				tu = four
			}
			bats = append(bats, CreerBatiment(g,
				Brut{"x": float64(i % 8), "z": float64(i / 8)}, tu))
		}
		return CreerPlateau(0, bats, nil, g, nil, nil)
	}

	// D'un seul coup — le temoin.
	temoinP := monter()
	if pr := AvancerBudget(temoinP, jusqua, 0); !pr.Fini {
		t.Fatalf("le temoin devrait finir, il s'est arrete a %d", pr.T)
	}
	temoin := Photo(temoinP)

	// Par tranches, avec des budgets volontairement mechants : 1 evenement a la
	// fois, puis 7, puis 97 — des nombres qui ne tombent jamais sur une
	// frontiere ronde.
	for _, budget := range []int{1, 7, 97} {
		p := monter()
		appels, garde := 0, 0
		for {
			pr := AvancerBudget(p, jusqua, budget)
			appels++
			if pr.Fini {
				break
			}
			// ⚠️ Le contrat : tant que ce n'est pas fini, `T` a AVANCE. Sans ca
			// l'appelant bouclerait sans progresser — le meme enfermement que
			// l'ancienne exception, en plus discret.
			if pr.T != p.T {
				t.Fatalf("budget %d : Progres.T (%d) et plateau.T (%d) divergent",
					budget, pr.T, p.T)
			}
			garde++
			if garde > 1000000 {
				t.Fatalf("budget %d : ca ne progresse pas", budget)
			}
		}
		if got := Photo(p); got != temoin {
			t.Errorf("budget %d (%d appels) : l'etat DIFFERE du rattrapage en un coup",
				budget, appels)
		} else {
			t.Logf("budget %d : %d appels, etat identique", budget, appels)
		}
		if p.T != jusqua {
			t.Errorf("budget %d : t final = %d, attendu %d", budget, p.T, jusqua)
		}
	}
}

// Une garde qui condamne est pire qu'une garde qui degrade : on verifie que le
// budget rend bien la main SANS perdre le travail deja fait.
func TestBudgetNePerdPasLeTravailFait(t *testing.T) {
	g := CreerGenres(map[string]string{"ble": "stock"})
	ferme, err := ChargerTuile(g, Brut{"tid": 1.0, "nom": "ferme", "cycle_minutes": 1.0,
		"production": []any{Brut{"ressource": "ble", "quantite": 10.0}},
		"stockage":   Brut{"*": 1000000.0}})
	if err != nil {
		t.Fatal(err)
	}
	Refiger(g, []*Tuile{ferme})
	b := CreerBatiment(g, Brut{"x": 0.0, "z": 0.0}, ferme)
	p := CreerPlateau(0, []*Batiment{b}, nil, g, nil, nil)

	pr := AvancerBudget(p, 3600, 10) // 10 cycles d'une minute, pas 60
	if pr.Fini {
		t.Fatal("10 evenements ne peuvent pas couvrir une heure de cycles d'une minute")
	}
	if pr.T != 600 {
		t.Errorf("t atteint = %d, attendu 600 (10 cycles de 60 s)", pr.T)
	}
	if got := b.Stock.Get(g.Reg.Id("ble")); got != 100 {
		t.Errorf("ble produit = %d, attendu 100 — le travail fait doit etre GARDE", got)
	}
	if p.T != pr.T {
		t.Errorf("le plateau doit porter le t atteint (%d), il porte %d", pr.T, p.T)
	}
}
