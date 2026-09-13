package moteur

import (
	"strings"
	"testing"
)

// ⚠️ LE TOUR COMPLET, celui que fait une route : catalogue -> record ->
// rattrapage -> garde-fous -> ecriture -> bloc pour Unity. C'est le seul test
// qui prouve que les morceaux se parlent ; les autres verifient chacun le sien.
func TestLeTourCompletDUneRoute(t *testing.T) {
	cat := ChargerCatalogue(sourcePourTest())
	if cat.Illisible() {
		t.Fatal("catalogue illisible")
	}

	// Un plateau 4x1 : une ferme (tid 1) en (0,0), une maison (tid 2) en (1,0),
	// et deux cases d'herbe.
	tiles := OctetsVersBase64([]int{1, 2, 0, 0})
	rec := recordCarte{
		"id": "pl1", "nom": "Ma colonie", "typeOfPlateau": "colonie",
		"largeur": 4.0, "hauteur": 1.0, "version": 3.0,
		"tilesBase64": tiles,
		"t":           1000.0,
		"etats":       `[{"x":0,"z":0,"stock":{"ble":40}},{"x":1,"z":0}]`,
		"reserve":     `{"or":250}`,
		"technos":     `{"irrigation":1}`,
	}

	partie := ChargerPartie(rec, cat, 1000)
	if partie.Id != "pl1" || partie.Largeur != 4 {
		t.Fatalf("record mal lu : %+v", partie.Id)
	}
	if len(partie.Plateau.Batiments) != 2 {
		t.Fatalf("2 batiments attendus, %d montes", len(partie.Plateau.Batiments))
	}
	or := cat.genres.Reg.Id("or")
	if partie.Plateau.Reserve.Get(or) != 250 {
		t.Errorf("reserve mal lue : %d", partie.Plateau.Reserve.Get(or))
	}
	// ⚠️ Le MONDE vient du tableau d'octets, pas des etats : c'est le piege du
	// 31/08. On le verifie en comptant une case d'herbe, qui n'a aucun etat.
	if partie.Plateau.monde().Tid(2, 0) != 0 || partie.Plateau.monde().Tid(1, 0) != 2 {
		t.Error("le monde doit venir de tilesBase64")
	}

	// Une heure de jeu.
	pr := partie.Avancer(1000+3600, 0)
	if !pr.Fini || pr.T != 4600 {
		t.Fatalf("rattrapage : %+v", pr)
	}

	if err := partie.Verifier(-1); err != nil {
		t.Fatalf("les garde-fous refusent un etat pourtant sain : %v", err)
	}
	if len(partie.Changements()) == 0 {
		t.Error("une heure de cycles doit faire bouger quelque chose")
	}

	partie.Ecrire(rec)
	if Entier(rec.Get("t"), 0) != 4600 {
		t.Errorf("le `t` ecrit vaut %v, attendu 4600", rec.Get("t"))
	}

	// Le bloc que lit Unity.
	bloc := FaireBloc(rec, partie, TechnosDe(rec), Indicateurs(partie.Plateau, cat.GenresBrut, partie.Plateau.T))
	if bloc.Plateau.T != 4600 || bloc.Plateau.Version != 3 {
		t.Errorf("entete du bloc : t=%d version=%d", bloc.Plateau.T, bloc.Plateau.Version)
	}
	if bloc.Plateau.TilesBase64 != tiles {
		t.Error("⚠️ le rattrapage ne doit JAMAIS toucher au terrain")
	}
	if len(bloc.Cases) != 2 {
		t.Fatalf("le bloc doit decrire les 2 cases, il en a %d", len(bloc.Cases))
	}
	for _, c := range bloc.Cases {
		// ⚠️ PLUS DE BORNE HAUTE (13/09) : une ligne bonus fait legitimement
		// monter au-dessus de 100 (§5.5). Seul le plancher tient encore.
		if c.Satisfaction < 0 {
			t.Errorf("satisfaction negative (%d) — elle est EN POUR CENT", c.Satisfaction)
		}
	}
	if bloc.TechnosAcquises["irrigation"] != 1 {
		t.Errorf("technos du joueur mal lues : %v", bloc.TechnosAcquises)
	}
	if _, y := bloc.IndicateursPourCent["satisfaction"]; !y {
		t.Error("l'indicateur declare doit etre publie")
	}
	// La ferme ENVOIE : son contenu est depensable. La maison ne fait que
	// recolter : le sien ne l'est pas.
	if bloc.Ressources.Depensable["ble"] <= 0 {
		t.Errorf("le ble de la ferme (qui envoie) doit etre depensable : %v", bloc.Ressources.Depensable)
	}
	if bloc.Ressources.Mobilise["habitant"] != 0 {
		t.Errorf("aucun cout `mobilise` dans ce catalogue : %v", bloc.Ressources.Mobilise)
	}
}

// ⚠️ Le rattrapage par TRANCHES doit marcher au niveau de la partie aussi, et
// rendre le meme etat qu'en un coup — sinon la reponse au §11.1 s'arrete au
// moteur et ne descend pas jusqu'a la route.
func TestUnePartieSeRattrapeParTranches(t *testing.T) {
	monter := func() *Partie {
		cat := ChargerCatalogue(sourcePourTest())
		rec := recordCarte{"id": "pl1", "largeur": 4.0, "hauteur": 1.0,
			"tilesBase64": OctetsVersBase64([]int{1, 2, 0, 0}), "t": 1000.0,
			"etats": `[{"x":0,"z":0,"stock":{"ble":40}},{"x":1,"z":0}]`}
		return ChargerPartie(rec, cat, 1000)
	}

	dun := monter()
	dun.Avancer(1000+7200, 0)
	temoin := Photo(dun.Plateau)

	parTranches := monter()
	appels := 0
	for {
		pr := parTranches.Avancer(1000+7200, 5) // 5 ruptures a la fois
		appels++
		if pr.Fini {
			break
		}
		if appels > 100000 {
			t.Fatal("ca ne progresse pas")
		}
	}
	if got := Photo(parTranches.Plateau); got != temoin {
		t.Errorf("l'etat differe apres %d tranches", appels)
	} else {
		t.Logf("%d tranches, etat identique", appels)
	}
}

// Les garde-fous refusent d'ecrire un etat qui ne peut pas etre vrai.
func TestLesGardeFousRefusentUnEtatImpossible(t *testing.T) {
	cat := ChargerCatalogue(sourcePourTest())
	rec := recordCarte{"id": "pl1", "largeur": 4.0, "hauteur": 1.0,
		"tilesBase64": OctetsVersBase64([]int{1, 2, 0, 0}), "t": 1000.0,
		"etats": `[{"x":0,"z":0},{"x":1,"z":0}]`}
	partie := ChargerPartie(rec, cat, 1000)

	// Un stock negatif ne peut pas etre vrai.
	partie.Plateau.Batiments[0].Stock.Set(cat.genres.Reg.Id("ble"), -1)
	if err := partie.Verifier(-1); err == nil || !strings.Contains(err.Error(), "negatif") {
		t.Errorf("un stock negatif doit etre refuse, recu : %v", err)
	}
	partie.Plateau.Batiments[0].Stock.Set(cat.genres.Reg.Id("ble"), 0)

	// Le temps ne recule pas.
	partie.Plateau.T = 500
	if err := partie.Verifier(-1); err == nil || !strings.Contains(err.Error(), "recule") {
		t.Errorf("un temps qui recule doit etre refuse, recu : %v", err)
	}
	partie.Plateau.T = 1000

	// Une case qui disparait sans geste non plus.
	partie.Plateau.Batiments = partie.Plateau.Batiments[:1]
	partie.Plateau.ViderCaches()
	if err := partie.Verifier(-1); err == nil || !strings.Contains(err.Error(), "cases") {
		t.Errorf("une case perdue doit etre refusee, recu : %v", err)
	}
	// ...mais APRES un geste, elle est attendue : c'est le parametre.
	if err := partie.Verifier(1); err != nil {
		t.Errorf("apres un geste, 1 case attendue doit passer : %v", err)
	}
}
