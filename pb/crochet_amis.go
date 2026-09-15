//go:build pocketbase

// ============================================================
//  pb/crochet_amis.go — LES AMIS (15/09).
//
//  Les regles sont dans `routes/amis.go` ; ici on lit la base :
//    1. `amities` cree    → destinataire, blocage, lien existant, 30 max ;
//                           pseudos recopies, statut force a `attente` ;
//    2. `amities` modifie → seul `attente` → `acceptee`, par le destinataire,
//                           30 max de chaque cote ;
//    3. `blocages` cree   → l'amitie (ou la demande) entre les deux disparait.
//
//  ⚠️ LE VERROU : sans lui, deux acceptations simultanees lisent toutes deux
//  « 29 amis » et le joueur finit a 31. Tenu jusqu'a l'ecriture (`e.Next()`).
//  ⚠️ Valable tant qu'il n'y a qu'UN conteneur.
//
//  ⚠️ UNE ERREUR DE LECTURE REFUSE (compte plein, lien existant) : ne pas
//  savoir lire n'ouvre aucune porte.
//
//  ⚠️ PAS COMPILE, MAIS TYPE-VERIFIE avec go/types contre les sources de
//  PocketBase v0.39.2 (15/09).
// ============================================================

package pb

import (
	"sync"

	"github.com/pocketbase/pocketbase/core"

	"sysb/routes"
)

const colAmities = "amities"

var verrouAmis sync.Mutex

// lienEntre : la ligne d'`amities` entre deux comptes, dans un sens ou l'autre.
// `err` non nil = illisible.
func lienEntre(app core.App, a, b string) (*core.Record, error) {
	recs, err := app.FindRecordsByFilter(colAmities,
		"(demandeur = {:a} && destinataire = {:b}) || (demandeur = {:b} && destinataire = {:a})",
		"", 1, 0, map[string]any{"a": a, "b": b})
	if err != nil {
		return nil, err
	}
	if len(recs) == 0 {
		return nil, nil
	}
	return recs[0], nil
}

// sontAmis : une amitie ACCEPTEE lie-t-elle les deux ? Illisible = non.
func sontAmis(app core.App, a, b string) bool {
	r, err := lienEntre(app, a, b)
	return err == nil && r != nil && r.GetString("statut") == routes.StatutAcceptee
}

// compter : le nombre de lignes d'`amities` au filtre donne, borne a MaxAmis+1.
// Illisible = MaxAmis (donc « plein »).
func compter(app core.App, filtre string, uid string) int {
	recs, err := app.FindRecordsByFilter(colAmities, filtre, "", routes.MaxAmis+1, 0,
		map[string]any{"uid": uid})
	if err != nil {
		app.Logger().Error("sysb: amities illisibles", "erreur", err.Error())
		return routes.MaxAmis
	}
	return len(recs)
}

func amisDe(app core.App, uid string) int {
	return compter(app, "statut = 'acceptee' && (demandeur = {:uid} || destinataire = {:uid})", uid)
}

func demandesEnvoyees(app core.App, uid string) int {
	return compter(app, "statut = 'attente' && demandeur = {:uid}", uid)
}

func brancherAmis(app core.App) {
	// ─── 1. Demande ────────────────────────────────────────────────────────
	app.OnRecordCreateRequest(colAmities).BindFunc(func(e *core.RecordRequestEvent) error {
		if estSuperuser(e) {
			return e.Next()
		}
		if e.Auth == nil {
			return e.UnauthorizedError("Connecte-toi.", nil)
		}
		verrouAmis.Lock()
		defer verrouAmis.Unlock()

		moi := e.Auth.Id
		dest := e.Record.GetString("destinataire")
		autre := unJoueur(e.App, dest)
		d := routes.DemandeAmi{Demandeur: moi, Destinataire: dest, DestinataireExiste: autre != nil}
		if autre != nil && dest != moi {
			lien, err := lienEntre(e.App, moi, dest)
			d.DejaLies = err != nil || lien != nil
			d.Bloque = bloqueEntre(e.App, moi, dest)
			d.EngagesDemandeur = amisDe(e.App, moi) + demandesEnvoyees(e.App, moi)
		}
		if refus := routes.JugerDemandeAmi(d); refus != "" {
			return e.BadRequestError(refus, nil)
		}
		e.Record.Set("demandeur", moi)
		e.Record.Set("demandeur_nom", e.Auth.GetString("pseudo"))
		e.Record.Set("destinataire_nom", autre.GetString("pseudo"))
		e.Record.Set("statut", routes.StatutAttente)
		return e.Next()
	})

	// ─── 2. Acceptation ────────────────────────────────────────────────────
	app.OnRecordUpdateRequest(colAmities).BindFunc(func(e *core.RecordRequestEvent) error {
		if estSuperuser(e) {
			return e.Next()
		}
		if e.Auth == nil {
			return e.UnauthorizedError("Connecte-toi.", nil)
		}
		verrouAmis.Lock()
		defer verrouAmis.Unlock()

		avant := e.Record.Original()
		var autres []string
		for _, c := range []string{"demandeur", "destinataire", "demandeur_nom", "destinataire_nom"} {
			if e.Record.GetString(c) != avant.GetString(c) {
				autres = append(autres, c)
			}
		}
		refus := routes.JugerAcceptation(routes.Acceptation{
			Auteur:       e.Auth.Id,
			Destinataire: avant.GetString("destinataire"),
			StatutAvant:  avant.GetString("statut"),
			StatutApres:  e.Record.GetString("statut"),
			AutresChamps: autres,
			AmisAuteur:   amisDe(e.App, e.Auth.Id),
			AmisAutre:    amisDe(e.App, avant.GetString("demandeur")),
		})
		if refus != "" {
			return e.BadRequestError(refus, nil)
		}
		return e.Next()
	})

	// ─── 3. Un blocage defait l'amitie ─────────────────────────────────────
	// ⚠️ APRES la creation reussie : un blocage refuse ne doit rien defaire.
	app.OnRecordAfterCreateSuccess(colBlocages).BindFunc(func(e *core.RecordEvent) error {
		a, b := e.Record.GetString("bloqueur"), e.Record.GetString("bloque")
		if lien, err := lienEntre(e.App, a, b); err == nil && lien != nil {
			if err := e.App.Delete(lien); err != nil {
				e.App.Logger().Error("sysb: amitie non retiree apres blocage", "erreur", err.Error())
			}
		}
		return e.Next()
	})
}
