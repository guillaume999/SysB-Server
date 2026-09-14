//go:build pocketbase

// ============================================================
//  pb/crochet_tuiles.go — LE REFUS DU PARTAGE, AU MOMENT D'ECRIRE UNE TUILE.
//
//  Un joueur ne peut citer, dans une tuile, qu'un modele 3D et une icone que
//  l'administrateur a OUVERTS A SA PLANETE.
//
//  ⚠️⚠️ POURQUOI UN CROCHET ET PAS UNE REGLE D'API. Il faudrait comparer DEUX
//  champs du corps l'un a l'autre — `@request.body.modele.planetes_autorisees`
//  contre `@request.body.planete`. Les regles PocketBase savent traverser une
//  relation du corps, pas arbitrer deux relations du corps entre elles. Une
//  regle ecrite « au plus proche » laisserait donc passer, en silence.
//
//  ⚠️ LA REGLE ELLE-MEME N'EST PAS ICI : elle est dans `routes/partage.go`,
//  compilee et testee. Ce fichier ne fait que LIRE les records et poser la
//  question. C'est la meme frontiere que partout : `pb/` ne decide rien.
//
//  ⚠️ L'ADMIN ET LE SUPERUSER PASSENT. C'est lui qui range le catalogue du jeu,
//  et ses tuiles vivent sur des planetes game — qu'il faudrait de toute facon
//  toutes cocher. Le refus vise le CONCEPTEUR, pas le proprietaire du jeu.
//
//  ⚠️ CE FICHIER N'A JAMAIS ETE COMPILE — meme raison que `adaptateur.go`.
// ============================================================

package pb

import (
	"github.com/pocketbase/pocketbase/core"

	"sysb/routes"
)

// partageDe : ce qu'un record de `tuile3dmodel` ou d'`icones` dit de son partage.
//
// ⚠️ UN RECORD ABSENT REND UN PARTAGE VIDE, donc « a personne » — pas « a tout
// le monde ». Une relation cassee ne doit pas ouvrir une porte.
func partageDe(app core.App, collection, id string) routes.Partageable {
	if id == "" {
		return routes.Partageable{}
	}
	rec, err := app.FindRecordById(collection, id)
	if err != nil || rec == nil {
		return routes.Partageable{}
	}
	return routes.Partageable{
		ToutesPlanetes:     rec.GetBool("toutes_planetes"),
		PlanetesAutorisees: rec.GetStringSlice("planetes_autorisees"),
	}
}

// brancherCrochetTuiles : le meme controle a la creation ET a la modification.
//
// ⚠️ LES DEUX, PAS UNE. Sans le controle a la modification, on cree une tuile
// avec un modele autorise, puis on la modifie pour en citer un autre — et la
// regle ne sert plus a rien.
func brancherCrochetTuiles(app core.App) {
	garde := func(e *core.RecordRequestEvent) error {
		// L'admin du jeu et le superuser rangent le catalogue : ils passent.
		if e.Auth == nil {
			return e.Next()
		}
		if e.Auth.Collection().Name == core.CollectionNameSuperusers {
			return e.Next()
		}
		if e.Auth.GetString("role") == "admin" {
			return e.Next()
		}

		planeteId := e.Record.GetString("planete")
		nomPlanete := planeteId
		planeteGame := false
		if planeteId != "" {
			if p, err := e.App.FindRecordById("planetes", planeteId); err == nil && p != nil {
				nomPlanete = p.GetString("nom")
				// ⚠️ VIDE = PLANETE GAME. Un seul champ separe les deux familles.
				planeteGame = p.GetString("proprietaire") == ""
			}
		}

		modeleId := e.Record.GetString("modele")
		iconeId := e.Record.GetString("icone")

		refus := routes.RefusDeTuile(
			partageDe(e.App, "tuile3dmodel", modeleId),
			partageDe(e.App, "icones", iconeId),
			modeleId != "", iconeId != "",
			planeteId, nomPlanete, planeteGame)

		if refus != "" {
			// 403 et pas 400 : ce n'est pas une saisie invalide, c'est un droit
			// qui manque. Le site affiche le verdict tel quel.
			return e.BadRequestError(refus, nil)
		}
		return e.Next()
	}

	app.OnRecordCreateRequest("tuiles").BindFunc(garde)
	app.OnRecordUpdateRequest("tuiles").BindFunc(garde)
}
