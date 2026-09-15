//go:build pocketbase

// ============================================================
//  pb/planete_joueur.go — LA PLANETE CREEE D'OFFICE (15/09).
//
//  Trois branchements, aucune regle : la regle vit dans
//  `routes/planetejoueur.go`, compilee et testee.
//
//    1. `users` cree  → la planete du joueur et ses deux modeles ;
//    2. demarrage     → le meme geste pour chaque compte (rattrapage des
//                       comptes d'avant, et reparation d'un echec passe) ;
//    3. `users` cree ou modifie par une requete → pseudo « game » refuse.
//
//  ⚠️ UN ECHEC DE CREATION NE BLOQUE PAS L'INSCRIPTION. Le compte est deja
//  ecrit quand le crochet 1 passe (AfterCreateSuccess) : on journalise, et le
//  rattrapage du prochain demarrage reessaie. Refuser l'inscription pour une
//  planete manquee fermerait le jeu a tout nouveau venu sur un souci de schema.
// ============================================================

package pb

import (
	"github.com/pocketbase/pocketbase/core"

	"sysb/moteur"
	"sysb/routes"
)

// assurerPlaneteDe : `routes.AssurerPlaneteDe` dans une transaction.
//
// ⚠️ La planete et ses modeles partent ENSEMBLE, ou pas du tout.
// ⚠️ Pas de catalogue : la route n'en a pas besoin, et le rattrapage passe sur
// tous les comptes — le charger a chaque fois serait la lecture la plus lourde
// du serveur, pour rien.
func assurerPlaneteDe(app core.App, uid string) routes.Reponse {
	var rep routes.Reponse
	err := app.RunInTransaction(func(tx core.App) error {
		rep = routes.AssurerPlaneteDe(&depot{app: tx, t: moteur.Maintenant()}, uid)
		if rep.Code >= 400 {
			return errAnnuler
		}
		return nil
	})
	if err != nil && err != errAnnuler {
		return routes.Reponse{Code: 500, Corps: map[string]any{
			"ok": false, "verdict": "transaction annulee : " + err.Error()}}
	}
	return rep
}

func journaliser(app core.App, uid string, rep routes.Reponse) {
	if rep.Code >= 400 {
		app.Logger().Error("sysb: planete du joueur NON creee",
			"joueur", uid, "code", rep.Code, "verdict", rep.Corps["verdict"])
		return
	}
	if rep.Corps["ecrit"] == true {
		app.Logger().Info("sysb: planete du joueur", "joueur", uid, "verdict", rep.Corps["verdict"])
	}
}

func brancherPlaneteJoueur(app core.App) {
	// ─── 1. A l'inscription ────────────────────────────────────────────────
	app.OnRecordAfterCreateSuccess("users").BindFunc(func(e *core.RecordEvent) error {
		journaliser(e.App, e.Record.Id, assurerPlaneteDe(e.App, e.Record.Id))
		return e.Next()
	})

	// ─── 2. Au demarrage : tous les comptes ────────────────────────────────
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		users, err := se.App.FindAllRecords("users")
		if err != nil {
			se.App.Logger().Error("sysb: rattrapage des planetes impossible", "erreur", err.Error())
			return se.Next()
		}
		for _, u := range users {
			journaliser(se.App, u.Id, assurerPlaneteDe(se.App, u.Id))
		}
		return se.Next()
	})

	// ─── 3. Le pseudo « game » est reserve ─────────────────────────────────
	// ⚠️ A la creation ET a la modification : sinon on s'inscrit « Samp »
	// puis on se renomme « game ».
	garde := func(e *core.RecordRequestEvent) error {
		if routes.PseudoReserve(e.Record.GetString("pseudo")) {
			return e.BadRequestError(routes.VerdictPseudoReserve, nil)
		}
		return e.Next()
	}
	app.OnRecordCreateRequest("users").BindFunc(garde)
	app.OnRecordUpdateRequest("users").BindFunc(garde)
}
