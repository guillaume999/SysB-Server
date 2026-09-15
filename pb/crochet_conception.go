//go:build pocketbase

// ============================================================
//  pb/crochet_conception.go — LA CONCEPTION PAR LES JOUEURS (15/09).
//
//  Un joueur cree, modifie et supprime SES tuiles, SES ressources et SES
//  technos, sur SA planete, dans les LIMITES que l'admin lui a fixees ; il
//  modifie SES deux modeles de plateau. Les regles d'API disent deja « sur sa
//  planete » ; ce crochet est le verrou qui sait COMPTER (les quotas), LIRE LA
//  GRILLE (pas de tuile du jeu peinte chez lui) et NUMEROTER (le tileId).
//
//  ⚠️ LA REGLE N'EST PAS ICI : elle est dans `routes/conception.go`, compilee
//  et testee. Ce fichier lit la base et pose la question.
//
//  ⚠️ L'ADMIN ET LE SUPERUSER PASSENT — sauf pour le tileId : il est attribue
//  par le serveur a TOUTE creation hors superuser (decision du 13/09 : « ni
//  l'admin ni les joueurs ne le saisissent »), et il ne change plus ensuite.
//  Le superuser (l'admin brut de PocketBase) garde la main : c'est la porte de
//  secours.
// ============================================================

package pb

import (
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"

	"sysb/moteur"
	"sysb/routes"
)

var collectionsConcues = []string{"tuiles", "ressources", "technologies"}

// role : "super", "admin", "joueur", ou "" sans compte.
func role(e *core.RecordRequestEvent) string {
	if e.Auth == nil {
		return ""
	}
	if e.Auth.IsSuperuser() {
		return "super"
	}
	if e.Auth.GetString("role") == "admin" {
		return "admin"
	}
	return "joueur"
}

// proprietaireDe : "" pour une planete game, "?" pour une planete introuvable
// (qui ne doit JAMAIS passer pour une planete game).
func proprietaireDe(app core.App, planeteId string) string {
	if planeteId == "" {
		return "?"
	}
	p, err := app.FindRecordById("planetes", planeteId)
	if err != nil || p == nil {
		return "?"
	}
	return p.GetString("proprietaire")
}

func limitesDe(app core.App, uid string) routes.Limites {
	fiches, err := app.FindAllRecords("limites")
	if err != nil {
		return routes.Limites{}
	}
	enr := make([]moteur.Enregistrement, 0, len(fiches))
	for _, f := range fiches {
		enr = append(enr, versLeMoteur(f))
	}
	return routes.LimitesDe(uid, enr)
}

// attribuerTileId : max + 1, lu juste avant d'ecrire. ⚠️ L'index unique sur
// `tuiles.tileId` est le vrai garde-fou : deux creations dans la meme seconde
// lisent le meme max, la seconde est refusee par la base (le site recommence).
func attribuerTileId(e *core.RecordRequestEvent) error {
	plusGrand := 0
	if recs, err := e.App.FindRecordsByFilter("tuiles", "tileId > 0", "-tileId", 1, 0); err == nil && len(recs) > 0 {
		plusGrand = recs[0].GetInt("tileId")
	}
	n, refus := routes.ProchainTileId(plusGrand)
	if refus != "" {
		return e.BadRequestError(refus, nil)
	}
	e.Record.Set("tileId", n)
	return nil
}

// iconeLue : la fiche `icones` citee, ou nil.
func iconeLue(app core.App, id string) *routes.IconeLue {
	if id == "" {
		return nil
	}
	r, err := app.FindRecordById("icones", id)
	if err != nil || r == nil {
		return nil
	}
	return &routes.IconeLue{Chemin: r.GetString("chemin"), Usage: r.GetString("usage"),
		Partage: partageDe(app, "icones", id)}
}

// vignette : pose `chemin_icone` d'apres l'icone choisie par un joueur, ou
// rend le refus (voir `routes.VignetteDeJoueur`).
func vignette(e *core.RecordRequestEvent, planete, proprietaire string) string {
	id := e.Record.GetString("icone")
	chemin, refus := routes.VignetteDeJoueur(e.Collection.Name, id, iconeLue(e.App, id), planete, proprietaire)
	if refus == "" {
		e.Record.Set("chemin_icone", chemin)
	}
	return refus
}

func brancherCrochetConception(app core.App) {
	// ─── Creation ──────────────────────────────────────────────────────────
	app.OnRecordCreateRequest(collectionsConcues...).BindFunc(func(e *core.RecordRequestEvent) error {
		qui := role(e)
		if qui == "super" {
			return e.Next()
		}
		if qui == "" {
			return e.ForbiddenError("Connecte-toi pour concevoir.", nil)
		}
		col := e.Collection.Name
		if col == "tuiles" {
			if err := attribuerTileId(e); err != nil {
				return err
			}
		}
		if qui == "admin" {
			return e.Next()
		}
		planete := e.Record.GetString("planete")
		proprio := proprietaireDe(e.App, planete)
		if refus := routes.RefusPlanete("create", e.Auth.Id, "", "", planete, proprio); refus != "" {
			return e.ForbiddenError(refus, nil)
		}
		if refus := vignette(e, planete, proprio); refus != "" {
			return e.BadRequestError(refus, nil)
		}
		deja, err := e.App.CountRecords(col, dbx.HashExp{"planete": planete})
		if err != nil {
			return e.InternalServerError("comptage impossible", err)
		}
		if refus := routes.RefusQuota(col, int(deja), limitesDe(e.App, e.Auth.Id)); refus != "" {
			return e.ForbiddenError(refus, nil)
		}
		return e.Next()
	})

	// ─── Modification ──────────────────────────────────────────────────────
	app.OnRecordUpdateRequest(collectionsConcues...).BindFunc(func(e *core.RecordRequestEvent) error {
		qui := role(e)
		if qui == "super" {
			return e.Next()
		}
		if qui == "" {
			return e.ForbiddenError("Connecte-toi pour concevoir.", nil)
		}
		avant := e.Record.Original()
		// ⚠️ LE NUMERO D'UNE TUILE NE CHANGE PLUS : un plateau le porte deja.
		if e.Collection.Name == "tuiles" {
			e.Record.Set("tileId", avant.GetInt("tileId"))
		}
		if qui == "admin" {
			return e.Next()
		}
		pAvant, pApres := avant.GetString("planete"), e.Record.GetString("planete")
		propApres := proprietaireDe(e.App, pApres)
		if refus := routes.RefusPlanete("update", e.Auth.Id, pAvant, proprietaireDe(e.App, pAvant),
			pApres, propApres); refus != "" {
			return e.ForbiddenError(refus, nil)
		}
		if refus := vignette(e, pApres, propApres); refus != "" {
			return e.BadRequestError(refus, nil)
		}
		return e.Next()
	})

	// ─── Suppression ───────────────────────────────────────────────────────
	app.OnRecordDeleteRequest(collectionsConcues...).BindFunc(func(e *core.RecordRequestEvent) error {
		qui := role(e)
		if qui == "super" || qui == "admin" {
			return e.Next()
		}
		if qui == "" {
			return e.ForbiddenError("Connecte-toi pour concevoir.", nil)
		}
		p := e.Record.GetString("planete")
		if refus := routes.RefusPlanete("delete", e.Auth.Id, p, proprietaireDe(e.App, p), "", ""); refus != "" {
			return e.ForbiddenError(refus, nil)
		}
		return e.Next()
	})

	// ─── Les modeles de plateau ────────────────────────────────────────────
	// ⚠️ UN JOUEUR MODIFIE SES DEUX MODELES, IL N'EN CREE NI N'EN SUPPRIME :
	// le serveur les fabrique avec la planete (`planetejoueur.go`).
	interdit := func(e *core.RecordRequestEvent) error {
		switch role(e) {
		case "super", "admin":
			return e.Next()
		}
		return e.ForbiddenError("Seul l'administrateur cree ou supprime un modele de plateau.", nil)
	}
	app.OnRecordCreateRequest("templates").BindFunc(interdit)
	app.OnRecordDeleteRequest("templates").BindFunc(interdit)

	app.OnRecordUpdateRequest("templates").BindFunc(func(e *core.RecordRequestEvent) error {
		switch role(e) {
		case "super", "admin":
			return e.Next()
		case "":
			return e.ForbiddenError("Connecte-toi pour concevoir.", nil)
		}
		avant := e.Record.Original()
		pAvant, pApres := avant.GetString("planete"), e.Record.GetString("planete")
		if refus := routes.RefusPlanete("update", e.Auth.Id, pAvant, proprietaireDe(e.App, pAvant),
			pApres, proprietaireDe(e.App, pApres)); refus != "" {
			return e.ForbiddenError(refus, nil)
		}
		// ⚠️ Ni le type ni l'appartenance ne bougent : `assurer` le cherche par
		// (type, planete), et `appartient` dit a qui il est.
		for _, champ := range []string{"typeOfPlateau", "appartient"} {
			if e.Record.GetString(champ) != avant.GetString(champ) {
				return e.ForbiddenError("Le champ « "+champ+" » d'un modele ne se change pas.", nil)
			}
		}
		permis := map[int]bool{}
		tuiles, err := e.App.FindRecordsByFilter("tuiles", "planete = {:p}", "", 0, 0, dbx.Params{"p": pAvant})
		if err != nil {
			return e.InternalServerError("lecture des tuiles impossible", err)
		}
		for _, t := range tuiles {
			permis[t.GetInt("tileId")] = true
		}
		refus := routes.RefusModele(avant.GetInt("largeur"), avant.GetInt("hauteur"),
			e.Record.GetInt("largeur"), e.Record.GetInt("hauteur"),
			limitesDe(e.App, e.Auth.Id), e.Record.GetString("tilesBase64"), permis)
		if refus != "" {
			return e.BadRequestError(refus, nil)
		}
		return e.Next()
	})
}
