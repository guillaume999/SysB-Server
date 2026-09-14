package routes

import (
	"strings"
	"testing"

	"sysb/moteur"
)

// ============================================================================
//  maplanete_test.go — LA CREATION DE LA PLANETE D'UN JOUEUR, 2026-09-14.
//
//  ⚠️ CE QUI EST TENU ICI, et qu'aucun type ne dit : « une seule planete par
//  joueur » (l'index ne porte que le nom), et le fait que les DEUX modeles
//  partent AVEC la planete — sans eux, le joueur a une planete qu'il ne peut
//  ni dessiner ni ouvrir.
// ============================================================================

func vide() *depot {
	d := neuf()
	d.planetes = []record{{"id": "pTerre", "nom": "Terre", "proprietaire": ""}}
	return d
}

func TestMaPlaneteCreeLaPlaneteEtSesDeuxModeles(t *testing.T) {
	d := vide()
	r := MaPlanete(d, "u1", "  Aragonia  ", "m3d1", "ico1")
	if r.Code != 200 || r.Corps["cree"] != true {
		t.Fatalf("%d %v", r.Code, r.Corps["verdict"])
	}

	if len(d.planetesCreees) != 1 {
		t.Fatalf("%d planete(s) creee(s)", len(d.planetesCreees))
	}
	p := d.planetesCreees[0]
	// ⚠️ LE NOM EST TRIME : « Aragonia » et «  Aragonia  » sont la meme planete,
	// et la comparaison stricte d'ailleurs ne le saurait pas.
	if moteur.Texte(p["nom"]) != "Aragonia" {
		t.Errorf("nom = %q", p["nom"])
	}
	if moteur.Texte(p["proprietaire"]) != "u1" {
		t.Errorf("⚠️ sans proprietaire, ce serait une planete GAME : %q", p["proprietaire"])
	}

	// ⚠️ LES DEUX SURFACES, parce qu'EarthScene ouvre les deux.
	if len(d.templatesCrees) != 2 {
		t.Fatalf("%d modele(s)", len(d.templatesCrees))
	}
	vus := map[string]bool{}
	for _, tpl := range d.templatesCrees {
		vus[moteur.Texte(tpl["typeOfPlateau"])] = true
		if moteur.Texte(tpl["planete"]) != "pl-planete-neuve" {
			t.Errorf("modele non rattache a la planete : %v", tpl["planete"])
		}
		// ⚠️ SANS `partages`, IL NE PEUT PAS LA DESSINER : c'est ce champ que
		// lit la regle d'API d'`update` sur `templates`.
		parts, _ := tpl["partages"].([]string)
		if len(parts) != 1 || parts[0] != "u1" {
			t.Errorf("⚠️ le joueur n'a pas le crayon sur son propre modele : %v", tpl["partages"])
		}
		// ⚠️ UNE GRILLE DE LA BONNE TAILLE, sinon `assurer` refuse en 422.
		n := len(moteur.Base64VersOctets(moteur.Texte(tpl["tilesBase64"])))
		if n != LargeurDeDepart*HauteurDeDepart {
			t.Errorf("grille de %d octets pour %d cases", n, LargeurDeDepart*HauteurDeDepart)
		}
	}
	if !vus["ground"] || !vus["space"] {
		t.Errorf("il manque une surface : %v", vus)
	}

	// ⚠️ LA PHRASE QUI EVITE L'ECRAN MUET.
	if v := moteur.Texte(r.Corps["verdict"]); !strings.Contains(v, "aucun pinceau") {
		t.Errorf("le verdict doit prevenir que l'editeur sera vide : %q", v)
	}
}

// ⚠️⚠️ L'ESSAI QUI TIENT LA REGLE. L'index unique de `planetes` ne porte que le
// NOM : rien en base n'empeche un joueur d'en avoir deux. C'est ici, et nulle
// part ailleurs.
func TestUneSeulePlanetePersoParJoueur(t *testing.T) {
	d := vide()
	d.planetes = append(d.planetes, record{"id": "pA", "nom": "Aragonia", "proprietaire": "u1"})
	r := MaPlanete(d, "u1", "Deuxieme", "", "")
	if r.Code != 409 {
		t.Fatalf("code %d", r.Code)
	}
	if v := moteur.Texte(r.Corps["verdict"]); !strings.Contains(v, "Aragonia") {
		t.Errorf("le refus doit NOMMER la planete qu'il a deja : %q", v)
	}
	if len(d.planetesCreees) != 0 {
		t.Error("rien ne doit etre ecrit")
	}
}

// Un autre joueur, lui, a le droit d'en creer une.
func TestLaPlaneteDunAutreNeBloquePas(t *testing.T) {
	d := vide()
	d.planetes = append(d.planetes, record{"id": "pA", "nom": "Aragonia", "proprietaire": "u2"})
	if r := MaPlanete(d, "u1", "Sebtopia", "", ""); r.Code != 200 {
		t.Fatalf("%d %v", r.Code, r.Corps["verdict"])
	}
}

// ⚠️ LE NOM EST LA CLE DE LA RECHERCHE : deux « Terre » rendraient le panneau
// menteur. La base refuserait aussi, mais avec un message de base de donnees.
func TestUnNomDejaPrisEstRefuseAvecUnePhraseLisible(t *testing.T) {
	d := vide()
	r := MaPlanete(d, "u1", "Terre", "", "")
	if r.Code != 409 || !strings.Contains(moteur.Texte(r.Corps["verdict"]), "autre nom") {
		t.Fatalf("%d %v", r.Code, r.Corps["verdict"])
	}
}

func TestUnNomVideEstRefuse(t *testing.T) {
	d := vide()
	if r := MaPlanete(d, "u1", "   ", "", ""); r.Code != 400 {
		t.Fatalf("code %d", r.Code)
	}
	if len(d.planetesCreees) != 0 {
		t.Error("rien ne doit etre ecrit")
	}
}

// ⚠️ L'APPARENCE EST FACULTATIVE : une planete sans sphere ni vignette se joue,
// elle est juste terne. La refuser bloquerait la creation le jour ou l'admin
// n'a encore ouvert aucune apparence.
func TestLApparenceEstFacultative(t *testing.T) {
	d := vide()
	if r := MaPlanete(d, "u1", "Nue", "", ""); r.Code != 200 {
		t.Fatalf("%d %v", r.Code, r.Corps["verdict"])
	}
}
