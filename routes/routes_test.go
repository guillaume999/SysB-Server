package routes

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"sysb/moteur"
)

// ─── Un depot factice : une base en memoire, sans PocketBase ────────────────

type record map[string]any

func (r record) Get(n string) any    { return r[n] }
func (r record) Set(n string, v any) { r[n] = v }

type depot struct {
	cat      *moteur.CatalogueCharge
	catDe    map[string]*moteur.CatalogueCharge
	t        int
	plateaux []record
	users    map[string]record
	sauves   int
	// Ce qu'`Assurer` demande en plus : les modeles, et le dernier record cree.
	templates []record
	// ⚠️ LES PLANETES DU FAUX. `nil` = une base d'AVANT le patch : c'est le cas
	// qui doit continuer de servir, et l'essai s'en sert.
	planetes []record
	dernier  record
	// nil = la collection complete ; sinon exactement ces champs-la.
	champsPlateau []string
	// ⚠️ `Sauver` echoue quand c'est demande : une route doit dire une panne
	// d'ecriture, pas repondre 200.
	sauverCasse bool
	// Ce que `AssurerPlaneteDe` a fabrique.
	planetesCreees []record
	templatesCrees []record
	// nil = `templates` complete ; sinon exactement ces champs-la.
	champsTemplates []string
}

func (d *depot) Catalogue() *moteur.CatalogueCharge { return d.cat }

// ⚠️ Par defaut, toutes les planetes jouent le meme catalogue — les essais
// d'avant le 15/09. `catDe` en donne un a part a une planete.
func (d *depot) CatalogueDe(planete string) *moteur.CatalogueCharge {
	if c, ok := d.catDe[planete]; ok {
		return c
	}
	return d.cat
}
func (d *depot) Maintenant() int { return d.t }

func (d *depot) PlateauxDe(uid string) ([]moteur.Enregistrement, error) {
	var out []moteur.Enregistrement
	for _, p := range d.plateaux {
		if moteur.Texte(p["ownerId"]) == uid {
			out = append(out, p)
		}
	}
	return out, nil
}

func (d *depot) PlateauParId(id string) (moteur.Enregistrement, error) {
	for _, p := range d.plateaux {
		if moteur.Texte(p["id"]) == id {
			return p, nil
		}
	}
	return nil, nil
}

func (d *depot) Utilisateur(uid string) (moteur.Enregistrement, error) {
	if u, y := d.users[uid]; y {
		return u, nil
	}
	return nil, nil
}

func (d *depot) Sauver(r moteur.Enregistrement) error {
	if d.sauverCasse {
		return errEcriture
	}
	d.sauves++
	// Un record neuf entre dans la base a l'ecriture, pas avant : c'est ce qui
	// rend le second appel d'`Assurer` capable de le retrouver.
	if rec, ok := r.(record); ok {
		for _, p := range d.plateaux {
			if moteur.Texte(p["id"]) == moteur.Texte(rec["id"]) {
				return nil
			}
		}
		d.plateaux = append(d.plateaux, rec)
	}
	return nil
}

// ⚠️ LA LISTE DES CHAMPS EST UNE DONNEE DU FAUX, pas une constante : c'est ce
// qui permet d'eprouver la collection SANS `t` — le vrai etat de la base au
// 13/09.
func (d *depot) ChampsPlateau() []string {
	if d.champsPlateau == nil {
		return []string{"id", "ownerId", "nom", "typeOfPlateau", "typeOfPlateau2", "planete",
			"largeur", "hauteur", "tilesBase64", "etats", "reserve", "version", "t"}
	}
	return d.champsPlateau
}

// ⚠️ LE TYPE **ET** LA PLANETE, COMME LE VRAI FILTRE POCKETBASE. Un faux qui ne
// regarderait que le type rendrait le modele terrien pour Jupiter — et l'essai
// qui doit attraper exactement ca serait vert pour rien.
func (d *depot) ModeleDuType(typeVoulu, planeteId string) (moteur.Enregistrement, error) {
	for _, m := range d.templates {
		if moteur.Texte(m["typeOfPlateau"]) == typeVoulu &&
			moteur.Texte(m["planete"]) == planeteId {
			return m, nil
		}
	}
	// ⚠️ `nil, nil` : « il n'y en a pas » n'est pas une panne de lecture.
	return nil, nil
}

// ⚠️ Les planetes creees en font partie : un second appel doit les retrouver.
func (d *depot) Planetes() ([]moteur.Enregistrement, error) {
	out := make([]moteur.Enregistrement, 0, len(d.planetes))
	for _, p := range d.planetes {
		out = append(out, p)
	}
	for _, p := range d.planetesCreees {
		out = append(out, p)
	}
	return out, nil
}

func (d *depot) TemplatesDe(planeteId string) ([]moteur.Enregistrement, error) {
	var out []moteur.Enregistrement
	for _, l := range [][]record{d.templates, d.templatesCrees} {
		for _, m := range l {
			if moteur.Texte(m["planete"]) == planeteId {
				out = append(out, m)
			}
		}
	}
	return out, nil
}

func (d *depot) ChampsTemplates() []string {
	if d.champsTemplates == nil {
		return []string{"id", "nom", "typeOfPlateau", "typeOfPlateau2", "planete", "appartient",
			"largeur", "hauteur", "tilesBase64", "etats", "actif", "amorcage"}
	}
	return d.champsTemplates
}

func (d *depot) NouveauPlateau() (moteur.Enregistrement, error) {
	d.dernier = record{"id": "pl-neuf"}
	return d.dernier, nil
}

// ⚠️ LES CREATEURS DE `AssurerPlaneteDe`. Le faux rend un id tout fait : la vraie base
// le genere, mais le sujet des essais est ce qu'on ECRIT dessus, pas l'id.
func (d *depot) NouvellePlanete() (moteur.Enregistrement, error) {
	r := record{"id": "pl-planete-neuve"}
	d.planetesCreees = append(d.planetesCreees, r)
	return r, nil
}

func (d *depot) NouveauTemplate() (moteur.Enregistrement, error) {
	r := record{"id": fmt.Sprintf("tpl-neuf-%d", len(d.templatesCrees)+1)}
	d.templatesCrees = append(d.templatesCrees, r)
	return r, nil
}

type errEcritureT struct{}

func (errEcritureT) Error() string { return "disque plein" }

var errEcriture = errEcritureT{}

func neuf() *depot {
	cat := moteur.ChargerCatalogue(sourceDeTest())
	return &depot{
		cat: cat, t: 100000,
		plateaux: []record{{
			"id": "pl1", "ownerId": "u1", "nom": "Ma colonie",
			"typeOfPlateau": "colonie", "typeOfPlateau2": "Terre", "planete": "pTerre",
			"largeur": 4.0, "hauteur": 1.0, "version": 3.0,
			"tilesBase64": moteur.OctetsVersBase64([]int{1, 2, 0, 0}),
			"t":           float64(100000 - 3600),
			"etats":       `[{"x":0,"z":0,"stock":{"ble":40}},{"x":1,"z":0}]`,
			"reserve":     `{"or":250}`,
		}},
		users: map[string]record{"u1": {"technos": `{"irrigation":1}`}},
	}
}

// ─── LE SCHEMA ──────────────────────────────────────────────────────────────

// ⚠️⚠️ L'ESSAI QUI REMPLACE UN GARDE-FOU QUI NE GARDAIT PAS. `Partie.Ecrire`
// posait `t` puis le relisait pour verifier qu'il etait retenu : PocketBase rend
// toujours ce qu'on vient de poser, meme sur un champ inconnu (v0.39.2,
// `Record.Set` -> `SetRaw`), et la valeur n'est perdue qu'au SAVE. Le controle
// tombait donc toujours juste — pendant qu'en vrai la collection `plateaux`
// n'avait PAS de champ `t`, que le temps n'etait jamais range, et que plus rien
// ne produisait.
func TestLesRoutesRefusentUnePlateauxSansChampT(t *testing.T) {
	sansT := []string{"id", "ownerId", "nom", "typeOfPlateau", "largeur", "hauteur",
		"tilesBase64", "etats", "reserve", "version"}

	for nom, appel := range map[string]func(*depot) Reponse{
		"etat":  func(d *depot) Reponse { return Etat(d, "u1", "pl1") },
		"passe": func(d *depot) Reponse { return Passe(d, "u1", "pl1", 0) },
		"geste": func(d *depot) Reponse {
			return Geste(d, "u1", DemandeGeste{Plateau: "pl1", Action: "poser", X: 2, Z: 0, Tuile: 1})
		},
		// ⚠️ AVEC UN MONDE, et l'ordre des garde-fous compte : `t` d'abord.
		// Cette collection factice n'a pas non plus `typeOfPlateau2`, et le
		// verdict attendu ici reste celui du champ `t` — le plus grave des deux,
		// puisque sans lui RIEN ne produit.
		"assurer": func(d *depot) Reponse { return Assurer(d, "u1", "ground", "Terre", "") },
	} {
		d := avecModele(neuf())
		d.champsPlateau = sansT
		r := appel(d)
		if r.Code != 500 || !strings.Contains(moteur.Texte(r.Corps["verdict"]), "champ `t`") {
			t.Errorf("%s : %d %v", nom, r.Code, r.Corps["verdict"])
		}
		if d.sauves != 0 {
			t.Errorf("%s a ecrit dans une base sans `t`", nom)
		}
	}
}

// ⚠️ VIDE = ON NE SAIT PAS LIRE LE SCHEMA, et ce n'est pas une raison de refuser
// de jouer. Sans cette branche, le moindre accroc a la lecture du schema
// fermerait le jeu.
func TestUneListeDeChampsVideNeJugePas(t *testing.T) {
	d := neuf()
	d.champsPlateau = []string{}
	if r := Etat(d, "u1", "pl1"); r.Code != 200 {
		t.Fatalf("code %d (%v)", r.Code, r.Corps["verdict"])
	}
}

// ─── ETAT ───────────────────────────────────────────────────────────────────

func TestEtatNEcritJamais(t *testing.T) {
	d := neuf()
	r := Etat(d, "u1", "pl1")
	if r.Code != 200 || r.Corps["ok"] != true {
		t.Fatalf("etat : %d %v", r.Code, r.Corps["verdict"])
	}
	if d.sauves != 0 {
		t.Error("⚠️ `etat` ne doit RIEN ecrire — c'est la seule route dont on est sur")
	}
	if r.Corps["ecrit"] != false {
		t.Error("`etat` doit dire qu'elle n'ecrit pas")
	}
	// Le t en base n'a pas bouge non plus.
	if moteur.Entier(d.plateaux[0]["t"], 0) != 100000-3600 {
		t.Error("`etat` a touche au `t` en base")
	}
	// Mais le bloc rendu, lui, est a jour.
	bloc := r.Corps["etat"].(moteur.Bloc)
	if bloc.Plateau.T != 100000 {
		t.Errorf("le bloc doit etre rattrape en memoire : t = %d", bloc.Plateau.T)
	}
}

func TestEtatSansPlateauRendLaListe(t *testing.T) {
	r := Etat(neuf(), "u1", "")
	if r.Code != 200 {
		t.Fatalf("code %d", r.Code)
	}
	if l, ok := r.Corps["plateaux"].([]any); !ok || len(l) != 1 {
		t.Errorf("la liste doit contenir 1 plateau : %v", r.Corps["plateaux"])
	}
	if _, y := r.Corps["etat"]; y {
		t.Error("la liste est un menu, pas une partie : pas de bloc")
	}
}

// ⚠️ LA LISTE PORTE `largeur` / `hauteur`, ET C'EST DEVENU LOAD-BEARING le
// 12/09 : depuis que l'ancien `?type=` du JS n'existe plus, le jeu retrouve le
// plateau d'un type EN LISANT CETTE LISTE. Sans ces deux champs, l'ecran de
// choix affichait « 0x0 » sous un 200 tranquille.
func TestLaListePorteLesDimensions(t *testing.T) {
	r := Etat(neuf(), "u1", "")
	l := r.Corps["plateaux"].([]any)[0].(map[string]any)
	if l["largeur"] != 4 || l["hauteur"] != 1 {
		t.Errorf("largeur/hauteur attendus 4/1, recus %v/%v", l["largeur"], l["hauteur"])
	}
	if l["typeOfPlateau"] != "colonie" {
		t.Errorf("le type sert a CHOISIR le plateau : %v", l["typeOfPlateau"])
	}
	// ⚠️ SANS LE MONDE, LE TYPE NE SUFFIT PLUS A CHOISIR : le joueur a un
	// `ground` par monde, et le jeu ouvrirait le premier venu.
	if l["typeOfPlateau2"] != "Terre" {
		t.Errorf("le monde sert AUSSI a choisir le plateau : %v", l["typeOfPlateau2"])
	}
	if l["id"] != "pl1" {
		t.Errorf("l'id sert a l'OUVRIR ensuite : %v", l["id"])
	}
}

// ⚠️⚠️ LA CLE EST TOUJOURS LA, MEME SANS ETIQUETTE — et c'est CA que cet essai
// garde. Cote client, `""` veut dire « ce plateau n'a pas de monde » et `null`
// (clé absente) veut dire « ce serveur ne connait pas les mondes » : les deux
// menent a des conduites opposees, et un `omitempty` pose un jour par
// distraction les confondrait sans que rien ne rougisse.
func TestLaListeRendLeMondeMemeVide(t *testing.T) {
	d := neuf()
	delete(d.plateaux[0], "typeOfPlateau2")
	l := Etat(d, "u1", "").Corps["plateaux"].([]any)[0].(map[string]any)
	v, y := l["typeOfPlateau2"]
	if !y {
		t.Fatal("la cle doit exister meme sans etiquette : son absence dit « vieux serveur »")
	}
	if v != "" {
		t.Errorf("un plateau sans etiquette rend la chaine vide, pas %v", v)
	}
}

// ⚠️ LE RATTRAPAGE EST RENDU PAR `etat`, PAS SEULEMENT PAR `passe` : c'est ce
// bloc qui alimente le « pendant ton absence... » a l'ouverture du plateau.
// `passe`, elle, ne le met que dans ses `rapports`, un par plateau.
func TestEtatRaconteLAbsence(t *testing.T) {
	r := Etat(neuf(), "u1", "pl1")
	rat, ok := r.Corps["rattrapage"].(map[string]any)
	if !ok {
		t.Fatalf("pas de bloc rattrapage : %v", r.Corps)
	}
	if rat["absence_s"] != 3600 {
		t.Errorf("absence_s attendu 3600, recu %v", rat["absence_s"])
	}
	if rat["depuis"] != 100000-3600 {
		t.Errorf("depuis attendu %d, recu %v", 100000-3600, rat["depuis"])
	}
	// ⚠️ COMPTE AVANT LE RATTRAPAGE : les deux cases du plateau de test.
	if rat["cases"] != 2 {
		t.Errorf("cases attendu 2, recu %v", rat["cases"])
	}
	if _, y := rat["cases_figees"]; !y {
		t.Error("`cases_figees` est la SEULE mesure de penurie du modele a cycles")
	}
}

func TestUnPlateauQuiNestPasAMoiEstIntrouvable(t *testing.T) {
	if r := Etat(neuf(), "u2", "pl1"); r.Code != 404 {
		t.Errorf("attendu 404, recu %d", r.Code)
	}
}

// ─── PASSE ──────────────────────────────────────────────────────────────────

func TestPasseEcritEtNeToucheJamaisAVersion(t *testing.T) {
	d := neuf()
	r := Passe(d, "u1", "pl1", 0)
	if r.Code != 200 || r.Corps["ecrit"] != true {
		t.Fatalf("passe : %d %v", r.Code, r.Corps["verdict"])
	}
	if moteur.Entier(d.plateaux[0]["t"], 0) != 100000 {
		t.Errorf("le `t` ecrit vaut %v, attendu 100000", d.plateaux[0]["t"])
	}
	// ⚠️ `version` dit « le TERRAIN a change ». Une passe n'y touche JAMAIS —
	// sinon le rafraichissement d'une minute fait refuser le geste du joueur
	// par lui-meme (09/09).
	if moteur.Entier(d.plateaux[0]["version"], 0) != 3 {
		t.Errorf("une passe ne doit PAS toucher a `version` : %v", d.plateaux[0]["version"])
	}
}

// ⚠️ ELLE ECRIT DES QUE LE TEMPS A AVANCE, meme si rien n'a bouge dans les
// coffres : `t` EST l'information.
func TestPasseEcritMemeSansChangement(t *testing.T) {
	d := neuf()
	// Un plateau vide d'etats : rien ne peut bouger, mais le temps avance.
	d.plateaux[0]["etats"] = `[]`
	r := Passe(d, "u1", "pl1", 0)
	if r.Corps["ecrit"] != true {
		t.Error("le temps a avance : il faut ecrire, sinon la meme minute est " +
			"recalculee a chaque appel")
	}
}

// ⚠️ UN PLATEAU HORODATE DANS LE FUTUR NE SE RATTRAPE PAS.
func TestPasseRefuseUnPlateauDansLeFutur(t *testing.T) {
	d := neuf()
	d.plateaux[0]["t"] = float64(d.t + 10000)
	r := Passe(d, "u1", "pl1", 0)
	if r.Corps["ecrit"] != false {
		t.Error("un plateau dans le futur ne doit pas etre ecrit")
	}
	rap := r.Corps["rapports"].([]any)[0].(map[string]any)
	if !strings.Contains(moteur.Texte(rap["raison"]), "futur") {
		t.Errorf("la raison doit le dire : %v", rap["raison"])
	}
}

// Le rattrapage par tranches, vu de la route (spec §11.1).
func TestPasseParTranchesLeDitEtReprend(t *testing.T) {
	d := neuf()
	d.plateaux[0]["t"] = float64(d.t - 86400) // un jour d'absence
	appels := 0
	for {
		r := Passe(d, "u1", "pl1", 20) // 20 ruptures par appel
		appels++
		if r.Code != 200 {
			t.Fatalf("appel %d : %d %v", appels, r.Code, r.Corps["verdict"])
		}
		if r.Corps["fini"] == true {
			break
		}
		// ⚠️ Tant que ce n'est pas fini, la passe DOIT avoir ecrit — sinon
		// l'appel suivant repart du meme instant et on tourne en rond.
		if r.Corps["ecrit"] != true {
			t.Fatal("une tranche non finie doit tout de meme ecrire son `t`")
		}
		if appels > 10000 {
			t.Fatal("ca ne progresse pas")
		}
	}
	if moteur.Entier(d.plateaux[0]["t"], 0) != d.t {
		t.Errorf("apres %d tranches, t = %v, attendu %d", appels, d.plateaux[0]["t"], d.t)
	}
	t.Logf("un jour rattrape en %d tranches", appels)
}

func TestPasseDitLaPanneDEcriture(t *testing.T) {
	d := neuf()
	d.sauverCasse = true
	r := Passe(d, "u1", "pl1", 0)
	if r.Code != 500 || r.Corps["ok"] != false {
		t.Errorf("une panne d'ecriture doit se dire, pas repondre 200 : %d %v", r.Code, r.Corps)
	}
}

// ─── GESTE ──────────────────────────────────────────────────────────────────

func TestGesteRefuseUneDemandeMalFormee(t *testing.T) {
	d := neuf()
	for _, cas := range []struct {
		nom string
		dem DemandeGeste
	}{
		{"action inconnue", DemandeGeste{Plateau: "pl1", Action: "danser", X: 2, Z: 0}},
		{"sans plateau", DemandeGeste{Action: "poser", X: 2, Z: 0, Tuile: 1}},
		// ⚠️ 300 EST VALIDE depuis le 15/09 (deux octets par case) : la borne
		// est 65 535.
		{"tuile hors bornes", DemandeGeste{Plateau: "pl1", Action: "poser", X: 2, Z: 0, Tuile: 65536}},
		{"tuile nulle", DemandeGeste{Plateau: "pl1", Action: "poser", X: 2, Z: 0, Tuile: 0}},
		{"x negatif", DemandeGeste{Plateau: "pl1", Action: "poser", X: -1, Z: 0, Tuile: 1}},
	} {
		if r := Geste(d, "u1", cas.dem); r.Code != 400 {
			t.Errorf("%s : attendu 400, recu %d", cas.nom, r.Code)
		}
	}
	if d.sauves != 0 {
		t.Error("une demande mal formee ne doit rien ecrire")
	}
}

func TestGesteMonteLaVersionMemeQuandIlRefuse(t *testing.T) {
	d := neuf()
	// ⚠️ Poser sur une case deja occupee par le meme type : « rien n'a change ».
	r := Geste(d, "u1", DemandeGeste{Plateau: "pl1", Action: "poser", X: 0, Z: 0, Tuile: 1})
	if r.Code != 200 {
		t.Fatalf("%d %v", r.Code, r.Corps["verdict"])
	}
	g := r.Corps["geste"].(map[string]any)
	if g["accepte"] == true {
		t.Error("poser le meme type sur la meme case n'est pas un evenement")
	}
	// ⚠️ `version` MONTE A CHAQUE GESTE, REFUS COMPRIS : le client doit savoir
	// que sa lecture est perimee, meme si le geste n'a rien fait.
	if moteur.Entier(d.plateaux[0]["version"], 0) != 4 {
		t.Errorf("version = %v, attendu 4", d.plateaux[0]["version"])
	}
}

func TestGestePerimeRefuseEtRendLEtatFrais(t *testing.T) {
	d := neuf()
	vieille := 1
	r := Geste(d, "u1", DemandeGeste{Plateau: "pl1", Action: "detruire", X: 0, Z: 0, Version: &vieille})
	g := r.Corps["geste"].(map[string]any)
	if g["perime"] != true || g["accepte"] == true {
		t.Fatalf("un jeton perime doit refuser : %v", g)
	}
	if !strings.Contains(moteur.Texte(g["refus"]), "version 3") {
		t.Errorf("le refus doit nommer la version courante : %v", g["refus"])
	}
	// ...et rendre l'etat A JOUR, pour que le client refasse son geste dessus.
	bloc := r.Corps["etat"].(moteur.Bloc)
	if bloc.Plateau.Version != 4 {
		t.Errorf("le bloc rendu doit porter la version ECRITE : %d", bloc.Plateau.Version)
	}
	// Le sol n'a pas bouge.
	if moteur.Texte(d.plateaux[0]["tilesBase64"]) != moteur.OctetsVersBase64([]int{1, 2, 0, 0}) {
		t.Error("un geste perime ne doit pas toucher au terrain")
	}
}

func TestGesteDetruitEtEcritLeSol(t *testing.T) {
	d := neuf()
	r := Geste(d, "u1", DemandeGeste{Plateau: "pl1", Action: "detruire", X: 0, Z: 0})
	g := r.Corps["geste"].(map[string]any)
	if g["accepte"] != true {
		t.Fatalf("la destruction devrait passer : %v", g["refus"])
	}
	if got := moteur.Texte(d.plateaux[0]["tilesBase64"]); got != moteur.OctetsVersBase64([]int{0, 2, 0, 0}) {
		t.Errorf("le sol doit porter 0 en (0,0) : %s", got)
	}
	// Un seul etat reste.
	var etats []any
	json.Unmarshal([]byte(mustJSON(d.plateaux[0]["etats"])), &etats)
	if len(etats) != 1 {
		t.Errorf("un seul etat doit rester, %d ecrits", len(etats))
	}
}

func mustJSON(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func TestGesteSurLePlateauDunAutreEstRefuse(t *testing.T) {
	d := neuf()
	if r := Geste(d, "u2", DemandeGeste{Plateau: "pl1", Action: "detruire", X: 0, Z: 0}); r.Code != 403 {
		t.Errorf("attendu 403, recu %d", r.Code)
	}
	if d.sauves != 0 {
		t.Error("rien ne doit etre ecrit")
	}
}
