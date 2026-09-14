package routes

import (
	"testing"

	"sysb/moteur"
)

// ============================================================================
//  territoire_test.go — CE QUI COMPTE DANS UNE LIMITE « EMPIRE », 2026-09-14.
//
//  ⚠️ L'ESSAI QUI COMPTE est celui de la PLANETE PERSO. Un filtre casse qui
//  rendrait toujours tout serait VERT sur l'empire, puisque l'empire garde deja
//  toutes les planetes game.
// ============================================================================

func plateau(id, planete string) moteur.Enregistrement {
	return record{"id": id, "planete": planete}
}

func ids(l []moteur.Enregistrement) []string {
	out := []string{}
	for _, r := range l {
		out = append(out, moteur.Texte(moteur.Champ(r, "id")))
	}
	return out
}

func memes(a []string, b ...string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Terre et Jupiter sont game (proprietaire vide) ; « chezMoi » est a u1.
var proprietaires = map[string]string{"pTerre": "", "pJupiter": "", "pChezMoi": "u1"}

var tousMesPlateaux = []moteur.Enregistrement{
	plateau("terre-sol", "pTerre"),
	plateau("terre-orbite", "pTerre"),
	plateau("jupiter-sol", "pJupiter"),
	plateau("chezmoi-sol", "pChezMoi"),
	plateau("chezmoi-orbite", "pChezMoi"),
}

// ⚠️ LA REGLE DU 13/09 TIENT : l'empire REGROUPE les planetes game. « max 1
// dans l'empire » vaut pour la Terre ET Jupiter ensemble.
func TestEmpireRegroupeLesPlanetesGame(t *testing.T) {
	vus := ids(plateauxDuTerritoire(tousMesPlateaux, "terre-sol", proprietaires, true))
	if !memes(vus, "terre-sol", "terre-orbite", "jupiter-sol") {
		t.Fatalf("%v", vus)
	}
}

// ⚠️⚠️ ET LA PLANETE PERSO EN SORT (14/09). Ce qu'un joueur batit chez lui ne
// compte pas dans son empire.
func TestLaPlanetePersoNeComptePasDansLEmpire(t *testing.T) {
	vus := ids(plateauxDuTerritoire(tousMesPlateaux, "terre-sol", proprietaires, true))
	for _, id := range vus {
		if id == "chezmoi-sol" || id == "chezmoi-orbite" {
			t.Fatalf("un plateau perso compte dans l'empire : %v", vus)
		}
	}
}

// ⚠️ ET RECIPROQUEMENT : chez lui, ses colonies de la Terre ne comptent pas.
func TestChezLuiSeulsSesDeuxPlateauxComptent(t *testing.T) {
	vus := ids(plateauxDuTerritoire(tousMesPlateaux, "chezmoi-sol", proprietaires, true))
	if !memes(vus, "chezmoi-sol", "chezmoi-orbite") {
		t.Fatalf("%v", vus)
	}
}

// ⚠️ SUR UNE BASE D'AVANT LE PATCH, rien ne change : tout compte, comme avant.
// Une nouveaute de schema ne doit pas changer une regle de jeu en silence.
func TestSansPlanetesConnuesToutCompteCommeAvant(t *testing.T) {
	vus := ids(plateauxDuTerritoire(tousMesPlateaux, "terre-sol", map[string]string{}, false))
	if len(vus) != len(tousMesPlateaux) {
		t.Fatalf("%v", vus)
	}
}

// ⚠️ Un plateau sans planete (ligne d'avant le patch, restee vide) reste dans
// l'empire : « pas de planete » n'est pas « chez quelqu'un ».
func TestUnPlateauSansPlaneteResteDansLEmpire(t *testing.T) {
	avecOrphelin := append([]moteur.Enregistrement{plateau("orphelin", "")}, tousMesPlateaux...)
	vus := ids(plateauxDuTerritoire(avecOrphelin, "terre-sol", proprietaires, true))
	if !memes(vus, "orphelin", "terre-sol", "terre-orbite", "jupiter-sol") {
		t.Fatalf("%v", vus)
	}
}
