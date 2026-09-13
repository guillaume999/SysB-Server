package routes

import (
	"strings"
	"testing"

	"sysb/moteur"
)

// ─── Le depot factice, augmente de ce qu'`Assurer` demande ──────────────────

// ⚠️ LE MODELE EST UN RECORD COMME UN AUTRE, avec ses champs json en `[]byte` :
// c'est ce que l'adaptateur livre. Un faux qui rendrait des `map[string]any`
// deja decodes prouverait quelque chose que la vraie base ne fait pas — la
// faute exacte du 12/09.
func avecModele(d *depot) *depot {
	d.templates = []record{{
		"id": "tpl1", "typeOfPlateau": "ground", "largeur": 4.0, "hauteur": 1.0,
		"tilesBase64": moteur.OctetsVersBase64([]int{1, 2, 0, 0}),
		"etats": js([]any{
			map[string]any{"x": 0, "z": 0, "stock": map[string]any{"ble": 40},
				// ⚠️ Le modele porte un chantier et un vieux temps : ni l'un ni
				// l'autre ne doit se leguer.
				"chantier": 999, "t": 1},
			map[string]any{"x": 1, "z": 0, "niveau": 1},
			// Hors plateau : elle ne se recopie pas.
			map[string]any{"x": 9, "z": 0},
		}),
		"amorcage": js(map[string]any{"ressources_depart": []any{
			map[string]any{"ressource": "or", "quantite": 250},
			map[string]any{"ressource": "ble", "quantite": 100},
		}}),
	}}
	return d
}

func vierge() *depot {
	d := neuf()
	d.plateaux = nil // un compte NEUF : c'est tout le sujet de cette route
	return avecModele(d)
}

func creer(t *testing.T, d *depot) (Reponse, record) {
	t.Helper()
	r := Assurer(d, "u1", "ground")
	if r.Code != 200 || r.Corps["cree"] != true {
		t.Fatalf("assurer : %d %v", r.Code, r.Corps["verdict"])
	}
	if d.dernier == nil {
		t.Fatal("aucun record cree")
	}
	return r, d.dernier
}

// ─── Ce qu'elle ecrit ───────────────────────────────────────────────────────

// ⚠️⚠️ L'ESSAI QUI JUSTIFIE LE PORTAGE : le plateau neuf part au format §10, a
// l'heure DU SERVEUR. Le JS (`assurer.pb.js`) ecrivait un `t` et un `chantier`
// PAR ETAT et aucun `t` sur le plateau — le porter tel quel aurait fabrique des
// plateaux perimes des leur premiere seconde.
func TestAssurerEcritLeFormatDesCyclesALHeureDuServeur(t *testing.T) {
	d := vierge()
	_, rec := creer(t, d)

	if moteur.Entier(rec["t"], -1) != d.t {
		t.Errorf("le plateau doit porter le `t` du serveur : %v", rec["t"])
	}
	etats, ok := rec["etats"].([]any)
	if !ok || len(etats) != 2 {
		t.Fatalf("2 etats attendus (la 3e est hors plateau) : %v", rec["etats"])
	}
	e, ok := etats[0].(moteur.EtatEcrit)
	if !ok {
		t.Fatalf("les etats doivent etre ecrits par `EcrireEtats` (§10) : %T", etats[0])
	}
	if e.TCycle != 0 {
		t.Errorf("un plateau neuf commence cycle a zero : t_cycle = %d", e.TCycle)
	}
	if e.Navettes == nil {
		t.Error("`navettes` doit etre une liste vide, jamais absente")
	}
	if e.ChantierFin != nil {
		t.Error("⚠️ le chantier de l'admin ne se legue pas")
	}
}

// ⚠️ LA LECON DU 12/09, REJOUEE ICI : les stocks du modele doivent arriver
// jusqu'en base. `CreerBatiment` lit `stock` en `map[string]any` de `float64` —
// preparer l'intermediaire en `map[string]int` passe la compilation, passe a
// l'oeil, et perd TOUT le stock sans un mot.
func TestAssurerRecopieLesStocksDuModele(t *testing.T) {
	d := vierge()
	_, rec := creer(t, d)
	e := rec["etats"].([]any)[0].(moteur.EtatEcrit)
	// 40 du modele + les 100 de la dotation : la ferme est le seul entrepot.
	if e.Stock["ble"] != 140 {
		t.Errorf("stock recopie puis dote attendu 140, lu %v", e.Stock)
	}
}

func TestAssurerVerseLaDotation(t *testing.T) {
	d := vierge()
	r, rec := creer(t, d)

	res, _ := rec["reserve"].(map[string]int)
	if res["or"] != 250 {
		t.Errorf("un FluxStock tombe dans la reserve : %v", rec["reserve"])
	}
	dot := r.Corps["dotation"].(map[string]any)
	if v := dot["versee"].(map[string]int); v["or"] != 250 || v["ble"] != 100 {
		t.Errorf("dotation versee : %v", v)
	}
	if p := dot["perdue"].(map[string]int); len(p) != 0 {
		t.Errorf("rien ne devait etre perdu : %v", p)
	}
}

// ⚠️ CE QUI NE RENTRE PAS EST DIT, JAMAIS AVALE. Un modele sans entrepot pour
// une ressource ferait sinon demarrer le joueur sans elle, et aucun message
// n'existerait pour l'expliquer.
func TestAssurerDitCeQuiNaPasTrouveDePlace(t *testing.T) {
	d := vierge()
	d.templates[0]["amorcage"] = js(map[string]any{"ressources_depart": []any{
		// `pain` n'est stocke que par la maison, qui n'envoie pas : la dotation
		// n'a nulle part ou aller.
		map[string]any{"ressource": "pain", "quantite": 7},
	}})
	r, _ := creer(t, d)
	dot := r.Corps["dotation"].(map[string]any)
	if p := dot["perdue"].(map[string]int); p["pain"] != 7 {
		t.Errorf("les 7 pains perdus doivent etre dits : %v", p)
	}
}

func TestAssurerNommeSelonLeType(t *testing.T) {
	for typ, attendu := range map[string]string{
		"ground": "Ma colonie", "space": "Ma station", "tpt": "Mon plateau (tpt)",
	} {
		if n := NomParDefaut(typ); n != attendu {
			t.Errorf("%s : « %s » au lieu de « %s »", typ, n, attendu)
		}
	}
}

func TestAssurerDemarreLaVersionA1(t *testing.T) {
	d := vierge()
	_, rec := creer(t, d)
	if moteur.Entier(rec["version"], -1) != 1 {
		t.Errorf("⚠️ version 1, pas 0 : « 0 » voudrait dire « jamais ecrit » (%v)", rec["version"])
	}
}

// ─── Ce qu'elle refuse ──────────────────────────────────────────────────────

// ⚠️ UN APPEL DE TROP NE FABRIQUE PAS UN DOUBLON.
func TestAssurerNEcritRienQuandLePlateauExiste(t *testing.T) {
	d := avecModele(neuf())
	d.plateaux[0]["typeOfPlateau"] = "ground"
	r := Assurer(d, "u1", "ground")
	if r.Code != 200 || r.Corps["cree"] != false || r.Corps["ecrit"] != false {
		t.Fatalf("%d %v", r.Code, r.Corps)
	}
	if d.sauves != 0 {
		t.Error("⚠️ elle a ecrit alors que le plateau existait")
	}
	if p := r.Corps["plateau"].(map[string]any); p["id"] != "pl1" {
		t.Errorf("elle doit rendre le plateau EXISTANT : %v", p)
	}
}

// ⚠️ PAS UNE PANNE : UNE SAISIE A FAIRE. Un 500 enverrait chercher un bug la ou
// il manque un modele sur le site.
func TestAssurerSansModeleRend404(t *testing.T) {
	d := vierge()
	d.templates = nil
	r := Assurer(d, "u1", "ground")
	if r.Code != 404 {
		t.Fatalf("code %d", r.Code)
	}
	if !strings.Contains(moteur.Texte(r.Corps["verdict"]), "templates") {
		t.Errorf("le verdict doit nommer la collection : %v", r.Corps["verdict"])
	}
	if d.sauves != 0 {
		t.Error("rien ne doit etre ecrit")
	}
}

func TestAssurerRefuseUnModeleSansDimensions(t *testing.T) {
	d := vierge()
	d.templates[0]["largeur"] = 0.0
	if r := Assurer(d, "u1", "ground"); r.Code != 422 {
		t.Fatalf("code %d (%v)", r.Code, r.Corps["verdict"])
	}
	if d.sauves != 0 {
		t.Error("rien ne doit etre ecrit")
	}
}

// ⚠️ LA GRILLE SE MESURE AVANT D'ECRIRE. Une grille qui ne fait pas
// largeur x hauteur donnerait un plateau dont les cases ne tombent pas en face
// du sol — et ca ne se verrait qu'a la premiere ouverture du jeu.
func TestAssurerRefuseUneGrilleQuiNeFaitPasLaTaille(t *testing.T) {
	d := vierge()
	d.templates[0]["tilesBase64"] = moteur.OctetsVersBase64([]int{1, 2})
	r := Assurer(d, "u1", "ground")
	if r.Code != 422 {
		t.Fatalf("code %d (%v)", r.Code, r.Corps["verdict"])
	}
	if d.sauves != 0 {
		t.Error("rien ne doit etre ecrit")
	}
}

func TestAssurerRefuseSansType(t *testing.T) {
	if r := Assurer(vierge(), "u1", ""); r.Code != 400 {
		t.Fatalf("code %d", r.Code)
	}
}

// ⚠️ SUR UN CATALOGUE ILLISIBLE, aucune case du plateau neuf ne serait
// reconnue : la dotation disparaitrait sur un 200 tranquille.
func TestAssurerRefuseUnCatalogueIllisible(t *testing.T) {
	d := vierge()
	d.cat = moteur.ChargerCatalogue(sourceFactice{})
	r := Assurer(d, "u1", "ground")
	if r.Code != 500 || !strings.Contains(moteur.Texte(r.Corps["verdict"]), "CATALOGUE ILLISIBLE") {
		t.Fatalf("%d %v", r.Code, r.Corps["verdict"])
	}
	if d.sauves != 0 {
		t.Error("rien ne doit etre ecrit")
	}
}

// ⚠️ UNE ECRITURE QUI NE PART PAS SE DIT. C'est exactement la famille de pannes
// qui repondent 200.
func TestAssurerDitQuandLEcritureEchoue(t *testing.T) {
	d := vierge()
	d.sauverCasse = true
	if r := Assurer(d, "u1", "ground"); r.Code != 500 {
		t.Fatalf("code %d", r.Code)
	}
}
