//go:build pocketbase

// ============================================================
//  pb/crochet_messages.go — LES MESSAGES PRIVES (15/09).
//
//  Quatre branchements, aucune regle : les regles vivent dans
//  `routes/messages.go`, compilees et testees. Ici on LIT la base et on pose
//  la question.
//
//    1. `messages_prives` cree  → blocage, AMITIE (crochet_amis.go), debit,
//                                contenu nettoye, les deux
//                                pseudos recopies PAR LE SERVEUR ;
//    2. `messages_prives` modifie → seul `lu`, seul le destinataire ;
//    3. `blocages` cree         → pas soi-meme, un compte qui existe ;
//    4. `signalements` cree     → message prive (son destinataire) ou message
//                                du chat (tous sauf l'auteur), message recopie ;
//    +  GET /api/sysb/joueurs?q= → chercher un joueur par son pseudo.
//
//  ⚠️ LA ROUTE `joueurs` EXISTE PARCE QUE `users` EST FERMEE : un joueur ne lit
//  que son propre compte. Elle ne rend que l'id et le pseudo, jamais l'email.
//
//  ⚠️ LE SUPERUSER PASSE PARTOUT (outil d'administration). L'admin du jeu
//  (`role = admin`), lui, est un joueur comme un autre ici : ses messages
//  prives obeissent aux memes bornes.
//
//  ⚠️ PAS COMPILE (module PocketBase injoignable depuis la session Claude),
//  MAIS TYPE-VERIFIE avec go/types contre les sources de PocketBase v0.39.2
//  (15/09). Ce que ca ne prouve pas : le comportement a l'execution.
// ============================================================

package pb

import (
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"

	"sysb/routes"
)

const (
	colMessages     = "messages_prives"
	colBlocages     = "blocages"
	colSignalements = "signalements"
)

func estSuperuser(e *core.RecordRequestEvent) bool {
	return e.Auth != nil && e.Auth.Collection().Name == core.CollectionNameSuperusers
}

// unJoueur : le compte `users` d'id donne, ou nil. Une erreur de lecture vaut
// « n'existe pas » — refuser un envoi vaut mieux que l'ecrire a l'aveugle.
func unJoueur(app core.App, id string) *core.Record {
	if id == "" {
		return nil
	}
	u, err := app.FindRecordById("users", id)
	if err != nil {
		return nil
	}
	return u
}

// bloqueEntre : l'un des deux a-t-il bloque l'autre ?
//
// ⚠️ UNE ERREUR DE LECTURE VAUT « BLOQUE ». Si la collection manque (patch pas
// passe), les envois sont refuses avec une phrase — mieux qu'un blocage ignore
// en silence.
func bloqueEntre(app core.App, a, b string) bool {
	recs, err := app.FindRecordsByFilter(colBlocages,
		"(bloqueur = {:a} && bloque = {:b}) || (bloqueur = {:b} && bloque = {:a})",
		"", 1, 0, map[string]any{"a": a, "b": b})
	if err != nil {
		app.Logger().Error("sysb: blocages illisibles", "erreur", err.Error())
		return true
	}
	return len(recs) > 0
}

// envoisRecents : les instants des messages de `uid` dans la fenetre de debit.
func envoisRecents(app core.App, uid string, maintenant time.Time) []int {
	depuis := maintenant.Add(-time.Duration(routes.FenetreMessages) * time.Second).
		UTC().Format(types.DefaultDateLayout)
	recs, err := app.FindRecordsByFilter(colMessages,
		"expediteur = {:uid} && created >= {:depuis}",
		"-created", routes.MaxMessagesFenetre+1, 0,
		map[string]any{"uid": uid, "depuis": depuis})
	if err != nil {
		return nil
	}
	out := make([]int, 0, len(recs))
	for _, r := range recs {
		out = append(out, int(r.GetDateTime("created").Time().Unix()))
	}
	return out
}

func brancherMessages(app core.App) {
	// ─── 1. Envoi d'un message ─────────────────────────────────────────────
	app.OnRecordCreateRequest(colMessages).BindFunc(func(e *core.RecordRequestEvent) error {
		if estSuperuser(e) {
			return e.Next()
		}
		if e.Auth == nil {
			return e.UnauthorizedError("Connecte-toi pour envoyer un message.", nil)
		}
		moi := e.Auth.Id
		dest := strings.TrimSpace(e.Record.GetString("destinataire"))
		autre := unJoueur(e.App, dest)
		maintenant := time.Now()

		env := routes.EnvoiMessage{
			Expediteur:         moi,
			Destinataire:       dest,
			Contenu:            e.Record.GetString("contenu"),
			DestinataireExiste: autre != nil,
			Maintenant:         int(maintenant.Unix()),
		}
		// ⚠️ On ne lit le blocage et le debit QUE pour un envoi deja valable
		// sur le texte — deux requetes de moins pour un message vide.
		if autre != nil && dest != moi {
			env.Bloque = bloqueEntre(e.App, moi, dest)
			env.Amis = sontAmis(e.App, moi, dest)
			env.EnvoisRecents = envoisRecents(e.App, moi, maintenant)
		}
		contenu, refus := routes.JugerMessage(env)
		if refus != "" {
			return e.BadRequestError(refus, nil)
		}

		// ⚠️ CE QUE LE CLIENT A ENVOYE POUR CES CHAMPS EST ECRASE : le nom de
		// l'expediteur, c'est le sien ; celui du destinataire, c'est la base.
		e.Record.Set("expediteur", moi)
		e.Record.Set("expediteur_nom", e.Auth.GetString("pseudo"))
		e.Record.Set("destinataire_nom", autre.GetString("pseudo"))
		e.Record.Set("contenu", contenu)
		e.Record.Set("lu", false)
		return e.Next()
	})

	// ─── 2. « Lu » ─────────────────────────────────────────────────────────
	app.OnRecordUpdateRequest(colMessages).BindFunc(func(e *core.RecordRequestEvent) error {
		if estSuperuser(e) {
			return e.Next()
		}
		if e.Auth == nil {
			return e.UnauthorizedError("Connecte-toi.", nil)
		}
		avant := e.Record.Original()
		var changes []string
		for _, c := range []string{"contenu", "expediteur", "destinataire", "expediteur_nom", "destinataire_nom"} {
			if e.Record.GetString(c) != avant.GetString(c) {
				changes = append(changes, c)
			}
		}
		if e.Record.GetBool("lu") != avant.GetBool("lu") {
			changes = append(changes, "lu")
		}
		if refus := routes.ModifAutorisee(e.Auth.Id, avant.GetString("destinataire"), changes); refus != "" {
			return e.ForbiddenError(refus, nil)
		}
		return e.Next()
	})

	// ─── 3. Blocage ────────────────────────────────────────────────────────
	app.OnRecordCreateRequest(colBlocages).BindFunc(func(e *core.RecordRequestEvent) error {
		if estSuperuser(e) {
			return e.Next()
		}
		if e.Auth == nil {
			return e.UnauthorizedError("Connecte-toi.", nil)
		}
		bloque := e.Record.GetString("bloque")
		if refus := routes.JugerBlocage(e.Auth.Id, bloque, unJoueur(e.App, bloque) != nil); refus != "" {
			return e.BadRequestError(refus, nil)
		}
		e.Record.Set("bloqueur", e.Auth.Id)
		return e.Next()
	})

	// ─── 4. Signalement ────────────────────────────────────────────────────
	app.OnRecordCreateRequest(colSignalements).BindFunc(func(e *core.RecordRequestEvent) error {
		if estSuperuser(e) {
			return e.Next()
		}
		if e.Auth == nil {
			return e.UnauthorizedError("Connecte-toi.", nil)
		}
		// ⚠️ DEUX SORTES DE MESSAGE : un message prive (`message`) — seul son
		// destinataire le signale — ou un message du chat (`message_chat`) —
		// tout le monde sauf son auteur. Exactement une des deux.
		msgId := e.Record.GetString("message")
		chatId := e.Record.GetString("message_chat")
		if (msgId == "") == (chatId == "") {
			return e.BadRequestError("Signale un seul message a la fois.", nil)
		}
		var motif, refus, contenu, auteur, auteurNom string
		if msgId != "" {
			msg, err := e.App.FindRecordById(colMessages, msgId)
			if err != nil || msg == nil {
				return e.BadRequestError("Ce message n'existe plus.", nil)
			}
			motif, refus = routes.JugerSignalement(e.Auth.Id, msg.GetString("destinataire"), e.Record.GetString("motif"))
			contenu, auteur, auteurNom = msg.GetString("contenu"), msg.GetString("expediteur"), msg.GetString("expediteur_nom")
		} else {
			msg, err := e.App.FindRecordById(colChat, chatId)
			if err != nil || msg == nil {
				return e.BadRequestError("Ce message n'existe plus.", nil)
			}
			motif, refus = routes.JugerSignalementChat(e.Auth.Id, msg.GetString("auteur"), e.Record.GetString("motif"))
			contenu, auteur, auteurNom = msg.GetString("contenu"), msg.GetString("auteur"), msg.GetString("auteur_nom")
		}
		if refus != "" {
			return e.ForbiddenError(refus, nil)
		}
		// ⚠️ LE MESSAGE EST RECOPIE : c'est la seule chose que l'admin lira de
		// la conversation, et il doit rester lisible meme si le message ou le
		// compte de l'auteur disparait (le chat est purge apres 7 jours).
		e.Record.Set("signaleur", e.Auth.Id)
		e.Record.Set("motif", motif)
		e.Record.Set("contenu_signale", contenu)
		e.Record.Set("auteur_signale", auteur)
		e.Record.Set("auteur_nom", auteurNom)
		e.Record.Set("traite", false)
		return e.Next()
	})

	// ─── GET /api/sysb/joueurs?q= ──────────────────────────────────────────
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		se.Router.GET("/api/sysb/joueurs", func(e *core.RequestEvent) error {
			if e.Auth == nil {
				return refuser(e, 401, "Connecte-toi pour chercher un joueur.")
			}
			q, ok := routes.NormaliserRecherche(e.Request.URL.Query().Get("q"))
			if !ok {
				return e.JSON(200, map[string]any{"ok": true, "joueurs": []routes.Joueur{}})
			}
			// ⚠️ Plus que la limite : `RangerJoueurs` retire soi-meme et remet
			// en tete ceux dont le pseudo COMMENCE par la recherche.
			recs, err := se.App.FindRecordsByFilter("users", "pseudo ~ {:q}", "pseudo",
				routes.RechercheLimite*3, 0, map[string]any{"q": q})
			if err != nil {
				return refuser(e, 500, "recherche impossible : "+err.Error())
			}
			liste := make([]routes.Joueur, 0, len(recs))
			for _, r := range recs {
				liste = append(liste, routes.Joueur{Id: r.Id, Pseudo: r.GetString("pseudo")})
			}
			return e.JSON(200, map[string]any{"ok": true,
				"joueurs": routes.RangerJoueurs(liste, e.Auth.Id, q)})
		})
		return se.Next()
	})
}
