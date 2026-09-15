//go:build pocketbase

// ============================================================
//  pb/crochet_chat.go — LE CHAT GENERAL DE L'ACCUEIL (15/09).
//
//  Deux branchements, aucune regle (elles sont dans `routes/chat.go`) :
//    1. `chat_general` cree → texte nettoye, UN MESSAGE PAR MINUTE, pseudo
//                             recopie par le serveur ;
//    2. purge quotidienne   → les messages de plus de 7 jours partent.
//
//  ⚠️⚠️ LE VERROU. Sans lui, deux envois partis dans la meme seconde lisent
//  tous deux « dernier message il y a 2 min » et passent tous les deux. On
//  tient le verrou JUSQU'A L'ECRITURE (`e.Next()`) : le suivant lit donc le
//  message du premier. Le chat est un filet d'eau, pas un debit — serialiser
//  ses ecritures ne coute rien. ⚠️ Un seul conteneur sur le NAS : si le serveur
//  passait un jour a plusieurs instances, ce verrou ne suffirait plus.
//
//  ⚠️ PAS COMPILE (module PocketBase injoignable depuis la session Claude),
//  MAIS TYPE-VERIFIE avec go/types contre les sources de PocketBase v0.39.2
//  (15/09) : signatures, champs et imports sont justes. Ce que ca ne prouve
//  pas : le comportement a l'execution.
// ============================================================

package pb

import (
	"sync"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"

	"sysb/routes"
)

const colChat = "chat_general"

var verrouChat sync.Mutex

// dernierChat : l'instant (unix) du dernier message de `uid`, 0 s'il n'y en a pas.
//
// ⚠️ UNE ERREUR DE LECTURE REND « IL Y A UN INSTANT » : on refuse plutot que
// d'ouvrir le robinet quand on ne sait pas lire.
func dernierChat(app core.App, uid string, maintenant time.Time) int {
	recs, err := app.FindRecordsByFilter(colChat, "auteur = {:uid}", "-created", 1, 0,
		map[string]any{"uid": uid})
	if err != nil {
		app.Logger().Error("sysb: chat illisible", "erreur", err.Error())
		return int(maintenant.Unix())
	}
	if len(recs) == 0 {
		return 0
	}
	return int(recs[0].GetDateTime("created").Time().Unix())
}

func brancherChat(app core.App) {
	app.OnRecordCreateRequest(colChat).BindFunc(func(e *core.RecordRequestEvent) error {
		if e.Auth != nil && e.Auth.Collection().Name == core.CollectionNameSuperusers {
			return e.Next()
		}
		if e.Auth == nil {
			return e.UnauthorizedError("Connecte-toi pour ecrire dans le chat.", nil)
		}

		verrouChat.Lock()
		defer verrouChat.Unlock()

		maintenant := time.Now()
		contenu, attente, refus := routes.JugerChat(routes.EnvoiChat{
			Contenu:      e.Record.GetString("contenu"),
			DernierEnvoi: dernierChat(e.App, e.Auth.Id, maintenant),
			Maintenant:   int(maintenant.Unix()),
		})
		if attente > 0 {
			// 429 : c'est ce que le jeu lit pour relancer son compte a rebours.
			return e.TooManyRequestsError(refus, nil)
		}
		if refus != "" {
			return e.BadRequestError(refus, nil)
		}
		e.Record.Set("auteur", e.Auth.Id)
		e.Record.Set("auteur_nom", e.Auth.GetString("pseudo"))
		e.Record.Set("contenu", contenu)
		return e.Next()
	})

	// ─── Purge quotidienne, 4 h du matin (heure du serveur) ───────────────
	app.Cron().MustAdd("sysb-purge-chat", "0 4 * * *", func() {
		limite := time.Now().Add(-time.Duration(routes.DureeVieChat) * time.Second).
			UTC().Format(types.DefaultDateLayout)
		total := 0
		// Par paquets : une purge apres une semaine chargee ne doit pas tout
		// lire d'un coup.
		for {
			recs, err := app.FindRecordsByFilter(colChat, "created < {:limite}", "created", 200, 0,
				map[string]any{"limite": limite})
			if err != nil || len(recs) == 0 {
				break
			}
			for _, r := range recs {
				if err := app.Delete(r); err != nil {
					app.Logger().Error("sysb: purge du chat interrompue", "erreur", err.Error())
					return
				}
				total++
			}
		}
		if total > 0 {
			app.Logger().Info("sysb: chat purge", "messages", total)
		}
	})
}
