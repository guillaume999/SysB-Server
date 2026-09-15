//go:build pocketbase

// ============================================================
//  pb/guildes.go — LES GUILDES (15/09) : le depot et les routes.
//
//  Les regles sont dans `routes/guildes.go` (testees). Ici : lire et ecrire
//  les cinq collections, calculer l'age de jeu, brancher les routes.
//
//    GET  /api/sysb/guildes              l'etat (liste, ma guilde, mes demandes)
//    POST /api/sysb/guilde/{action}      creer · demander · inviter · repondre ·
//                                        quitter · exclure · role · modifier ·
//                                        dissoudre · salon
//    GET  /api/sysb/guilde/salon?apres=  le salon de MA guilde
//
//  ⚠️ LES COLLECTIONS SONT FERMEES AUX JOUEURS (patch-guildes-2026-09-15.js) :
//  tout passe par ces routes. L'admin lit les guildes et les membres, dissout
//  une guilde et modere le salon directement (regles `role = 'admin'`), et
//  regle `reglages_guildes`.
//
//  ⚠️ UN VERROU + UNE TRANSACTION par ecriture : deux acceptations simultanees
//  ne font pas deborder une guilde, et une ecriture a moitie faite est annulee.
//  (Un seul conteneur : voir crochet_chat.go.)
//
//  ⚠️ PAS COMPILE, MAIS TYPE-VERIFIE avec go/types contre les sources de
//  PocketBase v0.39.2 (15/09).
// ============================================================

package pb

import (
	"strings"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase/core"

	"sysb/moteur"
	"sysb/routes"
)

const (
	colGuildes   = "guildes"
	colMembresG  = "membres_guilde"
	colDemandesG = "demandes_guilde"
	colMessagesG = "messages_guilde"
	colReglagesG = "reglages_guildes"
)

var verrouGuildes sync.Mutex

type depotGuildes struct{ app core.App }

func (d *depotGuildes) Maintenant() int { return int(time.Now().Unix()) }

// Reglages : la premiere fiche (la plus ancienne). Aucune fiche = les defauts.
func (d *depotGuildes) Reglages() (routes.ReglagesGuildes, error) {
	recs, err := d.app.FindRecordsByFilter(colReglagesG, "", "created", 1, 0)
	if err != nil {
		return routes.ReglagesGuildes{}, err
	}
	if len(recs) == 0 {
		return routes.ReglagesParDefaut, nil
	}
	r := recs[0]
	return routes.ReglagesGuildes{
		MembresMax:     r.GetInt("membres_max"),
		OfficiersMax:   r.GetInt("officiers_max"),
		DelaiSalon:     r.GetInt("delai_salon_s"),
		AgeMinCreation: r.GetInt("age_min_creation"),
	}, nil
}

func (d *depotGuildes) NomJoueur(uid string) (string, bool, error) {
	if uid == "" {
		return "", false, nil
	}
	u, err := d.app.FindRecordById("users", uid)
	if err != nil || u == nil {
		// Introuvable n'est pas une panne : le joueur n'existe pas.
		return "", false, nil
	}
	return u.GetString("pseudo"), true, nil
}

// AgeDe : l'age de jeu, lu sur TOUS les plateaux du joueur (toutes planetes).
func (d *depotGuildes) AgeDe(uid string) (int, error) {
	ages, err := d.app.FindAllRecords("ages")
	if err != nil {
		return 0, err
	}
	requis := make([]routes.AgeRequis, 0, len(ages))
	for _, a := range ages {
		r := routes.AgeRequis{Numero: a.GetInt("numero")}
		for _, v := range moteur.ListeJson(Normaliser(a.Get("batiments_requis"))) {
			if n := moteur.Entier(v, 0); n > 0 {
				r.Batiments = append(r.Batiments, n)
			}
		}
		requis = append(requis, r)
	}
	plateaux, err := d.app.FindRecordsByFilter("plateaux", "ownerId = {:uid}", "", 0, 0,
		map[string]any{"uid": uid})
	if err != nil {
		return 0, err
	}
	possedes := map[int]bool{}
	for _, p := range plateaux {
		cases, ok := moteur.LireGrille(p.GetString("tilesBase64"), p.GetInt("largeur"), p.GetInt("hauteur"))
		if !ok {
			continue
		}
		for _, t := range cases {
			if t > 0 {
				possedes[t] = true
			}
		}
	}
	return routes.AgeAtteint(requis, possedes), nil
}

func versGuilde(r *core.Record) routes.Guilde {
	return routes.Guilde{Id: r.Id, Nom: r.GetString("nom"), Description: r.GetString("description"),
		Chef: r.GetString("chef"), Cree: r.GetDateTime("created").String()}
}

func (d *depotGuildes) Guildes() ([]routes.Guilde, error) {
	recs, err := d.app.FindAllRecords(colGuildes)
	if err != nil {
		return nil, err
	}
	out := make([]routes.Guilde, 0, len(recs))
	for _, r := range recs {
		out = append(out, versGuilde(r))
	}
	return out, nil
}

func (d *depotGuildes) GuildeParId(id string) (*routes.Guilde, error) {
	r, err := d.app.FindRecordById(colGuildes, id)
	if err != nil || r == nil {
		return nil, nil
	}
	g := versGuilde(r)
	return &g, nil
}

func (d *depotGuildes) Membres() ([]routes.Membre, error) {
	recs, err := d.app.FindAllRecords(colMembresG)
	if err != nil {
		return nil, err
	}
	out := make([]routes.Membre, 0, len(recs))
	for _, r := range recs {
		out = append(out, routes.Membre{Id: r.Id, Guilde: r.GetString("guilde"), Joueur: r.GetString("joueur"),
			Nom: r.GetString("joueur_nom"), Role: r.GetString("role")})
	}
	return out, nil
}

func (d *depotGuildes) Demandes() ([]routes.DemandeGuilde, error) {
	recs, err := d.app.FindAllRecords(colDemandesG)
	if err != nil {
		return nil, err
	}
	out := make([]routes.DemandeGuilde, 0, len(recs))
	for _, r := range recs {
		out = append(out, routes.DemandeGuilde{Id: r.Id, Guilde: r.GetString("guilde"),
			GuildeNom: r.GetString("guilde_nom"), Joueur: r.GetString("joueur"),
			JoueurNom: r.GetString("joueur_nom"), Sens: r.GetString("sens"),
			Cree: r.GetDateTime("created").String()})
	}
	return out, nil
}

func (d *depotGuildes) InstantDernierMessage(guilde, uid string) (int, error) {
	recs, err := d.app.FindRecordsByFilter(colMessagesG, "guilde = {:g} && auteur = {:u}", "-created", 1, 0,
		map[string]any{"g": guilde, "u": uid})
	if err != nil {
		return 0, err
	}
	if len(recs) == 0 {
		return 0, nil
	}
	return int(recs[0].GetDateTime("created").Time().Unix()), nil
}

func versMessage(r *core.Record) routes.MessageGuilde {
	return routes.MessageGuilde{Id: r.Id, Guilde: r.GetString("guilde"), Auteur: r.GetString("auteur"),
		AuteurNom: r.GetString("auteur_nom"), Contenu: r.GetString("contenu"),
		Cree: r.GetDateTime("created").String()}
}

func (d *depotGuildes) MessagesSalon(guilde, apres string, limite int) ([]routes.MessageGuilde, error) {
	filtre, tri := "guilde = {:g}", "-created"
	params := map[string]any{"g": guilde}
	if apres != "" {
		filtre += " && created > {:apres}"
		params["apres"] = apres
		tri = "created"
	}
	recs, err := d.app.FindRecordsByFilter(colMessagesG, filtre, tri, limite, 0, params)
	if err != nil {
		return nil, err
	}
	out := make([]routes.MessageGuilde, len(recs))
	for i, r := range recs {
		out[i] = versMessage(r)
	}
	if apres == "" {
		// Les plus recents, remis dans l'ordre chronologique.
		for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
			out[i], out[j] = out[j], out[i]
		}
	}
	return out, nil
}

func (d *depotGuildes) nouveau(col string) (*core.Record, error) {
	c, err := d.app.FindCollectionByNameOrId(col)
	if err != nil {
		return nil, err
	}
	return core.NewRecord(c), nil
}

func (d *depotGuildes) CreerGuilde(g *routes.Guilde) error {
	r, err := d.nouveau(colGuildes)
	if err != nil {
		return err
	}
	r.Set("nom", g.Nom)
	r.Set("description", g.Description)
	r.Set("chef", g.Chef)
	if err := d.app.Save(r); err != nil {
		return err
	}
	g.Id = r.Id
	g.Cree = r.GetDateTime("created").String()
	return nil
}

func (d *depotGuildes) SauverGuilde(g routes.Guilde) error {
	r, err := d.app.FindRecordById(colGuildes, g.Id)
	if err != nil {
		return err
	}
	r.Set("description", g.Description)
	r.Set("chef", g.Chef)
	return d.app.Save(r)
}

func (d *depotGuildes) SupprimerGuilde(id string) error {
	r, err := d.app.FindRecordById(colGuildes, id)
	if err != nil {
		return err
	}
	// ⚠️ Membres, demandes et messages partent par CASCADE (relations
	// `cascadeDelete` posees par le patch).
	return d.app.Delete(r)
}

func (d *depotGuildes) AjouterMembre(m *routes.Membre) error {
	r, err := d.nouveau(colMembresG)
	if err != nil {
		return err
	}
	r.Set("guilde", m.Guilde)
	r.Set("joueur", m.Joueur)
	r.Set("joueur_nom", m.Nom)
	r.Set("role", m.Role)
	if err := d.app.Save(r); err != nil {
		return err
	}
	m.Id = r.Id
	return nil
}

func (d *depotGuildes) SauverMembre(m routes.Membre) error {
	r, err := d.app.FindRecordById(colMembresG, m.Id)
	if err != nil {
		return err
	}
	r.Set("role", m.Role)
	return d.app.Save(r)
}

func (d *depotGuildes) RetirerMembre(id string) error {
	r, err := d.app.FindRecordById(colMembresG, id)
	if err != nil {
		return err
	}
	return d.app.Delete(r)
}

func (d *depotGuildes) CreerDemande(x *routes.DemandeGuilde) error {
	r, err := d.nouveau(colDemandesG)
	if err != nil {
		return err
	}
	r.Set("guilde", x.Guilde)
	r.Set("guilde_nom", x.GuildeNom)
	r.Set("joueur", x.Joueur)
	r.Set("joueur_nom", x.JoueurNom)
	r.Set("sens", x.Sens)
	if err := d.app.Save(r); err != nil {
		return err
	}
	x.Id = r.Id
	x.Cree = r.GetDateTime("created").String()
	return nil
}

func (d *depotGuildes) SupprimerDemande(id string) error {
	r, err := d.app.FindRecordById(colDemandesG, id)
	if err != nil {
		return err
	}
	return d.app.Delete(r)
}

func (d *depotGuildes) CreerMessage(m *routes.MessageGuilde) error {
	r, err := d.nouveau(colMessagesG)
	if err != nil {
		return err
	}
	r.Set("guilde", m.Guilde)
	r.Set("auteur", m.Auteur)
	r.Set("auteur_nom", m.AuteurNom)
	r.Set("contenu", m.Contenu)
	if err := d.app.Save(r); err != nil {
		return err
	}
	m.Id = r.Id
	m.Cree = r.GetDateTime("created").String()
	return nil
}

// ecrireGuilde : une action d'ecriture, sous verrou et en transaction.
func ecrireGuilde(app core.App, action func(d routes.DepotGuildes) routes.Reponse) routes.Reponse {
	verrouGuildes.Lock()
	defer verrouGuildes.Unlock()
	var rep routes.Reponse
	err := app.RunInTransaction(func(tx core.App) error {
		rep = action(&depotGuildes{app: tx})
		if rep.Code >= 500 {
			return errAnnuler
		}
		return nil
	})
	if err != nil && err != errAnnuler {
		return routes.Reponse{Code: 500, Corps: map[string]any{"ok": false, "verdict": "transaction annulee : " + err.Error()}}
	}
	return rep
}

func brancherGuildes(app core.App) {
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		se.Router.GET("/api/sysb/guildes", func(e *core.RequestEvent) error {
			uid, _, refus := qui(e, e.Request.URL.Query().Get("joueur"))
			if refus != "" {
				return refuser(e, 401, refus)
			}
			return rendre(e, routes.GuildesEtat(&depotGuildes{app: app}, uid))
		})

		se.Router.GET("/api/sysb/guilde/salon", func(e *core.RequestEvent) error {
			q := e.Request.URL.Query()
			uid, _, refus := qui(e, q.Get("joueur"))
			if refus != "" {
				return refuser(e, 401, refus)
			}
			return rendre(e, routes.SalonLire(&depotGuildes{app: app}, uid, q.Get("apres")))
		})

		se.Router.POST("/api/sysb/guilde/{action}", func(e *core.RequestEvent) error {
			var c struct {
				Joueur      string `json:"joueur"`
				Nom         string `json:"nom"`
				Description string `json:"description"`
				Guilde      string `json:"guilde"`
				Cible       string `json:"cible"`
				Demande     string `json:"demande"`
				Accepter    bool   `json:"accepter"`
				Role        string `json:"role"`
				Contenu     string `json:"contenu"`
			}
			if err := e.BindBody(&c); err != nil {
				return refuser(e, 400, "corps illisible : "+err.Error())
			}
			uid, _, refus := qui(e, c.Joueur)
			if refus != "" {
				return refuser(e, 401, refus)
			}
			var action func(d routes.DepotGuildes) routes.Reponse
			switch strings.ToLower(e.Request.PathValue("action")) {
			case "creer":
				action = func(d routes.DepotGuildes) routes.Reponse { return routes.GuildeCreer(d, uid, c.Nom, c.Description) }
			case "demander":
				action = func(d routes.DepotGuildes) routes.Reponse { return routes.GuildeDemander(d, uid, c.Guilde) }
			case "inviter":
				action = func(d routes.DepotGuildes) routes.Reponse { return routes.GuildeInviter(d, uid, c.Cible) }
			case "repondre":
				action = func(d routes.DepotGuildes) routes.Reponse {
					return routes.GuildeRepondre(d, uid, c.Demande, c.Accepter)
				}
			case "quitter":
				action = func(d routes.DepotGuildes) routes.Reponse { return routes.GuildeQuitter(d, uid) }
			case "exclure":
				action = func(d routes.DepotGuildes) routes.Reponse { return routes.GuildeExclure(d, uid, c.Cible) }
			case "role":
				action = func(d routes.DepotGuildes) routes.Reponse { return routes.GuildeRole(d, uid, c.Cible, c.Role) }
			case "modifier":
				action = func(d routes.DepotGuildes) routes.Reponse { return routes.GuildeModifier(d, uid, c.Description) }
			case "dissoudre":
				action = func(d routes.DepotGuildes) routes.Reponse { return routes.GuildeDissoudre(d, uid) }
			case "salon":
				action = func(d routes.DepotGuildes) routes.Reponse { return routes.SalonEcrire(d, uid, c.Contenu) }
			default:
				return refuser(e, 404, "action de guilde inconnue")
			}
			return rendre(e, ecrireGuilde(app, action))
		})
		return se.Next()
	})
}
