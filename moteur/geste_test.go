package moteur

import "testing"

// ─── LA GRATUITE, JUSQU'AU PRELEVEMENT ──────────────────────────────────────
//
// ⚠️⚠️ CE FICHIER EXISTE A CAUSE DU 13/09 : le magasin annoncait « OFFERT », et
// la pose repondait « il manque 150 bois ». Les DEUX lecteurs de la regle
// `gratuite` — `CoutConstruction` cote Unity, `Estimer` ici — etaient d'accord ;
// c'est `PayerLaPose` qui reprenait le cout PLEIN de la tuile des que le devis
// offert se trouvait VIDE. Une habitation payee en bois et rien d'autre n'a
// aucune ligne `mobilise` : son devis offert etait donc `nil`, et un repli
// « nil = pas de devis, reprends `b.Tuile.Cout` » facturait le cadeau.
//
// ⚠️ L'ESSAI QUI MANQUAIT EST CELUI-LA : une tuile offerte SANS AUCUNE LIGNE
// `mobilise`. Une tuile offerte qui mobilise du monde rendait une liste non
// vide, donc passait — exactement le piege « vert pour rien » du meme jour sur
// `RefuseesTriees`, ou « jamais nil » teste avec UNE entree ne distinguait pas
// `var out []T` de `make(...)`.

// habitationOfferte : 150 bois a payer, les `offerts` premieres offertes.
// `mobilise` ajoute en plus une ligne de population mobilisee.
func habitationOfferte(t *testing.T, offerts int, mobilise bool) (*Genres, *Tuile) {
	t.Helper()
	g := CreerGenres(map[string]string{"bois": "stock", "population": "stock"})

	cout := []any{Brut{"ressource": "bois", "quantite": 150.0, "mode": "paye"}}
	if mobilise {
		cout = append(cout, Brut{"ressource": "population", "quantite": 2.0,
			"mode": "mobilise"})
	}
	tuile, err := ChargerTuile(g, Brut{
		"tid": 1.0, "nom": "Habitations Bois",
		"cout":       cout,
		"stockage":   Brut{"*": 100.0},
		"placements": []any{Brut{"regle": "gratuite", "offerts": float64(offerts)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	Refiger(g, []*Tuile{tuile})
	return g, tuile
}

// terrainVide : un sol nu, des coffres vides, et les batiments deja poses.
func terrainVide(g *Genres, deja []*Batiment, maintenant int) Terrain {
	p := CreerPlateau(maintenant, deja, map[string]int{}, g, nil, nil)
	return NouveauTerrainDesCases(nil, p, "ground")
}

// ⚠️ LE CAS DU BUG : offerte, aucune ligne `mobilise`, pas un seul bois en
// reserve. La pose doit passer, et ne RIEN prelever.
func TestUnePoseOfferteSansMobiliseNeSePaiePas(t *testing.T) {
	g, tuile := habitationOfferte(t, 2, false)
	cat := map[int]*Tuile{1: tuile}
	terrain := terrainVide(g, nil, 1000)

	res := Poser(terrain, cat, 3, 3, 1, 1000, OptionsGeste{})

	if !res.Ok {
		t.Fatalf("une pose offerte doit passer les coffres vides, refus : %q", res.Refus)
	}
	if !res.Offert {
		t.Fatal("la pose devait etre marquee offerte")
	}
	if n := res.Paye.Get(g.Reg.Id("bois")); n != 0 {
		t.Fatalf("un cadeau ne se facture pas : %d bois preleves", n)
	}
	if terrain.Tid(3, 3) != 1 {
		t.Fatal("la case devait porter la tuile posee")
	}
}

// La meme, avec du personnel : le `mobilise` reste exige (OFFERT NE VEUT PAS
// DIRE « SANS PERSONNEL »), mais le `paye` ne l'est toujours pas.
func TestUnePoseOfferteGardeSonPersonnel(t *testing.T) {
	g, tuile := habitationOfferte(t, 2, true)
	cat := map[int]*Tuile{1: tuile}
	terrain := terrainVide(g, nil, 1000)

	res := Poser(terrain, cat, 3, 3, 1, 1000, OptionsGeste{})

	if res.Ok {
		t.Fatal("sans population libre, la pose offerte doit etre refusee")
	}
	if len(res.Blocages) != 1 {
		t.Fatalf("un seul blocage attendu (la population), recu %v", res.Blocages)
	}
	// ⚠️ Le bois n'a RIEN a faire dans le refus : il est offert.
	for _, b := range res.Blocages {
		if contientTexte(b, "bois") {
			t.Fatalf("le bois est offert, il ne peut pas manquer : %q", b)
		}
	}
}

// Une fois les offertes epuisees, le prix revient — c'est la « fiche normale ».
func TestApresLesOffertesLePrixRevient(t *testing.T) {
	g, tuile := habitationOfferte(t, 2, false)
	cat := map[int]*Tuile{1: tuile}

	deja := []*Batiment{
		CreerBatiment(g, Brut{"x": 0.0, "z": 0.0}, tuile),
		CreerBatiment(g, Brut{"x": 1.0, "z": 0.0}, tuile),
	}
	terrain := terrainVide(g, deja, 1000)

	devis := Estimer(terrain, cat, 1, 1, nil, nil, nil)
	if devis.Offert {
		t.Fatal("la troisieme n'est plus offerte")
	}
	if len(devis.Lignes) != 1 || devis.Lignes[0].Quantite != 150 {
		t.Fatalf("le devis doit reprendre les 150 bois, recu %v", devis.Lignes)
	}

	res := Poser(terrain, cat, 3, 3, 1, 1000, OptionsGeste{})
	if res.Ok {
		t.Fatal("sans bois, la troisieme doit etre refusee")
	}
	if !contientTexte(res.Refus, "150") {
		t.Fatalf("le refus doit chiffrer le manque, recu %q", res.Refus)
	}
}

func contientTexte(s, motif string) bool {
	for i := 0; i+len(motif) <= len(s); i++ {
		if s[i:i+len(motif)] == motif {
			return true
		}
	}
	return false
}
