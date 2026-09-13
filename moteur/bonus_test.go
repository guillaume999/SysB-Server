package moteur

import "testing"

// ============================================================
//  §5.5 — LA LIGNE BONUS : monter AU-DESSUS de 100 % (13/09)
//
//  Demande de Guillaume : « il consomme 10 nourriture, il produit 100 % de
//  satisfaction ; s'il peut consommer 5 gibier en plus, il produit +20 % ».
//
//  ⚠️ Trois proprietes se tiennent la main, et les separer les casse :
//    1. le bonus AJOUTE sans entrer dans la demande      -> on depasse 100
//    2. il ne BLOQUE jamais un cycle                     -> il reste optionnel
//    3. une production ORDINAIRE reste plafonnee a 100 % -> le surplus ne paie
//       que par une tranche ecrite au-dessus de 100
// ============================================================

func genresBonus() *Genres {
	return CreerGenres(map[string]string{
		"nourriture": "stock", "gibier": "stock", "planche": "stock",
		"population": "mobilise", "satisfaction": "indicateur"})
}

// habitation : 10 nourriture (ordinaire) + 5 gibier (bonus 20 %), 50 places.
func habitationBonus(t *testing.T, g *Genres) *Tuile {
	t.Helper()
	tu, err := ChargerTuile(g, Brut{"tid": 1.0, "nom": "cabane", "cycle_minutes": 1.0,
		"utilisation": []any{
			Brut{"ressource": "nourriture", "quantite": 10.0},
			Brut{"ressource": "gibier", "quantite": 5.0, "bonus": 20.0},
		},
		"stockage": Brut{"population": 50.0, "*": 1000.0}})
	if err != nil {
		t.Fatal(err)
	}
	return tu
}

func satisfactionDe(t *testing.T, g *Genres, tu *Tuile, stock Brut) int {
	t.Helper()
	Refiger(g, []*Tuile{tu})
	b := CreerBatiment(g, Brut{"x": 0.0, "z": 0.0, "stock": stock}, tu)
	p := CreerPlateau(0, []*Batiment{b}, nil, g, nil, nil)
	servi, demande := MesurerIndice(p, b, 0, nil)
	return PourCent(servi, demande)
}

func TestBonusMonteAuDessusDeCent(t *testing.T) {
	g := genresBonus()
	tu := habitationBonus(t, g)

	cas := []struct {
		nom                string
		nourriture, gibier float64
		attendu            int
	}{
		{"tout servi : 100 + 20", 10, 5, 120},
		{"sans gibier : le bonus ne retire RIEN", 10, 0, 100},
		{"gibier a 2/5 : 100 + 20 x 2/5", 10, 2, 108},
		{"nourriture a moitie, gibier plein", 5, 5, 70},
		{"rien du tout", 0, 0, 0},
		{"QUE le gibier : la base ne demande rien de servi", 0, 5, 20},
		// ⚠️ Le trop-plein de gibier ne compte pas deux fois : la ligne
		// demande 5, elle est servie 5, pas 40.
		{"gibier en exces : plafonne a sa propre demande", 10, 40, 120},
	}
	for _, c := range cas {
		got := satisfactionDe(t, g, tu, Brut{"nourriture": c.nourriture, "gibier": c.gibier})
		if got != c.attendu {
			t.Errorf("%s : satisfaction = %d %%, attendu %d %%", c.nom, got, c.attendu)
		}
	}
}

// ⚠️⚠️ LA PROPRIETE QUI EMPECHE LE BONUS DE DEVENIR OBLIGATOIRE. Sans le
// `continue` de `DeQuoiTourner`, une cabane sans gibier n'entamerait plus
// AUCUN cycle et la colonie mourrait de faim faute de viande de luxe.
func TestBonusNeBloqueJamaisLeCycle(t *testing.T) {
	g := genresBonus()
	tu, err := ChargerTuile(g, Brut{"tid": 1.0, "nom": "cabane", "cycle_minutes": 1.0,
		"utilisation": []any{
			Brut{"ressource": "nourriture", "quantite": 10.0},
			Brut{"ressource": "gibier", "quantite": 5.0, "bonus": 20.0},
		},
		"production": []any{Brut{"ressource": "planche", "quantite": 4.0}},
		"stockage":   Brut{"population": 50.0, "*": 1000.0}})
	if err != nil {
		t.Fatal(err)
	}
	Refiger(g, []*Tuile{tu})
	// De la nourriture, PAS un gramme de gibier.
	b := CreerBatiment(g, Brut{"x": 0.0, "z": 0.0, "stock": Brut{"nourriture": 100.0}}, tu)
	p := CreerPlateau(0, []*Batiment{b}, nil, g, nil, nil)

	if !DeQuoiTourner(p, b, 0) {
		t.Fatal("une ligne bonus manquante EMPECHE le cycle de demarrer — elle devient obligatoire")
	}
	if err := Avancer(p, 600); err != nil { // 10 cycles d'une minute
		t.Fatal(err)
	}
	if got := b.Stock.Get(g.Reg.Id("planche")); got != 40 {
		t.Errorf("planches = %d, attendu 40 — 10 cycles servis a 100 %% sur la base", got)
	}
	if got := b.Stock.Get(g.Reg.Id("nourriture")); got != 0 {
		t.Errorf("nourriture restante = %d, attendu 0", got)
	}
}

// ⚠️⚠️ 120 % NE FAIT PAS SUR-PRODUIRE TOUT LE CATALOGUE. Une ligne ordinaire
// livre sa quantite declaree, jamais plus — sinon le debit affiche sur la
// fiche cesserait d'etre un maximum.
func TestProductionOrdinairePlafonneeACentPourCent(t *testing.T) {
	g := genresBonus()
	tu, err := ChargerTuile(g, Brut{"tid": 1.0, "nom": "cabane", "cycle_minutes": 1.0,
		"utilisation": []any{
			Brut{"ressource": "nourriture", "quantite": 10.0},
			Brut{"ressource": "gibier", "quantite": 5.0, "bonus": 20.0},
		},
		"production": []any{Brut{"ressource": "planche", "quantite": 10.0}},
		"stockage":   Brut{"population": 50.0, "*": 10000.0}})
	if err != nil {
		t.Fatal(err)
	}
	Refiger(g, []*Tuile{tu})
	b := CreerBatiment(g, Brut{"x": 0.0, "z": 0.0,
		"stock": Brut{"nourriture": 10.0, "gibier": 5.0}}, tu)
	p := CreerPlateau(0, []*Batiment{b}, nil, g, nil, nil)

	if servi, demande := MesurerIndice(p, b, 0, nil); PourCent(servi, demande) != 120 {
		t.Fatalf("le decor du test est faux : satisfaction = %d %%, attendue 120 %%",
			PourCent(servi, demande))
	}
	if err := Avancer(p, 60); err != nil {
		t.Fatal(err)
	}
	if got := b.Stock.Get(g.Reg.Id("planche")); got != 10 {
		t.Errorf("planches = %d, attendu 10 — une ligne ORDINAIRE ne depasse pas sa quantite", got)
	}
}

// Et le pendant : c'est l'escalier, et lui seul, qui fait payer le surplus.
func TestEscalierPaieAuDessusDeCent(t *testing.T) {
	g := genresBonus()
	maison, err := ChargerTuile(g, Brut{"tid": 1.0, "nom": "cabane", "cycle_minutes": 1.0,
		"utilisation": []any{
			Brut{"ressource": "nourriture", "quantite": 10.0},
			Brut{"ressource": "gibier", "quantite": 5.0, "bonus": 20.0},
		},
		"stockage": Brut{"population": 50.0, "*": 1000.0}})
	if err != nil {
		t.Fatal(err)
	}
	// Une scierie qui SUIT la satisfaction, avec une marche au-dessus de 100.
	scierie, err := ChargerTuile(g, Brut{"tid": 2.0, "nom": "scierie", "cycle_minutes": 1.0,
		"production": []any{Brut{"ressource": "planche", "quantite": 10.0,
			"indicateur": "satisfaction",
			"tranches": []any{
				[]any{120.0, 130.0},
				[]any{80.0, 100.0},
				[]any{0.0, 60.0},
			}}},
		"stockage": Brut{"*": 10000.0}})
	if err != nil {
		t.Fatal(err)
	}
	Refiger(g, []*Tuile{maison, scierie})

	planches := func(gibier float64) int {
		m := CreerBatiment(g, Brut{"x": 0.0, "z": 0.0,
			"stock": Brut{"nourriture": 10.0, "gibier": gibier}}, maison)
		s := CreerBatiment(g, Brut{"x": 1.0, "z": 0.0}, scierie)
		p := CreerPlateau(0, []*Batiment{m, s}, nil, g, nil, nil)
		if err := Avancer(p, 60); err != nil {
			t.Fatal(err)
		}
		return s.Stock.Get(g.Reg.Id("planche"))
	}

	if got := planches(5); got != 13 {
		t.Errorf("maison a 120 %% : planches = %d, attendu 13 (10 x 130 %%)", got)
	}
	if got := planches(0); got != 10 {
		t.Errorf("maison a 100 %% : planches = %d, attendu 10 (10 x 100 %%)", got)
	}
}

// ⚠️ « Ne rien demander » vaut 100 %, PAS la tranche du haut. Tant que rien ne
// montait au-dessus de 100 les deux se confondaient ; depuis, la tranche du
// haut offrirait le bonus maximal a un batiment qui ne consomme rien.
func TestNeRienDemanderNeDonnePasLaTrancheBonus(t *testing.T) {
	l := &Ligne{Tranches: [][2]int{{120, 130}, {80, 100}, {0, 60}}}
	if got := Tranche(l, 0, 0); got != 100 {
		t.Errorf("tranche d'un batiment qui ne demande rien = %d, attendu 100", got)
	}
	if got := Tranche(l, 6, 5); got != 130 {
		t.Errorf("tranche a 120 %% = %d, attendu 130", got)
	}
	if got := Tranche(l, 79, 100); got != 60 {
		t.Errorf("tranche a 79 %% = %d, attendu 60 (la falaise)", got)
	}
}

// Le catalogue refuse ce qui n'a pas de sens, bruyamment.
func TestBonusRefuseSurUneProduction(t *testing.T) {
	g := genresBonus()
	_, err := ChargerTuile(g, Brut{"tid": 1.0, "nom": "scierie", "cycle_minutes": 1.0,
		"production": []any{Brut{"ressource": "planche", "quantite": 10.0, "bonus": 20.0}},
		"stockage":   Brut{"*": 100.0}})
	if err == nil {
		t.Fatal("un bonus sur une PRODUCTION doit etre refuse — la satisfaction ne se fabrique pas")
	}
}

func TestBonusDecimalRefuse(t *testing.T) {
	g := genresBonus()
	_, err := ChargerTuile(g, Brut{"tid": 1.0, "nom": "cabane", "cycle_minutes": 1.0,
		"utilisation": []any{Brut{"ressource": "gibier", "quantite": 5.0, "bonus": 12.5}},
		"stockage":    Brut{"*": 100.0}})
	if err == nil {
		t.Fatal("un bonus decimal doit etre refuse, jamais arrondi en silence")
	}
}

// ⚠️ La non-regression qui compte : SANS bonus, le rapport est EXACTEMENT
// celui d'avant le 13/09.
func TestSansBonusLeRapportNeBougePas(t *testing.T) {
	acc := CreerRapport()
	acc.Ajouter(15, 20, 20)
	acc.Ajouter(10, 10, 10)
	if got := PourCent(acc.Servi(), acc.Demande()); got != 83 {
		t.Errorf("satisfaction = %d %%, attendu 83 %% (25 servis sur 30)", got)
	}
}
