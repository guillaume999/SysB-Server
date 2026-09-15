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

// enreg : un `*core.Record` vu par le moteur.
//
// ⚠️⚠️ TOUT RECORD QUI PART VERS LE MOTEUR PASSE PAR ICI, ET C'EST LE POINT.
// PocketBase rend un champ `json` dans un `types.JSONRaw` — un type NOMME sur
// `[]byte` — et le `switch v.(type)` du moteur compare des types EXACTS : il ne
// le voyait pas. Resultat au premier demarrage reel (12/09) : TOUS les champs
// json a nil, `GET /api/sysb/etat` -> 500 « CATALOGUE ILLISIBLE », 28 tuiles
// lues, **0 palier et 0 refusee** (ce couple de zeros est la signature : des
// tuiles mal saisies seraient REFUSEES, pas vides).
//
// ⚠️ NE JAMAIS RENDRE UN `*core.Record` NU au moteur. La reparation est ici, a
// l'entree, a UN seul endroit — pas dans `moteur/lecture.go`, qui n'a pas le
// droit de connaitre PocketBase, et surtout pas aux deux (`moteur` a un test
// qui tombe si quelqu'un l'y remet).
type enreg struct{ *core.Record }

func (e enreg) Get(nom string) any { return Normaliser(e.Record.Get(nom)) }

// versLeMoteur : le seul convertisseur. Utiliser partout ou un record sort.
func versLeMoteur(r *core.Record) moteur.Enregistrement { return enreg{r} }

// depot : `routes.Depot` branche sur une transaction PocketBase.
//
// ⚠️ LE CATALOGUE EST LU UNE FOIS PAR REQUETE, jamais par plateau : c'est
// l'appelant qui le passe. Le relire a chaque plateau multiplierait par N la
// lecture la plus lourde de la route.
type depot struct {
	app core.App
	// ⚠️ LES CATALOGUES DE LA REQUETE (15/09) : le global et ceux des planetes,
	// tous tires de la MEME lecture des collections.
	cats *routes.Catalogues
	t    int
}

func (d *depot) Catalogue() *moteur.CatalogueCharge { return d.cats.Global() }

func (d *depot) CatalogueDe(planeteId string) *moteur.CatalogueCharge {
	return d.cats.De(planeteId)
}

// catalogues : lus une fois par requete, HORS transaction (comme avant), et
// filtres par planete a la demande.
func catalogues(app core.App) *routes.Catalogues {
	return routes.NouveauxCatalogues(sourceRecords{app}, func() ([]moteur.Enregistrement, error) {
		return (&depot{app: app}).Planetes()
	})
}
func (d *depot) Maintenant() int { return d.t }

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
		out = append(out, versLeMoteur(r))
	}
	return out, nil
}

func (d *depot) PlateauParId(id string) (moteur.Enregistrement, error) {
	r, err := d.app.FindRecordById("plateaux", id)
	if err != nil {
		return nil, err
	}
	return versLeMoteur(r), nil
}

func (d *depot) Utilisateur(uid string) (moteur.Enregistrement, error) {
	r, err := d.app.FindRecordById("users", uid)
	if err != nil {
		return nil, err
	}
	return versLeMoteur(r), nil
}

// ChampsPlateau : les champs que la collection `plateaux` retient VRAIMENT.
//
// ⚠️⚠️ C'EST LE GARDE-FOU DU CHAMP `t`, ET IL NE POUVAIT PAS VIVRE AILLEURS.
// `Partie.Ecrire` le faisait en reposant la valeur puis en la relisant — ca ne
// prouve RIEN : `Record.Set` sur un champ absent de la collection retombe sur
// `SetRaw`, garde la valeur dans le record et la rend a `Get` (v0.39.2). Elle
// n'est perdue qu'au SAVE. Resultat mesure le 13/09 : `plateaux` n'avait
// toujours pas de champ `t`, le temps n'etait jamais range, et plus rien ne
// produisait — sous des 200 tranquilles.
//
// ⚠️ UNE LISTE VIDE EN CAS D'ERREUR, jamais une panne : ne pas savoir lire le
// schema n'est pas une raison de fermer le jeu. C'est `routes.gardeSchema` qui
// en decide.
func (d *depot) ChampsPlateau() []string {
	col, err := d.app.FindCollectionByNameOrId("plateaux")
	if err != nil || col == nil {
		return nil
	}
	noms := make([]string, 0, len(col.Fields))
	for _, f := range col.Fields {
		noms = append(noms, f.GetName())
	}
	return noms
}

// ModeleDuType : le modele d'un type SUR UNE PLANETE, dans `templates`.
//
// ⚠️ `nil, nil` QUAND IL N'Y EN A PAS, et surtout pas une erreur : « aucun
// modele `ground` » n'est pas une panne de lecture, c'est une saisie a faire
// sur le site — et c'est la route qui le dit, en 404 plutot qu'en 500.
//
// ⚠️⚠️ LE FILTRE PORTE SUR LA RELATION `planete` DEPUIS LE 14/09, plus sur le
// texte `typeOfPlateau2`. Ce qui ne change PAS : on prend LE PLUS ANCIEN
// (`created`), donc un brouillon cohabite toujours avec le modele en service.
// Ce qui change : c'est cadre A UNE PLANETE. Sans ce cadre, deux modeles
// `ground` de deux joueurs se departageaient a la date de creation, et le
// joueur debarquait chez quelqu'un d'autre — sans un mot.
//
// ⚠️ LA COMPARAISON RESTE STRICTE, LE VIDE COMPRIS : `planete = ""` ne ramene
// que les modeles non rattaches. Depuis `patch-relation-planete-2026-09-14.js`
// il n'y en a plus aucun — c'est voulu, la route repond alors 404 en nommant la
// planete plutot que de servir n'importe lequel.
func (d *depot) ModeleDuType(typeVoulu, planeteId string) (moteur.Enregistrement, error) {
	recs, err := d.app.FindRecordsByFilter("templates",
		"typeOfPlateau = {:type} && planete = {:planete}",
		"created", 1, 0, map[string]any{"type": typeVoulu, "planete": planeteId})
	if err != nil {
		return nil, err
	}
	if len(recs) == 0 {
		return nil, nil
	}
	return versLeMoteur(recs[0]), nil
}

// Planetes : toutes les planetes, d'un coup.
//
// ⚠️ IL Y EN A UNE POIGNEE — une par monde du jeu, une par joueur. Les lire
// entierement coute moins qu'une requete par plateau, et `geste` en a besoin
// pour CHAQUE plateau du joueur.
//
// ⚠️ UNE LISTE VIDE N'EST PAS UNE PANNE : c'est une base ou
// `patch-planetes-2026-09-14.js` n'est pas passe. Les routes le disent et
// retombent sur l'ancien comportement — elles ne refusent pas de servir.
// ⚠️ Et une collection ABSENTE rend une erreur, pas une liste vide : on la
// ravale ici, pour la meme raison. Un serveur deploye avant le patch doit
// continuer de jouer.
func (d *depot) Planetes() ([]moteur.Enregistrement, error) {
	recs, err := d.app.FindRecordsByFilter("planetes", "id != ''", "created", 500, 0, nil)
	if err != nil {
		return nil, nil
	}
	out := make([]moteur.Enregistrement, 0, len(recs))
	for _, r := range recs {
		out = append(out, versLeMoteur(r))
	}
	return out, nil
}

// NouvellePlanete / NouveauTemplate : les records que `AssurerPlaneteDe` cree
// (planete d'un joueur, a l'inscription et au rattrapage du demarrage).
//
// ⚠️ C'EST LE SERVEUR QUI LES POSE, PAS LE CLIENT. `templates.create` est admin
// dans les regles d'API, exprès : un joueur qui pourrait creer un modele
// pourrait en creer un chez quelqu'un d'autre. Le superuser du binaire passe
// au-dessus des regles — c'est tout l'interet d'avoir une route.
func (d *depot) NouvellePlanete() (moteur.Enregistrement, error) {
	col, err := d.app.FindCollectionByNameOrId("planetes")
	if err != nil {
		return nil, err
	}
	return versLeMoteur(core.NewRecord(col)), nil
}

// TemplatesDe : les modeles rattaches a une planete.
func (d *depot) TemplatesDe(planeteId string) ([]moteur.Enregistrement, error) {
	recs, err := d.app.FindRecordsByFilter("templates", "planete = {:planete}",
		"created", 0, 0, map[string]any{"planete": planeteId})
	if err != nil {
		return nil, err
	}
	out := make([]moteur.Enregistrement, 0, len(recs))
	for _, r := range recs {
		out = append(out, versLeMoteur(r))
	}
	return out, nil
}

// ChampsTemplates : les champs que `templates` retient VRAIMENT — meme garde-fou
// que `ChampsPlateau`, pour `appartient`.
func (d *depot) ChampsTemplates() []string {
	col, err := d.app.FindCollectionByNameOrId("templates")
	if err != nil || col == nil {
		return nil
	}
	noms := make([]string, 0, len(col.Fields))
	for _, f := range col.Fields {
		noms = append(noms, f.GetName())
	}
	return noms
}

func (d *depot) NouveauTemplate() (moteur.Enregistrement, error) {
	col, err := d.app.FindCollectionByNameOrId("templates")
	if err != nil {
		return nil, err
	}
	return versLeMoteur(core.NewRecord(col)), nil
}

// NouveauPlateau : un record `plateaux` neuf, PAS ENCORE ECRIT. C'est `Sauver`
// qui l'ecrit, dans la meme transaction que le reste.
func (d *depot) NouveauPlateau() (moteur.Enregistrement, error) {
	col, err := d.app.FindCollectionByNameOrId("plateaux")
	if err != nil {
		return nil, err
	}
	return versLeMoteur(core.NewRecord(col)), nil
}

// Sauver — ⚠️ IL FAUT DEBALLER : ce que le moteur tient est un `enreg`, pas un
// `*core.Record`. Le refus est explicite plutot que silencieux : une ecriture
// qui ne part pas est exactement le genre de panne qui repond 200.
func (d *depot) Sauver(r moteur.Enregistrement) error {
	e, ok := r.(enreg)
	if !ok {
		return errPasUnRecord
	}
	return d.app.Save(e.Record)
}

type errT string

func (e errT) Error() string { return string(e) }

const errPasUnRecord = errT("ce n'est pas un record PocketBase enveloppe (voir versLeMoteur)")

// sourceRecords : ce que `moteur.ChargerCatalogue` demande.
type sourceRecords struct{ app core.App }

func (s sourceRecords) Tous(collection string) []moteur.Enregistrement {
	recs, err := s.app.FindAllRecords(collection)
	if err != nil {
		return nil
	}
	out := make([]moteur.Enregistrement, 0, len(recs))
	for _, r := range recs {
		out = append(out, versLeMoteur(r))
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
		//
		// ⚠️ ON NE LIT PAS `?type=`, ET C'EST DELIBERE (12/09). Le JS l'acceptait ;
		// ici le client fait les deux pas lui-meme (la liste, puis l'id). NE PAS
		// « le remettre au cas ou » : une route qui accepterait `type` sans que
		// personne l'envoie est du code mort, et une route qui l'ACCEPTE A MOITIE
		// — en l'ignorant — rend la LISTE avec un 200 la ou le client attend un
		// plateau. C'est exactement l'ecran vide sans erreur du 12/09.
		se.Router.GET("/api/sysb/etat", func(e *core.RequestEvent) error {
			q := e.Request.URL.Query()
			uid, _, refus := qui(e, q.Get("joueur"))
			if refus != "" {
				return refuser(e, 401, refus)
			}
			d := &depot{app: app, cats: catalogues(app), t: moteur.Maintenant()}
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

			cats := catalogues(app)
			t := moteur.Maintenant()
			var rep routes.Reponse
			err := app.RunInTransaction(func(tx core.App) error {
				rep = routes.Passe(&depot{app: tx, cats: cats, t: t}, uid, q.Get("plateau"), budget)
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
				Joueur  string `json:"joueur"`
				Plateau string `json:"plateau"`
				Action  string `json:"action"`
				X       int    `json:"x"`
				Z       int    `json:"z"`
				Tuile   int    `json:"tuile"`
				Version *int   `json:"version"`
			}
			if err := e.BindBody(&corps); err != nil {
				return refuser(e, 400, "corps illisible : "+err.Error())
			}
			uid, _, refus := qui(e, corps.Joueur)
			if refus != "" {
				return refuser(e, 401, refus)
			}

			cats := catalogues(app)
			t := moteur.Maintenant()
			dem := routes.DemandeGeste{Plateau: corps.Plateau, Action: corps.Action,
				X: corps.X, Z: corps.Z, Tuile: corps.Tuile, Version: corps.Version}
			var rep routes.Reponse
			err := app.RunInTransaction(func(tx core.App) error {
				rep = routes.Geste(&depot{app: tx, cats: cats, t: t}, uid, dem)
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

		// ─── POST /api/sysb/assurer ─────────────────────────────────────────
		// ⚠️ LA SEULE PORTE PAR LAQUELLE UN COMPTE NEUF OBTIENT UN PLATEAU :
		// `plateaux` est fermee en `role='admin'` depuis le 04/09, le client ne
		// peut plus le fabriquer lui-meme. La retirer rend le jeu injouable
		// pour tout nouvel arrivant — et interdit de vider `plateaux`.
		se.Router.POST("/api/sysb/assurer", func(e *core.RequestEvent) error {
			var corps struct {
				Joueur string `json:"joueur"`
				Type   string `json:"type"`
				// ⚠️ `monde` = LE NOM de la planete (« Terre », « Jupiter »).
				// C'est LE PONT, le temps qu'Unity envoie un identifiant : il
				// part avec la mise a jour du client, le jour meme.
				// Absent = chaine vide, et la comparaison reste stricte : un
				// client qui l'oublie se fait refuser en 404, il ne se fait pas
				// servir la Terre par defaut.
				Monde string `json:"monde"`
				// ⚠️ `planete` = SON IDENTIFIANT. C'est la bonne cle : elle ne
				// depend d'aucun libellé, donc renommer une planete ne casse
				// aucune partie. Quand les deux sont la, c'est elle qui gagne.
				Planete string `json:"planete"`
			}
			if err := e.BindBody(&corps); err != nil {
				return refuser(e, 400, "corps illisible : "+err.Error())
			}
			uid, _, refus := qui(e, corps.Joueur)
			if refus != "" {
				return refuser(e, 401, refus)
			}

			cats := catalogues(app)
			t := moteur.Maintenant()
			var rep routes.Reponse
			// ⚠️ TOUTE LA ROUTE DANS LA TRANSACTION : c'est ce qui rend le
			// controle « ce plateau existe-t-il deja » infranchissable par deux
			// ouvertures simultanees du jeu.
			err := app.RunInTransaction(func(tx core.App) error {
				rep = routes.Assurer(&depot{app: tx, cats: cats, t: t}, uid, corps.Type, corps.Monde, corps.Planete)
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

	brancherCrochetTuiles(app)
	brancherCrochetConception(app)
	brancherPlaneteJoueur(app)
	brancherMessages(app)
	brancherChat(app)
	brancherAmis(app)
	brancherGuildes(app)
}

const errAnnuler = errT("annulation volontaire")
