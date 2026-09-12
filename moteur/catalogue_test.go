package moteur

import (
	"encoding/json"
	"strings"
	"testing"
)

// recordCarte : un record PocketBase reduit a ce que le moteur en lit.
type recordCarte map[string]any

func (r recordCarte) Get(nom string) any    { return r[nom] }
func (r recordCarte) Set(nom string, v any) { r[nom] = v }

type sourceFactice map[string][]Enregistrement

func (s sourceFactice) Tous(c string) []Enregistrement { return s[c] }

func js(v any) string { b, _ := json.Marshal(v); return string(b) }

func sourcePourTest() sourceFactice {
	return sourceFactice{
		// ⚠️ Les champs json arrivent en CHAINE, comme PocketBase les rend sur
		// ce chemin de lecture. `LireJson` doit les avaler.
		"ressources": {
			recordCarte{"code": "ble", "genre": "stock"},
			recordCarte{"code": "pain", "genre": "stock"},
			recordCarte{"code": "or", "genre": "FluxStock"},
			recordCarte{"code": "habitant", "genre": "mobilise"},
			recordCarte{"code": "satisfaction", "genre": "indicateur"},
			recordCarte{"code": "sansgenre"}, // ⚠️ genre vide = "stock"
		},
		"tuiles": {
			recordCarte{"tileId": 1.0, "nom": "Ferme", "actif": true,
				"niveaux": js([]any{
					map[string]any{"niveau": 1, "cycle_minutes": 2,
						"production": []any{map[string]any{"ressource": "ble", "quantite": 10}},
						"cout":       []any{map[string]any{"ressource": "or", "quantite": 50}}},
					map[string]any{"niveau": 2, "cycle_minutes": 1,
						"production": []any{map[string]any{"ressource": "ble", "quantite": 30}}},
				}),
				"logistique": js(map[string]any{
					"stockage": []any{map[string]any{"ressource": "ble", "max": 500}},
					"appros": []any{map[string]any{"sens": "envoi", "cible": "tout", "rayon": 4,
						"debit":   map[string]any{"navettes": 2, "quantite": 50},
						"vitesse": map[string]any{"crans": 1, "periode_s": 20}}},
				}),
				"placement": js([]any{map[string]any{"regle": "support", "base": "liste", "tileIds": []any{0}}}),
			},
			// Une habitation : tranches {seuil, rendement} + `direct`.
			recordCarte{"tileId": 2.0, "nom": "Maison", "actif": true,
				"niveaux": js([]any{map[string]any{"niveau": 1, "cycle_minutes": 1,
					"demarre_partiel": true,
					"utilisation":     []any{map[string]any{"ressource": "ble", "quantite": 10, "direct": true}},
					"production": []any{map[string]any{"ressource": "pain", "quantite": 4,
						"indicateur": "satisfaction",
						"tranches": []any{
							map[string]any{"seuil": 80, "rendement": 100},
							map[string]any{"seuil": 0, "rendement": 60}}}}}}),
				"logistique": js(map[string]any{
					"stockage": []any{map[string]any{"ressource": "habitant", "max": 4},
						map[string]any{"ressource": "pain", "max": 50}}}),
			},
			// ⚠️ CELLE-CI DOIT ETRE REFUSEE : `par_minute` est l'ancienne notation.
			recordCarte{"tileId": 3.0, "nom": "Vieille scierie", "actif": true,
				"niveaux": js([]any{map[string]any{"niveau": 1, "cycle_minutes": 1,
					"production": []any{map[string]any{"ressource": "ble", "par_minute": 3}}}}),
			},
			// Decochee sur le site : elle n'entre pas, et ce n'est pas une faute.
			recordCarte{"tileId": 4.0, "nom": "Brouillon", "actif": false},
		},
		"technologies": {
			recordCarte{"code": "irrigation", "nom": "Irrigation", "batiment": 5.0,
				"debloque": js([]any{60}), "technos_requises": js([]any{"base"})},
			// ⚠️ Sans batiment : brouillon, elle n'entre pas.
			recordCarte{"code": "vide", "batiment": 0.0},
		},
	}
}

func TestCatalogueLitEtRefuse(t *testing.T) {
	c := ChargerCatalogue(sourcePourTest())

	if c.Illisible() {
		t.Fatal("le catalogue ne devrait pas etre illisible")
	}
	if c.Lecture.Ressources != 6 {
		t.Errorf("ressources = %d, attendu 6", c.Lecture.Ressources)
	}
	if c.Lecture.Tuiles != 3 {
		t.Errorf("tuiles actives = %d, attendu 3 (la decochee ne compte pas)", c.Lecture.Tuiles)
	}
	if c.Lecture.Technologies != 1 {
		t.Errorf("technos = %d, attendu 1 (celle sans batiment est un brouillon)", c.Lecture.Technologies)
	}

	// ⚠️ LA TUILE REFUSEE : elle ne joue pas, et le catalogue le DIT.
	if c.TuilePour(3, 1) != nil {
		t.Error("la tuile 3 porte `par_minute` : elle doit etre REFUSEE")
	}
	if len(c.Alertes) != 1 || !strings.Contains(c.Alertes[0], "TUILE REFUSEE 3") {
		t.Errorf("alertes = %v — la tuile refusee doit se dire", c.Alertes)
	}
	if !strings.Contains(c.Refusees[3], "par_minute") {
		t.Errorf("la raison du refus doit nommer `par_minute` : %q", c.Refusees[3])
	}

	// Une tuile decochee rend nil SANS alerte : c'est un etat du catalogue.
	if c.TuilePour(4, 1) != nil {
		t.Error("la tuile decochee ne doit pas jouer")
	}

	// ⚠️ LES PALIERS : ameliorer peut changer le rythme.
	f1, f2 := c.TuilePour(1, 1), c.TuilePour(1, 2)
	if f1 == nil || f2 == nil {
		t.Fatal("les deux paliers de la ferme doivent charger")
	}
	if f1.DureeCycleS != 120 {
		t.Errorf("palier 1 : cycle = %d s, attendu 120 (2 minutes)", f1.DureeCycleS)
	}
	if f2.DureeCycleS != 60 {
		t.Errorf("palier 2 : cycle = %d s, attendu 60", f2.DureeCycleS)
	}
	if f1.NbNiveaux != 2 {
		t.Errorf("« ameliorable » se lit sur le nombre de paliers : %d", f1.NbNiveaux)
	}
	// Un niveau jamais saisi retombe sur le premier palier.
	if c.TuilePour(1, 7) != f1 {
		t.Error("un niveau inconnu doit retomber sur le palier 1")
	}

	// Le stockage : une LISTE en base, une table pour le moteur.
	ble := c.genres.Reg.Id("ble")
	if got := f1.MaxStocke(ble); got != 500 {
		t.Errorf("stockage ble = %d, attendu 500", got)
	}

	// ⚠️ LES TRANCHES : le site ecrit {seuil, rendement}, le moteur lit des
	// couples — et elles doivent etre TRIEES par seuil decroissant.
	m := c.TuilePour(2, 1)
	if m == nil || len(m.Production) != 1 {
		t.Fatal("la maison doit charger")
	}
	tr := m.Production[0].Tranches
	if len(tr) != 2 || tr[0] != [2]int{80, 100} || tr[1] != [2]int{0, 60} {
		t.Errorf("tranches mal lues ou mal triees : %v", tr)
	}
	if !m.Utilisation[0].Direct {
		t.Error("la consommation de la maison est « en direct »")
	}
	if !m.DemarrePartiel {
		t.Error("la maison coche « demarre avec ce qu'il y a »")
	}
	if m.PlacesLogees() != 4 {
		t.Errorf("places logees = %d, attendu 4", m.PlacesLogees())
	}

	// Les regles de pose, typees.
	if len(f1.ReglesPose) != 1 || f1.ReglesPose[0].Regle != "support" {
		t.Errorf("regles de pose mal lues : %+v", f1.ReglesPose)
	}

	// ⚠️ UN GENRE VIDE VAUT « stock » — donc ca voyage et ca prend de la place.
	sg := c.genres.Reg.Id("sansgenre")
	if !c.genres.EnCoffre(sg) {
		t.Error("un genre vide doit valoir `stock`")
	}
	// ⚠️ ET LA REGLE D'APPRO SANS `ressources` DOIT LE VOIR : c'est la preuve
	// que le catalogue a bien ete REFERME (Refiger) apres avoir tout charge.
	// La ferme est chargee AVANT que `sansgenre` ne soit rencontre dans l'ordre
	// des tuiles ; sans `Refiger`, sa liste de transportables serait trop courte.
	trouve := false
	for _, code := range f1.Appros[0].Transportables {
		if code == sg {
			trouve = true
		}
	}
	if !trouve {
		t.Error("une regle sans `ressources` doit transporter TOUS les codes de la " +
			"table des genres — le catalogue n'a pas ete referme")
	}
	// ...mais PAS un indicateur ni un `mobilise` : ils ne voyagent pas.
	for _, code := range f1.Appros[0].Transportables {
		if c.genres.EstIndicateur(code) || c.genres.EstMobilise(code) || c.genres.EstFluxStock(code) {
			t.Errorf("« %s » ne devrait pas voyager", c.genres.Reg.Nom(code))
		}
	}
}

// Un catalogue vide doit se DIRE illisible : un moteur qui n'a rien compris
// ressemble a un moteur qui n'a rien a faire, et rendrait « 0 gagne » au lieu
// d'une erreur.
func TestCatalogueVideSeDitIllisible(t *testing.T) {
	if !ChargerCatalogue(sourceFactice{}).Illisible() {
		t.Error("un catalogue vide doit se dire illisible")
	}
}
