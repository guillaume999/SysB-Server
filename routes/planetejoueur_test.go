package routes

import (
	"strings"
	"testing"

	"sysb/moteur"
)

// ============================================================================
//  planetejoueur_test.go — LA PLANETE CREEE D'OFFICE, 2026-09-15.
//
//  ⚠️ CE QUI EST TENU ICI, et qu'aucun type ne dit : une seule planete par
//  joueur (l'index ne porte que le nom), les DEUX modeles qui partent avec
//  elle, `appartient` = le joueur, et l'idempotence — le rattrapage du
//  demarrage repasse sur TOUS les comptes a chaque redemarrage.
// ============================================================================

func avecJoueur(pseudo string) *depot {
	d := neuf()
	d.planetes = []record{
		{"id": "pTerre", "nom": "Terre", "proprietaire": ""},
		{"id": "pGame", "nom": "Game", "proprietaire": ""},
	}
	d.users["u1"] = record{"id": "u1", "pseudo": pseudo, "email": "samp@exemple.fr"}
	return d
}

func TestLaPlaneteDuJoueurPorteSonPseudoEtSesDeuxModeles(t *testing.T) {
	d := avecJoueur("  Samp  ")
	r := AssurerPlaneteDe(d, "u1")
	if r.Code != 200 || r.Corps["cree"] != true {
		t.Fatalf("%d %v", r.Code, r.Corps["verdict"])
	}
	if len(d.planetesCreees) != 1 {
		t.Fatalf("%d planete(s) creee(s)", len(d.planetesCreees))
	}
	p := d.planetesCreees[0]
	if moteur.Texte(p["nom"]) != "Samp" {
		t.Errorf("nom = %q, attendu le pseudo", p["nom"])
	}
	if moteur.Texte(p["proprietaire"]) != "u1" {
		t.Errorf("⚠️ sans proprietaire, ce serait une planete GAME : %q", p["proprietaire"])
	}

	if len(d.templatesCrees) != 2 {
		t.Fatalf("%d modele(s)", len(d.templatesCrees))
	}
	vus := map[string]bool{}
	for _, tpl := range d.templatesCrees {
		vus[moteur.Texte(tpl["typeOfPlateau"])] = true
		if moteur.Texte(tpl["planete"]) != "pl-planete-neuve" {
			t.Errorf("modele non rattache a la planete : %v", tpl["planete"])
		}
		if moteur.Texte(tpl["appartient"]) != "u1" {
			t.Errorf("⚠️ appartient = %q, attendu le joueur", tpl["appartient"])
		}
		if _, pose := tpl["partages"]; pose {
			t.Errorf("⚠️ `partages` est retire (15/09), il ne doit plus etre ecrit : %v", tpl["partages"])
		}
		n := len(moteur.Base64VersOctets(moteur.Texte(tpl["tilesBase64"])))
		if n != LargeurDeDepart*HauteurDeDepart {
			t.Errorf("grille de %d octets pour %d cases", n, LargeurDeDepart*HauteurDeDepart)
		}
	}
	if !vus["ground"] || !vus["space"] {
		t.Errorf("il manque une surface : %v", vus)
	}
}

// ⚠️⚠️ L'IDEMPOTENCE : le rattrapage repasse a chaque demarrage.
func TestUnSecondAppelNEcritRien(t *testing.T) {
	d := avecJoueur("Samp")
	AssurerPlaneteDe(d, "u1")
	r := AssurerPlaneteDe(d, "u1")
	if r.Code != 200 || r.Corps["ecrit"] != false {
		t.Fatalf("%d %v", r.Code, r.Corps)
	}
	if len(d.planetesCreees) != 1 || len(d.templatesCrees) != 2 {
		t.Fatalf("doublon : %d planete(s), %d modele(s)", len(d.planetesCreees), len(d.templatesCrees))
	}
}

// Une planete qui a perdu un modele le retrouve, sans en recreer une.
func TestUnModeleManquantEstAjoute(t *testing.T) {
	d := avecJoueur("Samp")
	d.planetes = append(d.planetes, record{"id": "pA", "nom": "Samp", "proprietaire": "u1"})
	d.templates = []record{{"id": "tg", "typeOfPlateau": "ground", "planete": "pA"}}
	r := AssurerPlaneteDe(d, "u1")
	if r.Code != 200 || r.Corps["cree"] != false {
		t.Fatalf("%d %v", r.Code, r.Corps)
	}
	if len(d.planetesCreees) != 0 {
		t.Error("⚠️ une seule planete par joueur")
	}
	if len(d.templatesCrees) != 1 || moteur.Texte(d.templatesCrees[0]["typeOfPlateau"]) != "space" {
		t.Fatalf("attendu le seul `space` : %v", d.templatesCrees)
	}
}

// ⚠️ UN NOM PRIS NE BLOQUE PAS : on numerote, casse ignoree.
func TestUnNomPrisEstNumerote(t *testing.T) {
	d := avecJoueur("terre")
	if r := AssurerPlaneteDe(d, "u1"); r.Code != 200 {
		t.Fatalf("%d %v", r.Code, r.Corps["verdict"])
	}
	if n := moteur.Texte(d.planetesCreees[0]["nom"]); n != "terre (2)" {
		t.Errorf("nom = %q", n)
	}
}

func TestLesReplisDuNom(t *testing.T) {
	if n := NomDePlaneteDe("", "bob@x.fr", "u9", nil); n != "bob" {
		t.Errorf("repli email : %q", n)
	}
	if n := NomDePlaneteDe(" ", "", "u9", nil); n != "Planete u9" {
		t.Errorf("repli id : %q", n)
	}
	// ⚠️ « game » n'est jamais un nom de planete de joueur.
	if n := NomDePlaneteDe("GAME", "", "u9", nil); n != "GAME (2)" {
		t.Errorf("reserve : %q", n)
	}
	if n := NomDePlaneteDe("A", "", "u9", []string{"a", "A (2)"}); n != "A (3)" {
		t.Errorf("numerotation : %q", n)
	}
}

func TestPseudoReserve(t *testing.T) {
	for _, p := range []string{"game", "Game", " GAME "} {
		if !PseudoReserve(p) {
			t.Errorf("%q devrait etre refuse", p)
		}
	}
	for _, p := range []string{"", "gamer", "Samp", "le game"} {
		if PseudoReserve(p) {
			t.Errorf("%q ne devrait pas etre refuse", p)
		}
	}
}

// ⚠️ LE PIEGE DU CHAMP ABSENT : sans `appartient` en base, on ne cree RIEN.
func TestSansChampAppartientRienNEstEcrit(t *testing.T) {
	d := avecJoueur("Samp")
	d.champsTemplates = []string{"id", "nom", "typeOfPlateau", "planete"}
	r := AssurerPlaneteDe(d, "u1")
	if r.Code != 500 || !strings.Contains(moteur.Texte(r.Corps["verdict"]), "patch-appartient") {
		t.Fatalf("%d %v", r.Code, r.Corps["verdict"])
	}
	if len(d.planetesCreees) != 0 || len(d.templatesCrees) != 0 {
		t.Error("rien ne doit etre ecrit")
	}
}

func TestCompteInconnu(t *testing.T) {
	d := avecJoueur("Samp")
	if r := AssurerPlaneteDe(d, "inconnu"); r.Code != 404 {
		t.Fatalf("code %d", r.Code)
	}
	if r := AssurerPlaneteDe(d, ""); r.Code != 400 {
		t.Fatalf("code %d", r.Code)
	}
}
