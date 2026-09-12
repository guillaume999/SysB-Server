package routes

// Le meme catalogue factice que dans `moteur` — recopie plutot que partage :
// un paquet de test commun ferait dependre le moteur de ses propres tests.

import (
	"encoding/json"

	"sysb/moteur"
)

type recCat map[string]any

func (r recCat) Get(nom string) any    { return r[nom] }
func (r recCat) Set(nom string, v any) { r[nom] = v }

type sourceFactice map[string][]moteur.Enregistrement

func (s sourceFactice) Tous(c string) []moteur.Enregistrement { return s[c] }

func js(v any) string { b, _ := json.Marshal(v); return string(b) }

func sourceDeTest() sourceFactice {
	return sourceFactice{
		// ⚠️ Les champs json arrivent en CHAINE, comme PocketBase les rend sur
		// ce chemin de lecture. `LireJson` doit les avaler.
		"ressources": {
			recCat{"code": "ble", "genre": "stock"},
			recCat{"code": "pain", "genre": "stock"},
			recCat{"code": "or", "genre": "FluxStock"},
			recCat{"code": "habitant", "genre": "mobilise"},
			recCat{"code": "satisfaction", "genre": "indicateur"},
			recCat{"code": "sansgenre"}, // ⚠️ genre vide = "stock"
		},
		"tuiles": {
			recCat{"tileId": 1.0, "nom": "Ferme", "actif": true,
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
			recCat{"tileId": 2.0, "nom": "Maison", "actif": true,
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
			recCat{"tileId": 3.0, "nom": "Vieille scierie", "actif": true,
				"niveaux": js([]any{map[string]any{"niveau": 1, "cycle_minutes": 1,
					"production": []any{map[string]any{"ressource": "ble", "par_minute": 3}}}}),
			},
			// Decochee sur le site : elle n'entre pas, et ce n'est pas une faute.
			recCat{"tileId": 4.0, "nom": "Brouillon", "actif": false},
		},
		"technologies": {
			recCat{"code": "irrigation", "nom": "Irrigation", "batiment": 5.0,
				"debloque": js([]any{60}), "technos_requises": js([]any{"base"})},
			// ⚠️ Sans batiment : brouillon, elle n'entre pas.
			recCat{"code": "vide", "batiment": 0.0},
		},
	}
}
