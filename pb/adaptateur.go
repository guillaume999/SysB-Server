//go:build pocketbase

// ============================================================
//  pb/adaptateur.go — LA SEULE PARTIE QUI CONNAIT POCKETBASE
//
//  ⚠️⚠️ CE FICHIER N'A JAMAIS ETE COMPILE. Il a ete ecrit dans une session
//  Claude, ou le module PocketBase est injoignable : il depend de
//  `modernc.org/sqlite`, et ni `modernc.org` ni `gitlab.com` ne sont accessibles
//  depuis le conteneur (seul `github.com` l'est). Le module demande en plus Go
//  1.27, contre 1.24 disponible.
//
//  ⚠️ LE `//go:build pocketbase` EN TETE est la pour ca : sans lui,
//  `go build ./...` et `go test ./...` echoueraient partout tant que le module
//  n'est pas la. Une fois le module en place, compile avec :
//
//      go build -tags pocketbase ./cmd/sysb
//
//  Tu peux retirer la ligne le jour ou le module fait partie du projet pour de
//  bon.
//
//  ⚠️ ECRIT POUR **PocketBase v0.39.2** — la version que sert ton instance
//  (lue en bas de l'admin, le 12/09). L'API Go a change en profondeur a la
//  v0.23 (le routeur, puis `core.App`) ; v0.39 est bien au-dela, donc les
//  formes utilisees ici (`app.OnServe()`, `se.Router`, `core.RequestEvent`,
//  `e.Auth`, `app.RunInTransaction`) sont les bonnes.
//
//  ⚠️⚠️ ATTENDS-TOI QUAND MEME A UNE OU DEUX CORRECTIONS A LA PREMIERE
//  COMPILATION : je n'ai pas pu verifier les signatures contre le vrai module.
//  Si une ne colle pas, c'est ICI et nulle part ailleurs — `routes/`, lui, est
//  compile et teste.
//
//  ⚠️ PINGLE `v0.39.2` DANS `go.mod` plutot que de suivre `latest` : un binaire
//  maison sur une base qui bouge, c'est un matin ou plus rien ne compile.
//
//  ⚠️ CE FICHIER NE DECIDE RIEN. Il route, il authentifie, il ouvre une
//  transaction. Toute regle qui pourrait se tromper vit dans `routes/`.
// ============================================================

package pb

import (
	"github.com/pocketbase/pocketbase/core"

	"sysb/moteur"
	"sysb/routes"
)

// depot : `routes.Depot` branche sur une transaction PocketBase.
//
// ⚠️ LE CATALOGUE EST LU UNE FOIS PAR REQUETE, jamais par plateau : c'est
// l'appelant qui le passe. Le relire a chaque plateau multiplierait par N la
// lecture la plus lourde de la route.
type depot struct {
	app core.App
	cat *moteur.CatalogueCharge
	t   int
}

func (d *depot) Catalogue() *moteur.CatalogueCharge { return d.cat }
func (d *depot) Maintenant() int                    { return d.t }

func (d *depot) PlateauxDe(uid string) ([]moteur.Enregistrement, error) {
	// ⚠️ MEME TRI QUE LE JS (`typeOfPlateau`) : l'ordre des plateaux decide
	// l'ordre des rapports, et un banc qui compare deux sorties n'aime pas les
	// listes qui dansent.
	recs, err := d.app.FindRecordsByFilter("plateaux", "ownerId = {:uid}",
		"typeOfPlateau", 20, 0, map[string]any{"uid": uid})
	if err != nil {
		return nil, err
	}
	out := make([]moteur.Enregistrement, 0, len(recs))
	for _, r := range recs {
		out = append(out, r)
	}
	return out, nil
}

func (d *depot) PlateauParId(id string) (moteur.Enregistrement, error) {
	r, err := d.app.FindRecordById("plateaux", id)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (d *depot) Utilisateur(uid string) (moteur.Enregistrement, error) {
	r, err := d.app.FindRecordById("users", uid)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (d *depot) Sauver(r moteur.Enregistrement) error {
	rec, ok := r.(*core.Record)
	if !ok {
		return errPasUnRecord
	}
	return d.app.Save(rec)
}

type errT string

func (e errT) Error() string { return string(e) }

const errPasUnRecord = errT("ce n'est pas un record PocketBase")

// sourceRecords : ce que `moteur.ChargerCatalogue` demande.
type sourceRecords struct{ app core.App }

func (s sourceRecords) Tous(collection string) []moteur.Enregistrement {
	recs, err := s.app.FindAllRecords(collection)
	if err != nil {
		return nil
	}
	out := make([]moteur.Enregistrement, 0, len(recs))
	for _, r := range recs {
		out = append(out, r)
	}
	return out
}

// qui : l'identite du demandeur.
//
// ⚠️ UN SUPERUSER DOIT NOMMER LE JOUEUR. Sans ca, un outil d'administration
// ferait avancer le plateau de quelqu'un d'autre sans que personne le sache.
func qui(e *core.RequestEvent, joueur string) (uid string, estSuper bool, refus string) {
	if e.Auth == nil {
		return "", false, "Connecte-toi : on n'agit que sur SES propres plateaux."
	}
	estSuper = e.Auth.Collection().Name == core.CollectionNameSuperusers
	if !estSuper {
		return e.Auth.Id, false, ""
	}
	if joueur == "" {
		return "", true, "Superuser : nomme le joueur (?joueur=<id> ou \"joueur\" dans le corps)."
	}
	return joueur, true, ""
}

func rendre(e *core.RequestEvent, r routes.Reponse) error {
	return e.JSON(r.Code, r.Corps)
}

func refuser(e *core.RequestEvent, code int, verdict string) error {
	return e.JSON(code, map[string]any{"ok": false, "ecrit": false, "verdict": verdict})
}

// Brancher : les routes sur le serveur. A appeler depuis `main`.
func Brancher(app core.App) {
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {

		// ─── GET /api/sysb/etat ─────────────────────────────────────────────
		// ⚠️ HORS TRANSACTION, et c'est voulu : elle n'ecrit rien.
		se.Router.GET("/api/sysb/etat", func(e *core.RequestEvent) error {
			q := e.Request.URL.Query()
			uid, _, refus := qui(e, q.Get("joueur"))
			if refus != "" {
				return refuser(e, 401, refus)
			}
			d := &depot{app: app, cat: moteur.ChargerCatalogue(sourceRecords{app}),
				t: moteur.Maintenant()}
			return rendre(e, routes.Etat(d, uid, q.Get("plateau")))
		})

		// ─── POST /api/sysb/passe ───────────────────────────────────────────
		// ⚠️ LA SEULE ROUTE AUTORISEE A FAIRE AVANCER LE TEMPS EN BASE.
		se.Router.POST("/api/sysb/passe", func(e *core.RequestEvent) error {
			q := e.Request.URL.Query()
			uid, _, refus := qui(e, q.Get("joueur"))
			if refus != "" {
				return refuser(e, 401, refus)
			}
			// `budget` : 0 = tout le chemin. Sinon la passe s'arrete, ecrit le
			// `t` atteint et rend `fini: false` — le client rappelle (§11.1).
			budget := moteur.Entier(q.Get("budget"), 0)

			cat := moteur.ChargerCatalogue(sourceRecords{app})
			t := moteur.Maintenant()
			var rep routes.Reponse
			err := app.RunInTransaction(func(tx core.App) error {
				rep = routes.Passe(&depot{app: tx, cat: cat, t: t}, uid, q.Get("plateau"), budget)
				if rep.Code >= 500 {
					// ⚠️ ON ANNULE LA TRANSACTION sur une panne — mais on GARDE
					// la reponse pour la rendre telle quelle. Une base a moitie
					// ecrite est pire qu'une passe refusee.
					return errAnnuler
				}
				return nil
			})
			if err != nil && err != errAnnuler {
				return refuser(e, 500, "transaction annulee : "+err.Error())
			}
			return rendre(e, rep)
		})

		// ─── POST /api/sysb/geste ───────────────────────────────────────────
		se.Router.POST("/api/sysb/geste", func(e *core.RequestEvent) error {
			var corps struct {
				Joueur  string  `json:"joueur"`
				Plateau string  `json:"plateau"`
				Action  string  `json:"action"`
				X       int     `json:"x"`
				Z       int     `json:"z"`
				Tuile   int     `json:"tuile"`
				Version *int    `json:"version"`
			}
			if err := e.BindBody(&corps); err != nil {
				return refuser(e, 400, "corps illisible : "+err.Error())
			}
			uid, _, refus := qui(e, corps.Joueur)
			if refus != "" {
				return refuser(e, 401, refus)
			}

			cat := moteur.ChargerCatalogue(sourceRecords{app})
			t := moteur.Maintenant()
			dem := routes.DemandeGeste{Plateau: corps.Plateau, Action: corps.Action,
				X: corps.X, Z: corps.Z, Tuile: corps.Tuile, Version: corps.Version}
			var rep routes.Reponse
			err := app.RunInTransaction(func(tx core.App) error {
				rep = routes.Geste(&depot{app: tx, cat: cat, t: t}, uid, dem)
				if rep.Code >= 500 {
					return errAnnuler
				}
				return nil
			})
			if err != nil && err != errAnnuler {
				return refuser(e, 500, "transaction annulee : "+err.Error())
			}
			return rendre(e, rep)
		})

		return se.Next()
	})
}

const errAnnuler = errT("annulation volontaire")
