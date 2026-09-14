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
	d.planetes = []record{
		{"id": "pTerre", "nom": "Terre", "proprietaire": ""},
		{"id": "pJupiter", "nom": "Jupiter", "proprietaire": ""},
	}
	d.templates = []record{{
		"id": "tpl1", "typeOfPlateau": "ground", "typeOfPlateau2": "Terre", "planete": "pTerre",
		"largeur": 4.0, "hauteur": 1.0,
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
	r := Assurer(d, "u1", "ground", "Terre", "")
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

// ⚠️ SANS MONDE, LE NOM N'A PAS BOUGE : les plateaux d'avant l'etiquetage
// s'appellent toujours « Ma colonie », sans parenthese vide.
func TestAssurerNommeSelonLeType(t *testing.T) {
	for typ, attendu := range map[string]string{
		"ground": "Ma colonie", "space": "Ma station", "tpt": "Mon plateau (tpt)",
	} {
		if n := NomParDefaut(typ, ""); n != attendu {
			t.Errorf("%s : « %s » au lieu de « %s »", typ, n, attendu)
		}
	}
}

// ⚠️⚠️ DEUX COLONIES CHEZ LE MEME JOUEUR DEPUIS LE 13/09, UNE PAR MONDE. Deux
// lignes « Ma colonie » dans sa liste, ce serait deux plateaux qu'on ne peut
// pas departager a l'oeil — exactement la raison qui avait deja fait sortir
// TPTplateau de « Ma colonie » le 29/08.
func TestAssurerMetLeMondeDansLeNom(t *testing.T) {
	for _, c := range []struct{ typ, monde, attendu string }{
		{"ground", "Jupiter", "Ma colonie (Jupiter)"},
		{"space", "Jupiter", "Ma station (Jupiter)"},
		{"ground", "Terre", "Ma colonie (Terre)"},
	} {
		if n := NomParDefaut(c.typ, c.monde); n != c.attendu {
			t.Errorf("%s/%s : « %s » au lieu de « %s »", c.typ, c.monde, n, c.attendu)
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
	r := Assurer(d, "u1", "ground", "Terre", "")
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
	r := Assurer(d, "u1", "ground", "Terre", "")
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
	if r := Assurer(d, "u1", "ground", "Terre", ""); r.Code != 422 {
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
	r := Assurer(d, "u1", "ground", "Terre", "")
	if r.Code != 422 {
		t.Fatalf("code %d (%v)", r.Code, r.Corps["verdict"])
	}
	if d.sauves != 0 {
		t.Error("rien ne doit etre ecrit")
	}
}

func TestAssurerRefuseSansType(t *testing.T) {
	if r := Assurer(vierge(), "u1", "", "Terre", ""); r.Code != 400 {
		t.Fatalf("code %d", r.Code)
	}
}

// ⚠️ SUR UN CATALOGUE ILLISIBLE, aucune case du plateau neuf ne serait
// reconnue : la dotation disparaitrait sur un 200 tranquille.
func TestAssurerRefuseUnCatalogueIllisible(t *testing.T) {
	d := vierge()
	d.cat = moteur.ChargerCatalogue(sourceFactice{})
	r := Assurer(d, "u1", "ground", "Terre", "")
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
	if r := Assurer(d, "u1", "ground", "Terre", ""); r.Code != 500 {
		t.Fatalf("code %d", r.Code)
	}
}

// ─── Les mondes (13/09) ─────────────────────────────────────────────────────

// ⚠️⚠️ L'ESSAI CENTRAL DE LA FONCTIONNALITE : le joueur a DEJA sa colonie
// terrienne, il clique sur Jupiter, et il doit en obtenir une SECONDE. Sans la
// comparaison du monde, la boucle « ai-je deja un plateau de ce type ? »
// tombait sur la Terre et rendait 200 / « tu en as deja un » — avec le mauvais
// plateau dedans, et rien pour s'en apercevoir.
func TestAssurerFabriqueJupiterQuandLaTerreExisteDeja(t *testing.T) {
	d := avecModele(neuf())
	d.plateaux[0]["typeOfPlateau"] = "ground" // la colonie TERRIENNE, deja la
	d.templates = append(d.templates, record{
		"id": "tplJ", "typeOfPlateau": "ground", "typeOfPlateau2": "Jupiter", "planete": "pJupiter",
		"largeur": 4.0, "hauteur": 1.0,
		"tilesBase64": moteur.OctetsVersBase64([]int{1, 2, 0, 0}),
		"etats":       js([]any{map[string]any{"x": 0, "z": 0}}),
	})

	r := Assurer(d, "u1", "ground", "Jupiter", "")
	if r.Code != 200 || r.Corps["cree"] != true {
		t.Fatalf("%d %v", r.Code, r.Corps["verdict"])
	}
	if d.dernier == nil {
		t.Fatal("aucun record cree")
	}
	// ⚠️ ET IL PORTE SON MONDE. Sans ce champ en base, la lecture suivante ne le
	// reconnaitrait pas comme jupiterien et on en refabriquerait un a chaque
	// ouverture du jeu.
	if got := moteur.Texte(d.dernier["typeOfPlateau2"]); got != "Jupiter" {
		t.Errorf("le plateau neuf doit porter son monde : %q", got)
	}
	if got := moteur.Texte(d.dernier["nom"]); got != "Ma colonie (Jupiter)" {
		t.Errorf("le nom doit departager les deux colonies : %q", got)
	}
	// ⚠️ Et il vient du modele de JUPITER, pas de celui de la Terre : le modele
	// terrien porte deux etats, le jupiterien un seul.
	if n := len(d.dernier["etats"].([]any)); n != 1 {
		t.Errorf("⚠️ recopie depuis le mauvais modele : %d etats au lieu de 1", n)
	}
}

// ⚠️ LE MODELE SE CHOISIT SUR LES DEUX ETIQUETTES. Un modele `ground` de la
// Terre ne doit PAS servir a fabriquer Jupiter : le joueur y debarquerait avec
// le decor et les tuiles terriennes, et rien ne le dirait.
func TestAssurerNePrendPasLeModeleDunAutreMonde(t *testing.T) {
	d := vierge() // un seul modele, ground / Terre
	r := Assurer(d, "u1", "ground", "Jupiter", "")
	if r.Code != 404 {
		t.Fatalf("code %d (%v)", r.Code, r.Corps["verdict"])
	}
	// ⚠️ LE VERDICT NOMME LE MONDE : « aucun modele ground » enverrait chercher
	// un modele qui existe — il existe, mais ailleurs.
	if !strings.Contains(moteur.Texte(r.Corps["verdict"]), "Jupiter") {
		t.Errorf("le verdict doit nommer le monde : %v", r.Corps["verdict"])
	}
	if d.sauves != 0 {
		t.Error("rien ne doit etre ecrit")
	}
}

// ⚠️ LA MEME COLONIE DEUX FOIS NE SE FABRIQUE PAS DEUX FOIS, monde compris.
func TestAssurerNEcritRienQuandLeMemeMondeExisteDeja(t *testing.T) {
	d := avecModele(neuf())
	d.plateaux[0]["typeOfPlateau"] = "ground"
	r := Assurer(d, "u1", "ground", "Terre", "")
	if r.Code != 200 || r.Corps["cree"] != false || d.sauves != 0 {
		t.Fatalf("%d %v (sauves=%d)", r.Code, r.Corps["verdict"], d.sauves)
	}
	// ⚠️ Le resume rendu porte le monde : c'est par lui qu'Unity verifie qu'il a
	// bien recu le plateau du monde demande.
	if p := r.Corps["plateau"].(map[string]any); p["typeOfPlateau2"] != "Terre" {
		t.Errorf("le resume doit porter le monde : %v", p)
	}
}

// ⚠️⚠️ LE PIEGE DU CHAMP `t`, REJOUE MOT POUR MOT. `Record.Set` sur un champ
// absent de la collection ne se plaint pas (PocketBase v0.39.2 retombe sur
// `SetRaw`) : la valeur vit dans le record, `Get` la rend, et elle n'est perdue
// qu'au SAVE. Sans ce refus, la colonie de Jupiter serait ecrite SANS monde,
// jamais reconnue, et refabriquee a chaque ouverture du jeu — sous des 200.
func TestAssurerRefuseUnePlateauxSansChampMonde(t *testing.T) {
	d := vierge()
	d.champsPlateau = []string{"id", "ownerId", "nom", "typeOfPlateau", "largeur",
		"hauteur", "tilesBase64", "etats", "reserve", "version", "t"}
	r := Assurer(d, "u1", "ground", "Terre", "")
	if r.Code != 500 || !strings.Contains(moteur.Texte(r.Corps["verdict"]), "typeOfPlateau2") {
		t.Fatalf("%d %v", r.Code, r.Corps["verdict"])
	}
	if d.sauves != 0 {
		t.Error("rien ne doit etre ecrit")
	}
}

// ⚠️ ET LA MEME COLLECTION SERT SANS BRONCHER TANT QU'AUCUN MONDE N'EST
// ⚠️⚠️ CET ESSAI A CHANGE DE SENS LE 14/09, ET C'EST UN CHOIX, PAS UNE
// REGRESSION.
//
// Il garantissait : « une base d'avant les mondes fabrique ses plateaux comme
// avant ». Ce n'est plus tenable — depuis que le modele se cherche PAR PLANETE,
// une base sans planete n'a aucun moyen de retrouver un modele. La garantie
// devient donc : **on le DIT, et on nomme le patch a lancer**.
//
// Le silence serait le vrai danger : un 404 « aucun modele ground » enverrait
// chercher une saisie manquante sur le site, alors qu'il manque une table.
func TestAssurerSansPlanetesLeDitEtNommeLePatch(t *testing.T) {
	d := vierge()
	d.planetes = nil // une base d'avant `patch-planetes-2026-09-14.js`
	r := Assurer(d, "u1", "ground", "Terre", "")
	if r.Code != 404 {
		t.Fatalf("code %d, %v", r.Code, r.Corps["verdict"])
	}
	if v := moteur.Texte(r.Corps["verdict"]); !strings.Contains(v, "patch-planetes") {
		t.Fatalf("le verdict ne nomme pas le patch : %q", v)
	}
	if d.dernier != nil {
		t.Fatal("un plateau a ete fabrique alors qu'aucune planete n'existe")
	}
}

// ⚠️ LE PONT DU NOM A UNE DATE DE PEREMPTION. Tant qu'Unity envoie « Jupiter »
// et pas un identifiant, `assurer` doit retrouver la planete par son NOM — et
// la comparaison est STRICTE, la casse comprise, comme partout depuis le 13/09.
// Cet essai part le jour ou le pont part.
func TestAssurerRetrouveLaPlaneteParSonNomEtRefuseLaCasse(t *testing.T) {
	d := vierge()
	if r := Assurer(d, "u1", "ground", "Terre", ""); r.Code != 200 || r.Corps["cree"] != true {
		t.Fatalf("par le nom : %d %v", r.Code, r.Corps["verdict"])
	}
	if got := moteur.Texte(d.dernier["planete"]); got != "pTerre" {
		t.Fatalf("la planete ecrite est %q", got)
	}
	// ⚠️ L'ETIQUETTE TEXTE EST ECRITE EN PLUS, le temps de la bascule : sans
	// elle, le site et Unity ne verraient pas le plateau neuf.
	if got := moteur.Texte(d.dernier["typeOfPlateau2"]); got != "Terre" {
		t.Fatalf("l'etiquette de compatibilite est %q", got)
	}

	autre := vierge()
	if r := Assurer(autre, "u1", "ground", "terre", ""); r.Code != 404 {
		t.Fatalf("« terre » minuscule devrait etre refuse : %d", r.Code)
	}
}

// L'identifiant l'emporte, et c'est lui qui restera quand le pont partira.
func TestAssurerAccepteLIdentifiantDeLaPlanete(t *testing.T) {
	d := vierge()
	r := Assurer(d, "u1", "ground", "", "pTerre")
	if r.Code != 200 || r.Corps["cree"] != true {
		t.Fatalf("%d %v", r.Code, r.Corps["verdict"])
	}
	if got := moteur.Texte(d.dernier["nom"]); got != "Ma colonie (Terre)" {
		t.Fatalf("le nom par defaut ne porte pas la planete : %q", got)
	}
}

// ⚠️ « Game » — le porte-contenu commun — N'EST PAS JOUABLE, et le verdict doit
// le dire. Aucun modele ne lui est rattache, expres : sans cette phrase, on
// partirait saisir un modele qu'il ne faut surtout pas creer.
func TestAssurerSurLePorteContenuCommunLeDit(t *testing.T) {
	d := vierge()
	d.planetes = append(d.planetes, record{"id": "pGame", "nom": "Game", "proprietaire": ""})
	r := Assurer(d, "u1", "ground", "", "pGame")
	if r.Code != 404 {
		t.Fatalf("code %d", r.Code)
	}
	if v := moteur.Texte(r.Corps["verdict"]); !strings.Contains(v, "ne se joue pas") {
		t.Fatalf("le verdict n'explique pas : %q", v)
	}
}

// ⚠️ LE CHAMP `planete` SUR `plateaux` EST RELEVE, comme `t` et l'etiquette
// avant lui : sans lui, le plateau fabrique ici serait ecrit SANS planete et
// refabrique a chaque ouverture, sous des 200 tranquilles.
func TestAssurerRefuseUneCollectionSansChampPlanete(t *testing.T) {
	d := vierge()
	d.champsPlateau = []string{"id", "ownerId", "nom", "typeOfPlateau", "typeOfPlateau2",
		"largeur", "hauteur", "tilesBase64", "etats", "reserve", "version", "t"}
	r := Assurer(d, "u1", "ground", "Terre", "")
	if r.Code != 500 || !strings.Contains(moteur.Texte(r.Corps["verdict"]), "`planete`") {
		t.Fatalf("%d %v", r.Code, r.Corps["verdict"])
	}
	if d.dernier != nil {
		t.Fatal("un plateau a ete fabrique sans champ `planete`")
	}
}
