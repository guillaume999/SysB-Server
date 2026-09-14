package routes

import (
	"strings"
	"testing"
)

// ============================================================================
//  partage_test.go — LE FILET DE LA REGLE DU PARTAGE, 2026-09-14.
//
//  ⚠️ L'ESSAI QUI COMPTE est celui de la PLANETE DE JOUEUR : une regle cassee
//  qui rendrait toujours `true` serait VERTE sur une planete game, puisque la
//  troisieme branche l'autorise de toute facon.
// ============================================================================

// ⚠️⚠️ LE COEUR DU SUJET. Si cet essai passe au rouge en rendant `true`, c'est
// que quelqu'un a trouve plus simple d'ecrire « pas de liste = ouvert » — et
// tous les modeles 3D du jeu apparaitraient d'un coup chez tous les joueurs.
func TestUnPartageableNeufNestOuvertAAucunePlaneteDeJoueur(t *testing.T) {
	neuf := Partageable{}
	if AutoriseeSur(neuf, "pAragonia", false) {
		t.Fatal("une liste vide doit vouloir dire « a personne »")
	}
}

// ⚠️ …mais l'existant continue de servir sur les planetes game, SANS rien
// cocher. C'est ce qui rend le patch indolore.
func TestUnPartageableNeufSertQuandMemeSurUnePlaneteGame(t *testing.T) {
	if !AutoriseeSur(Partageable{}, "pTerre", true) {
		t.Fatal("une planete game doit passer par la troisieme branche")
	}
}

func TestOuvertureNommeeEtOuvertureATous(t *testing.T) {
	nomme := Partageable{PlanetesAutorisees: []string{"pAragonia"}}
	if !AutoriseeSur(nomme, "pAragonia", false) {
		t.Fatal("la planete nommee doit passer")
	}
	if AutoriseeSur(nomme, "pSeb", false) {
		t.Fatal("une autre planete de joueur ne doit pas passer")
	}

	toutes := Partageable{ToutesPlanetes: true}
	for _, p := range []string{"pAragonia", "pSeb", "pPasEncoreNee"} {
		if !AutoriseeSur(toutes, p, false) {
			t.Fatalf("« a toutes » doit couvrir %s", p)
		}
	}
}

// ⚠️ SANS PLANETE, PAS D'AUTORISATION — meme « a toutes ». Laisser passer
// ouvrirait tout a une tuile mal rattachee.
func TestSansPlaneteRienNestAutorise(t *testing.T) {
	if AutoriseeSur(Partageable{ToutesPlanetes: true}, "", false) {
		t.Fatal("une tuile sans planete ne doit rien pouvoir citer")
	}
}

// ⚠️ LE REFUS NOMME LAQUELLE DES DEUX LISTES A DIT NON : une tuile a besoin
// d'un modele 3D ET d'une icone, et rien ne garantit que les deux soient
// ouvertes ensemble.
func TestLeRefusNommeLaListeFautive(t *testing.T) {
	ouvert := Partageable{PlanetesAutorisees: []string{"pA"}}
	ferme := Partageable{}

	if r := RefusDeTuile(ouvert, ouvert, true, true, "pA", "Aragonia", false); r != "" {
		t.Fatalf("les deux sont ouverts, rien ne doit etre refuse : %q", r)
	}
	if r := RefusDeTuile(ferme, ouvert, true, true, "pA", "Aragonia", false); !strings.Contains(r, "modele 3D") {
		t.Fatalf("le refus doit nommer le modele 3D : %q", r)
	}
	if r := RefusDeTuile(ouvert, ferme, true, true, "pA", "Aragonia", false); !strings.Contains(r, "icone") {
		t.Fatalf("le refus doit nommer l'icone : %q", r)
	}
	if r := RefusDeTuile(ferme, ferme, true, true, "", "", false); !strings.Contains(r, "aucune planete") {
		t.Fatalf("sans planete, le refus doit le dire : %q", r)
	}
}

// ⚠️ CE QUI N'EST PAS CITE N'EST PAS JUGE : une tuile sans icone n'est pas
// refusee POUR SON ICONE. Le catalogue a ses propres refus pour ca.
func TestUneTuileSansIconeNestPasRefuseePourSonIcone(t *testing.T) {
	if r := RefusDeTuile(Partageable{ToutesPlanetes: true}, Partageable{}, true, false,
		"pA", "Aragonia", false); r != "" {
		t.Fatalf("%q", r)
	}
}
