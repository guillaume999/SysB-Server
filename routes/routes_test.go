package routes

import (
	"encoding/json"
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
	t        int
	plateaux []record
	users    map[string]record
	sauves   int
	// ⚠️ `Sauver` echoue quand c'est demande : une route doit dire une panne
	// d'ecriture, pas repondre 200.
	sauverCasse bool
}

func (d *depot) Catalogue() *moteur.CatalogueCharge { return d.cat }
func (d *depot) Maintenant() int                    { return d.t }

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
	return nil
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
			"typeOfPlateau": "colonie", "largeur": 4.0, "hauteur": 1.0, "version": 3.0,
			"tilesBase64": moteur.OctetsVersBase64([]int{1, 2, 0, 0}),
			"t":           float64(100000 - 3600),
			"etats":       `[{"x":0,"z":0,"stock":{"ble":40}},{"x":1,"z":0}]`,
			"reserve":     `{"or":250}`,
		}},
		users: map[string]record{"u1": {"technos": `{"irrigation":1}`}},
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
		{"tuile hors bornes", DemandeGeste{Plateau: "pl1", Action: "poser", X: 2, Z: 0, Tuile: 300}},
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
