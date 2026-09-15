package routes

import (
	"strings"
	"testing"

	"sysb/moteur"
)

// ============================================================================
//  catalogue_planete_test.go — CE QUI JOUE SUR QUELLE PLANETE (15/09).
//
//  ⚠️ L'ESSAI QUI COMPTE est celui du JOUEUR : un filtre casse qui rendrait tout
//  serait vert sur la Terre, qui voit deja presque tout.
// ============================================================================

var planetesDuJeu = []moteur.Enregistrement{
	record{"id": "pGame", "nom": "Game", "proprietaire": ""},
	record{"id": "pTerre", "nom": "Terre", "proprietaire": ""},
	record{"id": "pJupiter", "nom": "Jupiter", "proprietaire": ""},
	record{"id": "pSeb", "nom": "Seb", "proprietaire": "u1"},
	record{"id": "pZoe", "nom": "Zoe", "proprietaire": "u2"},
}

func gardees(g map[string]bool) []string {
	var out []string
	for _, id := range []string{"", "pGame", "pTerre", "pJupiter", "pSeb", "pZoe"} {
		if g[id] {
			out = append(out, id)
		}
	}
	return out
}

func TestChezUnJoueurSaPlaneteSeule(t *testing.T) {
	g, tout := PlanetesDuCatalogue(planetesDuJeu, "pSeb")
	if tout || !memes(gardees(g), "pSeb") {
		t.Fatalf("attendu [pSeb], recu %v (tout=%v)", gardees(g), tout)
	}
}

func TestSurLaTerreLaTerreEtGame(t *testing.T) {
	g, tout := PlanetesDuCatalogue(planetesDuJeu, "pTerre")
	// ⚠️ PAS Jupiter : le magasin filtre par monde (13/09).
	if tout || !memes(gardees(g), "", "pGame", "pTerre") {
		t.Fatalf("recu %v (tout=%v)", gardees(g), tout)
	}
}

// Un plateau sans planete lit tout le jeu de l'admin, jamais un joueur.
func TestPlateauSansPlaneteLitLeJeuPasLesJoueurs(t *testing.T) {
	for _, id := range []string{"", "inconnue"} {
		g, tout := PlanetesDuCatalogue(planetesDuJeu, id)
		if tout || !memes(gardees(g), "", "pGame", "pTerre", "pJupiter") {
			t.Fatalf("%q : recu %v (tout=%v)", id, gardees(g), tout)
		}
	}
}

func TestSansPlanetesToutJoue(t *testing.T) {
	if _, tout := PlanetesDuCatalogue(nil, "pSeb"); !tout {
		t.Fatal("une base d'avant les planetes doit tout jouer")
	}
}

// Deux planetes, deux « bois » : chacune garde le sien, et la tuile de Seb ne
// joue que chez Seb.
func TestCataloguesSepares(t *testing.T) {
	palier := js([]any{map[string]any{"niveau": 1, "cycle_minutes": 1,
		"production": []any{map[string]any{"ressource": "bois", "quantite": 1}}}})
	src := sourceFactice{
		"ressources": {
			recCat{"code": "bois", "genre": "stock", "planete": "pGame"},
			recCat{"code": "bois", "genre": "mobilise", "planete": "pSeb"},
		},
		"tuiles": {
			recCat{"tileId": 1.0, "nom": "Scierie", "actif": true, "niveaux": palier, "planete": "pTerre"},
			recCat{"tileId": 2.0, "nom": "Scierie de Jupiter", "actif": true, "niveaux": palier, "planete": "pJupiter"},
			recCat{"tileId": 300.0, "nom": "Cabane de Seb", "actif": true, "niveaux": palier, "planete": "pSeb"},
		},
	}
	lectures := 0
	cats := NouveauxCatalogues(compteur{src, &lectures}, func() ([]moteur.Enregistrement, error) {
		return planetesDuJeu, nil
	})
	seb := cats.De("pSeb")
	if seb.TuilePour(300, 1) == nil || seb.TuilePour(1, 1) != nil {
		t.Fatal("chez Seb : sa cabane seule")
	}
	if seb.GenresBrut["bois"] != "mobilise" {
		t.Errorf("chez Seb, « bois » est le sien : %q", seb.GenresBrut["bois"])
	}
	terre := cats.De("pTerre")
	if terre.TuilePour(1, 1) == nil || terre.TuilePour(2, 1) != nil || terre.TuilePour(300, 1) != nil {
		t.Fatal("sur la Terre : la scierie de la Terre seule")
	}
	if terre.GenresBrut["bois"] != "stock" {
		t.Errorf("sur la Terre, « bois » est celui de Game : %q", terre.GenresBrut["bois"])
	}
	// Le meme catalogue n'est construit qu'une fois, et les collections ne sont
	// lues qu'une fois pour tous.
	if cats.De("pSeb") != seb {
		t.Error("le catalogue de Seb doit etre garde")
	}
	cats.Global()
	cats.De("pZoe")
	if lectures != 3 {
		t.Errorf("attendu 3 lectures (ressources, tuiles, technologies), recu %d", lectures)
	}
	// ⚠️ Une planete neuve a un catalogue VIDE : ce n'est pas une base illisible,
	// le garde-fou regarde le GLOBAL.
	if !cats.De("pZoe").Illisible() || cats.Global().Illisible() {
		t.Error("vide chez Zoe, lisible au global")
	}
}

type compteur struct {
	src sourceFactice
	n   *int
}

func (c compteur) Tous(col string) []moteur.Enregistrement {
	*c.n++
	return c.src.Tous(col)
}

// Le geste refuse une tuile d'une autre planete — c'est le serveur qui tient la
// regle, pas le magasin.
func TestGesteRefuseUneTuileDUneAutrePlanete(t *testing.T) {
	// Temoin : sur le catalogue commun, la meme pose passe (sinon l'essai
	// ci-dessous serait vert pour une autre raison).
	temoin := Geste(neuf(), "u1", DemandeGeste{Plateau: "pl1", Action: "poser", X: 2, Z: 0, Tuile: 1})
	if g, _ := temoin.Corps["geste"].(map[string]any); g == nil || g["accepte"] != true {
		t.Fatalf("temoin : la pose devait passer, recu %v", temoin.Corps)
	}

	d := neuf()
	d.plateaux[0]["planete"] = "pSeb"
	vide := moteur.ChargerCatalogue(sourceFactice{
		"ressources": {recCat{"code": "ble", "genre": "stock"}},
	})
	d.catDe = map[string]*moteur.CatalogueCharge{"pSeb": vide}
	r := Geste(d, "u1", DemandeGeste{Plateau: "pl1", Action: "poser", X: 2, Z: 0, Tuile: 1})
	g, _ := r.Corps["geste"].(map[string]any)
	if r.Code != 200 || g == nil || g["accepte"] != false {
		t.Fatalf("attendu un refus (200, accepte=false), recu %d %v", r.Code, r.Corps)
	}
	if refus, _ := g["refus"].(string); !strings.Contains(refus, "pas dans le catalogue") {
		t.Errorf("le refus doit dire pourquoi : %q", refus)
	}
}
