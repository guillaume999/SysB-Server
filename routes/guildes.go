package routes

// ============================================================================
//  guildes.go — LES GUILDES (15/09).
//
//  Demande de Guillaume : un systeme de guilde ; on y entre sur DEMANDE
//  (acceptee par le chef ou un officier) ou sur INVITATION (acceptee par le
//  joueur) ; l'admin regle les limites (membres, officiers, anti-spam du
//  salon, age de jeu minimum pour creer) ; le joueur gere sa guilde sur le
//  site, et un SALON sur le site n'est lisible que par les membres. Dans le
//  jeu : inviter, et repondre aux invitations.
//
//  ⚠️⚠️ TOUT PASSE PAR DES ROUTES, AUCUNE COLLECTION N'EST OUVERTE AUX JOUEURS.
//  « Un officier accepte une demande pour SA guilde » oblige a croiser deux
//  lignes (la demande, et la fiche de membre de celui qui accepte) : une regle
//  d'API PocketBase ne sait pas le dire sans ambiguite. Les routes, si — et
//  elles se testent ici, sans PocketBase.
//
//  ⚠️ UN JOUEUR, UNE GUILDE. Rejoindre efface toutes ses autres demandes et
//  invitations.
//
//  ⚠️ LES ROLES : chef (un seul), officier (au plus `OfficiersMax`), membre.
//    · le chef : tout — roles, exclusion, description, transmission, dissolution ;
//    · un officier : accepte / refuse les demandes, invite, exclut un MEMBRE ;
//    · un membre : lit et ecrit le salon, invite ? NON — seuls chef et
//      officiers invitent ; il peut quitter.
//    · le chef ne QUITTE pas : il transmet, ou il dissout. Seul, il dissout.
// ============================================================================

import (
	"fmt"
	"sort"
	"strings"
)

const (
	RoleChef     = "chef"
	RoleOfficier = "officier"
	RoleMembre   = "membre"

	SensDemande    = "demande"    // le joueur demande a entrer
	SensInvitation = "invitation" // la guilde invite le joueur

	NomGuildeMin         = 3
	NomGuildeMax         = 30
	DescriptionGuildeMax = 500
	MessageGuildeMax     = 500
	// Un joueur n'a pas plus de N demandes en cours (toutes guildes).
	DemandesJoueurMax = 5
	// Le salon rend au plus N messages par lecture.
	SalonLimite = 100
)

// ReglagesGuildes : ce que l'admin regle (collection `reglages_guildes`).
type ReglagesGuildes struct {
	MembresMax     int `json:"membres_max"`
	OfficiersMax   int `json:"officiers_max"`
	DelaiSalon     int `json:"delai_salon_s"`
	AgeMinCreation int `json:"age_min_creation"`
}

// ReglagesParDefaut : ce qui vaut tant que l'admin n'a rien enregistre.
var ReglagesParDefaut = ReglagesGuildes{MembresMax: 20, OfficiersMax: 3, DelaiSalon: 10, AgeMinCreation: 1}

// Normaliser : une valeur absurde (negative, nulle la ou il en faut une)
// retombe sur le defaut plutot que de fermer les guildes en silence.
func (r ReglagesGuildes) Normaliser() ReglagesGuildes {
	d := ReglagesParDefaut
	if r.MembresMax < 1 {
		r.MembresMax = d.MembresMax
	}
	if r.OfficiersMax < 0 {
		r.OfficiersMax = d.OfficiersMax
	}
	if r.DelaiSalon < 0 {
		r.DelaiSalon = d.DelaiSalon
	}
	if r.AgeMinCreation < 0 {
		r.AgeMinCreation = 0
	}
	return r
}

type Guilde struct {
	Id          string `json:"id"`
	Nom         string `json:"nom"`
	Description string `json:"description"`
	Chef        string `json:"chef"`
	Cree        string `json:"created"`
}

type Membre struct {
	Id     string `json:"id"`
	Guilde string `json:"guilde"`
	Joueur string `json:"joueur"`
	Nom    string `json:"nom"`
	Role   string `json:"role"`
}

type DemandeGuilde struct {
	Id        string `json:"id"`
	Guilde    string `json:"guilde"`
	GuildeNom string `json:"guilde_nom"`
	Joueur    string `json:"joueur"`
	JoueurNom string `json:"joueur_nom"`
	Sens      string `json:"sens"`
	Cree      string `json:"created"`
}

type MessageGuilde struct {
	Id        string `json:"id"`
	Guilde    string `json:"guilde"`
	Auteur    string `json:"auteur"`
	AuteurNom string `json:"auteur_nom"`
	Contenu   string `json:"contenu"`
	Cree      string `json:"created"`
}

// AgeRequis : un age et les tileIds qu'il faut TOUS posseder pour l'atteindre.
type AgeRequis struct {
	Numero    int
	Batiments []int
}

// AgeAtteint — L'AGE DE JEU D'UN JOUEUR : on monte les ages dans l'ordre, et
// on s'arrete au premier dont il ne possede pas TOUS les batiments requis.
// Un age sans batiment requis est acquis. Aucun age = 0.
//
// ⚠️ Les ages se suivent : posseder les batiments de l'age 4 sans ceux de
// l'age 2 laisse le joueur a l'age 1.
func AgeAtteint(ages []AgeRequis, possedes map[int]bool) int {
	tries := append([]AgeRequis(nil), ages...)
	sort.Slice(tries, func(a, b int) bool { return tries[a].Numero < tries[b].Numero })
	atteint := 0
	for _, a := range tries {
		if a.Numero <= 0 {
			continue
		}
		for _, t := range a.Batiments {
			if !possedes[t] {
				return atteint
			}
		}
		atteint = a.Numero
	}
	return atteint
}

// DepotGuildes : ce que les routes des guildes lisent et ecrivent.
// ⚠️ Toute erreur de lecture ou d'ecriture fait repondre 500 : on n'invente
// jamais une guilde vide sur une base illisible.
type DepotGuildes interface {
	Maintenant() int
	Reglages() (ReglagesGuildes, error)
	// NomJoueur : le pseudo d'un compte, et s'il existe.
	NomJoueur(uid string) (string, bool, error)
	AgeDe(uid string) (int, error)

	Guildes() ([]Guilde, error)
	GuildeParId(id string) (*Guilde, error)
	Membres() ([]Membre, error)
	Demandes() ([]DemandeGuilde, error)
	// InstantDernierMessage : unix du dernier message de `uid` dans le salon
	// de `guilde`, 0 s'il n'y en a pas.
	InstantDernierMessage(guilde, uid string) (int, error)
	MessagesSalon(guilde, apres string, limite int) ([]MessageGuilde, error)

	CreerGuilde(g *Guilde) error
	SauverGuilde(g Guilde) error
	// SupprimerGuilde : ses membres, demandes et messages partent avec elle.
	SupprimerGuilde(id string) error
	AjouterMembre(m *Membre) error
	SauverMembre(m Membre) error
	RetirerMembre(id string) error
	CreerDemande(d *DemandeGuilde) error
	SupprimerDemande(id string) error
	CreerMessage(m *MessageGuilde) error
}

const (
	VerdictGuildeConnexion    = "Connecte-toi."
	VerdictGuildeDejaMembre   = "Tu fais deja partie d'une guilde."
	VerdictGuildeAucune       = "Tu ne fais partie d'aucune guilde."
	VerdictGuildeIntrouvable  = "Cette guilde n'existe plus."
	VerdictGuildePleine       = "Cette guilde est complete."
	VerdictGuildeNom          = "Le nom d'une guilde fait de 3 a 30 caracteres."
	VerdictGuildeNomPris      = "Ce nom de guilde est deja pris."
	VerdictGuildeDescription  = "La description fait 500 caracteres au plus."
	VerdictGuildeDroit        = "Ton role dans la guilde ne le permet pas."
	VerdictGuildeDejaDemande  = "Une demande ou une invitation est deja en cours."
	VerdictGuildeTropDemandes = "Tu as deja 5 demandes en cours : attends une reponse."
	VerdictGuildeJoueur       = "Ce joueur n'existe pas."
	VerdictGuildeJoueurPris   = "Ce joueur fait deja partie d'une guilde."
	VerdictGuildeSoi          = "Pas sur toi-meme."
	VerdictGuildeDemande      = "Cette demande n'existe plus."
	VerdictGuildeChefQuitte   = "Le chef ne quitte pas sa guilde : transmets-la, ou dissous-la."
	VerdictGuildeOfficiers    = "La guilde a deja son nombre maximum d'officiers."
	VerdictGuildeRole         = "Role inconnu."
	VerdictGuildePasMembre    = "Ce joueur ne fait pas partie de ta guilde."
	VerdictGuildeVide         = "Le message est vide."
	VerdictGuildeLong         = "Le message fait 500 caracteres au plus."
)

func verdictAgeGuilde(min, age int) string {
	return fmt.Sprintf("Il faut avoir atteint l'age %d pour fonder une guilde (tu es a l'age %d).", min, age)
}

func verdictSalonAttente(s int) string {
	return fmt.Sprintf("Anti-spam du salon : encore %d s.", s)
}

func panne(err error) Reponse {
	return erreur(500, "base illisible : "+err.Error(), nil)
}

func refusG(code int, verdict string) Reponse { return erreur(code, verdict, nil) }

func okG(extra map[string]any) Reponse {
	c := map[string]any{"ok": true, "ecrit": true}
	for k, v := range extra {
		c[k] = v
	}
	return Reponse{Code: 200, Corps: c}
}

// vue : tout l'etat lu une fois, pour une requete.
type vue struct {
	reglages ReglagesGuildes
	guildes  []Guilde
	membres  []Membre
	demandes []DemandeGuilde
}

func lire(d DepotGuildes) (*vue, error) {
	r, err := d.Reglages()
	if err != nil {
		return nil, err
	}
	g, err := d.Guildes()
	if err != nil {
		return nil, err
	}
	m, err := d.Membres()
	if err != nil {
		return nil, err
	}
	dm, err := d.Demandes()
	if err != nil {
		return nil, err
	}
	return &vue{reglages: r.Normaliser(), guildes: g, membres: m, demandes: dm}, nil
}

func (v *vue) membreDe(uid string) *Membre {
	for i := range v.membres {
		if v.membres[i].Joueur == uid {
			return &v.membres[i]
		}
	}
	return nil
}

func (v *vue) guilde(id string) *Guilde {
	for i := range v.guildes {
		if v.guildes[i].Id == id {
			return &v.guildes[i]
		}
	}
	return nil
}

func (v *vue) membresDe(guilde string) []Membre {
	var out []Membre
	for _, m := range v.membres {
		if m.Guilde == guilde {
			out = append(out, m)
		}
	}
	return out
}

func (v *vue) compter(guilde, role string) int {
	n := 0
	for _, m := range v.membres {
		if m.Guilde == guilde && (role == "" || m.Role == role) {
			n++
		}
	}
	return n
}

func (v *vue) demande(id string) *DemandeGuilde {
	for i := range v.demandes {
		if v.demandes[i].Id == id {
			return &v.demandes[i]
		}
	}
	return nil
}

func (v *vue) lien(guilde, joueur string) *DemandeGuilde {
	for i := range v.demandes {
		if v.demandes[i].Guilde == guilde && v.demandes[i].Joueur == joueur {
			return &v.demandes[i]
		}
	}
	return nil
}

func gere(role string) bool { return role == RoleChef || role == RoleOfficier }

// ─── Lecture ────────────────────────────────────────────────────────────────

// ResumeGuilde : une guilde dans la liste publique.
type ResumeGuilde struct {
	Id          string `json:"id"`
	Nom         string `json:"nom"`
	Description string `json:"description"`
	ChefNom     string `json:"chef_nom"`
	Membres     int    `json:"membres"`
	Cree        string `json:"created"`
}

// GuildesEtat : GET /api/sysb/guildes — tout ce dont le site et le jeu ont
// besoin, en une lecture : reglages, age du joueur, liste des guildes, SA
// guilde (membres, et demandes s'il la gere), et ses demandes / invitations.
func GuildesEtat(d DepotGuildes, uid string) Reponse {
	if uid == "" {
		return refusG(401, VerdictGuildeConnexion)
	}
	v, err := lire(d)
	if err != nil {
		return panne(err)
	}
	age, err := d.AgeDe(uid)
	if err != nil {
		return panne(err)
	}

	liste := make([]ResumeGuilde, 0, len(v.guildes))
	for _, g := range v.guildes {
		r := ResumeGuilde{Id: g.Id, Nom: g.Nom, Description: g.Description, Cree: g.Cree,
			Membres: v.compter(g.Id, "")}
		for _, m := range v.membres {
			if m.Guilde == g.Id && m.Role == RoleChef {
				r.ChefNom = m.Nom
			}
		}
		liste = append(liste, r)
	}
	sort.Slice(liste, func(a, b int) bool { return strings.ToLower(liste[a].Nom) < strings.ToLower(liste[b].Nom) })

	corps := map[string]any{
		"ok":         true,
		"reglages":   v.reglages,
		"age":        age,
		"peut_creer": age >= v.reglages.AgeMinCreation,
		"guildes":    liste,
	}

	moi := v.membreDe(uid)
	if moi != nil {
		g := v.guilde(moi.Guilde)
		membres := v.membresDe(moi.Guilde)
		sort.Slice(membres, func(a, b int) bool {
			ra, rb := rangRole(membres[a].Role), rangRole(membres[b].Role)
			if ra != rb {
				return ra < rb
			}
			return strings.ToLower(membres[a].Nom) < strings.ToLower(membres[b].Nom)
		})
		ma := map[string]any{"guilde": g, "role": moi.Role, "membres": membres}
		if gere(moi.Role) {
			var dm []DemandeGuilde
			for _, x := range v.demandes {
				if x.Guilde == moi.Guilde {
					dm = append(dm, x)
				}
			}
			ma["demandes"] = nonNul(dm)
		}
		corps["ma_guilde"] = ma
	}
	var miennes []DemandeGuilde
	for _, x := range v.demandes {
		if x.Joueur == uid {
			miennes = append(miennes, x)
		}
	}
	corps["mes_demandes"] = nonNul(miennes)
	return Reponse{Code: 200, Corps: corps}
}

func nonNul(l []DemandeGuilde) []DemandeGuilde {
	if l == nil {
		return []DemandeGuilde{}
	}
	return l
}

func rangRole(r string) int {
	switch r {
	case RoleChef:
		return 0
	case RoleOfficier:
		return 1
	}
	return 2
}

// ─── Creer ──────────────────────────────────────────────────────────────────

func GuildeCreer(d DepotGuildes, uid, nom, description string) Reponse {
	if uid == "" {
		return refusG(401, VerdictGuildeConnexion)
	}
	nom = strings.Join(strings.Fields(NettoyerTexte(nom)), " ")
	description = NettoyerTexte(description)
	if n := len([]rune(nom)); n < NomGuildeMin || n > NomGuildeMax {
		return refusG(400, VerdictGuildeNom)
	}
	if len([]rune(description)) > DescriptionGuildeMax {
		return refusG(400, VerdictGuildeDescription)
	}
	v, err := lire(d)
	if err != nil {
		return panne(err)
	}
	if v.membreDe(uid) != nil {
		return refusG(400, VerdictGuildeDejaMembre)
	}
	age, err := d.AgeDe(uid)
	if err != nil {
		return panne(err)
	}
	if age < v.reglages.AgeMinCreation {
		return refusG(403, verdictAgeGuilde(v.reglages.AgeMinCreation, age))
	}
	for _, g := range v.guildes {
		if strings.EqualFold(g.Nom, nom) {
			return refusG(400, VerdictGuildeNomPris)
		}
	}
	pseudo, _, err := d.NomJoueur(uid)
	if err != nil {
		return panne(err)
	}
	g := &Guilde{Nom: nom, Description: description, Chef: uid}
	if err := d.CreerGuilde(g); err != nil {
		return panne(err)
	}
	if err := d.AjouterMembre(&Membre{Guilde: g.Id, Joueur: uid, Nom: pseudo, Role: RoleChef}); err != nil {
		return panne(err)
	}
	if err := effacerDemandesDe(d, v, uid); err != nil {
		return panne(err)
	}
	return okG(map[string]any{"guilde": g, "verdict": "Guilde fondee."})
}

func effacerDemandesDe(d DepotGuildes, v *vue, uid string) error {
	for _, x := range v.demandes {
		if x.Joueur == uid {
			if err := d.SupprimerDemande(x.Id); err != nil {
				return err
			}
		}
	}
	return nil
}

// ─── Demander / inviter ─────────────────────────────────────────────────────

func GuildeDemander(d DepotGuildes, uid, guildeId string) Reponse {
	if uid == "" {
		return refusG(401, VerdictGuildeConnexion)
	}
	v, err := lire(d)
	if err != nil {
		return panne(err)
	}
	if v.membreDe(uid) != nil {
		return refusG(400, VerdictGuildeDejaMembre)
	}
	g := v.guilde(guildeId)
	if g == nil {
		return refusG(404, VerdictGuildeIntrouvable)
	}
	if v.compter(g.Id, "") >= v.reglages.MembresMax {
		return refusG(400, VerdictGuildePleine)
	}
	if v.lien(g.Id, uid) != nil {
		return refusG(400, VerdictGuildeDejaDemande)
	}
	n := 0
	for _, x := range v.demandes {
		if x.Joueur == uid && x.Sens == SensDemande {
			n++
		}
	}
	if n >= DemandesJoueurMax {
		return refusG(400, VerdictGuildeTropDemandes)
	}
	pseudo, _, err := d.NomJoueur(uid)
	if err != nil {
		return panne(err)
	}
	dm := &DemandeGuilde{Guilde: g.Id, GuildeNom: g.Nom, Joueur: uid, JoueurNom: pseudo, Sens: SensDemande}
	if err := d.CreerDemande(dm); err != nil {
		return panne(err)
	}
	return okG(map[string]any{"demande": dm, "verdict": "Demande envoyee."})
}

func GuildeInviter(d DepotGuildes, uid, joueur string) Reponse {
	if uid == "" {
		return refusG(401, VerdictGuildeConnexion)
	}
	if joueur == uid {
		return refusG(400, VerdictGuildeSoi)
	}
	v, err := lire(d)
	if err != nil {
		return panne(err)
	}
	moi := v.membreDe(uid)
	if moi == nil {
		return refusG(400, VerdictGuildeAucune)
	}
	if !gere(moi.Role) {
		return refusG(403, VerdictGuildeDroit)
	}
	pseudo, existe, err := d.NomJoueur(joueur)
	if err != nil {
		return panne(err)
	}
	if !existe {
		return refusG(404, VerdictGuildeJoueur)
	}
	if v.membreDe(joueur) != nil {
		return refusG(400, VerdictGuildeJoueurPris)
	}
	g := v.guilde(moi.Guilde)
	if g == nil {
		return refusG(404, VerdictGuildeIntrouvable)
	}
	if v.compter(g.Id, "") >= v.reglages.MembresMax {
		return refusG(400, VerdictGuildePleine)
	}
	if v.lien(g.Id, joueur) != nil {
		return refusG(400, VerdictGuildeDejaDemande)
	}
	// ⚠️ Borne des invitations en cours : pas plus que de places libres, pour
	// qu'une guilde ne puisse pas arroser tout le jeu.
	enCours := 0
	for _, x := range v.demandes {
		if x.Guilde == g.Id && x.Sens == SensInvitation {
			enCours++
		}
	}
	if enCours >= v.reglages.MembresMax {
		return refusG(400, "Trop d'invitations en cours pour cette guilde.")
	}
	dm := &DemandeGuilde{Guilde: g.Id, GuildeNom: g.Nom, Joueur: joueur, JoueurNom: pseudo, Sens: SensInvitation}
	if err := d.CreerDemande(dm); err != nil {
		return panne(err)
	}
	return okG(map[string]any{"demande": dm, "verdict": "Invitation envoyee."})
}

// GuildeRepondre : accepter ou refuser une demande / invitation.
//
//	demande     → le chef ou un officier de la guilde accepte ou refuse ;
//	              le joueur peut l'ANNULER (refuser).
//	invitation  → le joueur accepte ou refuse ;
//	              le chef ou un officier peut l'ANNULER (refuser).
func GuildeRepondre(d DepotGuildes, uid, demandeId string, accepter bool) Reponse {
	if uid == "" {
		return refusG(401, VerdictGuildeConnexion)
	}
	v, err := lire(d)
	if err != nil {
		return panne(err)
	}
	dm := v.demande(demandeId)
	if dm == nil {
		return refusG(404, VerdictGuildeDemande)
	}
	moi := v.membreDe(uid)
	gerant := moi != nil && moi.Guilde == dm.Guilde && gere(moi.Role)
	concerne := dm.Joueur == uid

	var peut bool
	switch {
	case accepter && dm.Sens == SensDemande:
		peut = gerant
	case accepter && dm.Sens == SensInvitation:
		peut = concerne
	default:
		peut = gerant || concerne
	}
	if !peut {
		return refusG(403, VerdictGuildeDroit)
	}
	if !accepter {
		if err := d.SupprimerDemande(dm.Id); err != nil {
			return panne(err)
		}
		return okG(map[string]any{"verdict": "Retire."})
	}

	g := v.guilde(dm.Guilde)
	if g == nil {
		return refusG(404, VerdictGuildeIntrouvable)
	}
	if v.membreDe(dm.Joueur) != nil {
		// Entre-temps, il a rejoint une autre guilde : la demande n'a plus d'objet.
		_ = d.SupprimerDemande(dm.Id)
		return refusG(400, VerdictGuildeJoueurPris)
	}
	if v.compter(g.Id, "") >= v.reglages.MembresMax {
		return refusG(400, VerdictGuildePleine)
	}
	if err := d.AjouterMembre(&Membre{Guilde: g.Id, Joueur: dm.Joueur, Nom: dm.JoueurNom, Role: RoleMembre}); err != nil {
		return panne(err)
	}
	if err := effacerDemandesDe(d, v, dm.Joueur); err != nil {
		return panne(err)
	}
	return okG(map[string]any{"verdict": dm.JoueurNom + " rejoint " + g.Nom + "."})
}

// ─── Quitter / exclure / roles ──────────────────────────────────────────────

func GuildeQuitter(d DepotGuildes, uid string) Reponse {
	if uid == "" {
		return refusG(401, VerdictGuildeConnexion)
	}
	v, err := lire(d)
	if err != nil {
		return panne(err)
	}
	moi := v.membreDe(uid)
	if moi == nil {
		return refusG(400, VerdictGuildeAucune)
	}
	if moi.Role == RoleChef {
		return refusG(400, VerdictGuildeChefQuitte)
	}
	if err := d.RetirerMembre(moi.Id); err != nil {
		return panne(err)
	}
	return okG(map[string]any{"verdict": "Tu as quitte la guilde."})
}

func GuildeExclure(d DepotGuildes, uid, joueur string) Reponse {
	if uid == "" {
		return refusG(401, VerdictGuildeConnexion)
	}
	if joueur == uid {
		return refusG(400, VerdictGuildeSoi)
	}
	v, err := lire(d)
	if err != nil {
		return panne(err)
	}
	moi := v.membreDe(uid)
	if moi == nil {
		return refusG(400, VerdictGuildeAucune)
	}
	cible := v.membreDe(joueur)
	if cible == nil || cible.Guilde != moi.Guilde {
		return refusG(404, VerdictGuildePasMembre)
	}
	// Le chef exclut tout le monde ; un officier, seulement un membre.
	if !(moi.Role == RoleChef || (moi.Role == RoleOfficier && cible.Role == RoleMembre)) {
		return refusG(403, VerdictGuildeDroit)
	}
	if err := d.RetirerMembre(cible.Id); err != nil {
		return panne(err)
	}
	return okG(map[string]any{"verdict": cible.Nom + " a ete exclu."})
}

// GuildeRole : le chef nomme officier, redescend membre, ou TRANSMET la guilde
// (role « chef ») — l'ancien chef devient alors officier s'il reste une place,
// membre sinon.
func GuildeRole(d DepotGuildes, uid, joueur, role string) Reponse {
	if uid == "" {
		return refusG(401, VerdictGuildeConnexion)
	}
	if role != RoleChef && role != RoleOfficier && role != RoleMembre {
		return refusG(400, VerdictGuildeRole)
	}
	if joueur == uid {
		return refusG(400, VerdictGuildeSoi)
	}
	v, err := lire(d)
	if err != nil {
		return panne(err)
	}
	moi := v.membreDe(uid)
	if moi == nil {
		return refusG(400, VerdictGuildeAucune)
	}
	if moi.Role != RoleChef {
		return refusG(403, VerdictGuildeDroit)
	}
	cible := v.membreDe(joueur)
	if cible == nil || cible.Guilde != moi.Guilde {
		return refusG(404, VerdictGuildePasMembre)
	}
	g := v.guilde(moi.Guilde)
	if g == nil {
		return refusG(404, VerdictGuildeIntrouvable)
	}
	officiers := v.compter(g.Id, RoleOfficier)

	switch role {
	case RoleOfficier:
		if cible.Role == RoleOfficier {
			return okG(map[string]any{"verdict": "Deja officier."})
		}
		if officiers >= v.reglages.OfficiersMax {
			return refusG(400, VerdictGuildeOfficiers)
		}
	case RoleChef:
		// La cible quitte peut-etre un poste d'officier : il se libere.
		if cible.Role == RoleOfficier {
			officiers--
		}
		ancien := *moi
		ancien.Role = RoleMembre
		if officiers < v.reglages.OfficiersMax {
			ancien.Role = RoleOfficier
		}
		if err := d.SauverMembre(ancien); err != nil {
			return panne(err)
		}
		g.Chef = cible.Joueur
		if err := d.SauverGuilde(*g); err != nil {
			return panne(err)
		}
	}
	c := *cible
	c.Role = role
	if err := d.SauverMembre(c); err != nil {
		return panne(err)
	}
	return okG(map[string]any{"verdict": c.Nom + " est maintenant " + role + "."})
}

func GuildeModifier(d DepotGuildes, uid, description string) Reponse {
	if uid == "" {
		return refusG(401, VerdictGuildeConnexion)
	}
	description = NettoyerTexte(description)
	if len([]rune(description)) > DescriptionGuildeMax {
		return refusG(400, VerdictGuildeDescription)
	}
	v, err := lire(d)
	if err != nil {
		return panne(err)
	}
	moi := v.membreDe(uid)
	if moi == nil {
		return refusG(400, VerdictGuildeAucune)
	}
	if moi.Role != RoleChef {
		return refusG(403, VerdictGuildeDroit)
	}
	g := v.guilde(moi.Guilde)
	if g == nil {
		return refusG(404, VerdictGuildeIntrouvable)
	}
	g.Description = description
	if err := d.SauverGuilde(*g); err != nil {
		return panne(err)
	}
	return okG(map[string]any{"verdict": "Description enregistree."})
}

func GuildeDissoudre(d DepotGuildes, uid string) Reponse {
	if uid == "" {
		return refusG(401, VerdictGuildeConnexion)
	}
	v, err := lire(d)
	if err != nil {
		return panne(err)
	}
	moi := v.membreDe(uid)
	if moi == nil {
		return refusG(400, VerdictGuildeAucune)
	}
	if moi.Role != RoleChef {
		return refusG(403, VerdictGuildeDroit)
	}
	if err := d.SupprimerGuilde(moi.Guilde); err != nil {
		return panne(err)
	}
	return okG(map[string]any{"verdict": "Guilde dissoute."})
}

// ─── Salon ──────────────────────────────────────────────────────────────────

// SalonLire : les messages du salon de SA guilde. Un non-membre recoit 403 —
// c'est CE refus qui rend le salon prive, pas le site.
func SalonLire(d DepotGuildes, uid, apres string) Reponse {
	if uid == "" {
		return refusG(401, VerdictGuildeConnexion)
	}
	v, err := lire(d)
	if err != nil {
		return panne(err)
	}
	moi := v.membreDe(uid)
	if moi == nil {
		return refusG(403, VerdictGuildeAucune)
	}
	msgs, err := d.MessagesSalon(moi.Guilde, apres, SalonLimite)
	if err != nil {
		return panne(err)
	}
	if msgs == nil {
		msgs = []MessageGuilde{}
	}
	dernier, err := d.InstantDernierMessage(moi.Guilde, uid)
	if err != nil {
		return panne(err)
	}
	return Reponse{Code: 200, Corps: map[string]any{
		"ok": true, "messages": msgs, "guilde": v.guilde(moi.Guilde),
		"delai_salon_s": v.reglages.DelaiSalon,
		"attente":       AttenteSalon(dernier, d.Maintenant(), v.reglages.DelaiSalon),
	}}
}

// AttenteSalon : les secondes avant le prochain message permis (0 = libre).
func AttenteSalon(dernier, maintenant, delai int) int {
	if dernier <= 0 || delai <= 0 {
		return 0
	}
	if r := dernier + delai - maintenant; r > 0 {
		return r
	}
	return 0
}

func SalonEcrire(d DepotGuildes, uid, contenu string) Reponse {
	if uid == "" {
		return refusG(401, VerdictGuildeConnexion)
	}
	contenu = NettoyerTexte(contenu)
	if contenu == "" {
		return refusG(400, VerdictGuildeVide)
	}
	if len([]rune(contenu)) > MessageGuildeMax {
		return refusG(400, VerdictGuildeLong)
	}
	v, err := lire(d)
	if err != nil {
		return panne(err)
	}
	moi := v.membreDe(uid)
	if moi == nil {
		return refusG(403, VerdictGuildeAucune)
	}
	dernier, err := d.InstantDernierMessage(moi.Guilde, uid)
	if err != nil {
		return panne(err)
	}
	if s := AttenteSalon(dernier, d.Maintenant(), v.reglages.DelaiSalon); s > 0 {
		return erreur(429, verdictSalonAttente(s), map[string]any{"attente": s})
	}
	m := &MessageGuilde{Guilde: moi.Guilde, Auteur: uid, AuteurNom: moi.Nom, Contenu: contenu}
	if err := d.CreerMessage(m); err != nil {
		return panne(err)
	}
	return okG(map[string]any{"message": m, "attente": v.reglages.DelaiSalon})
}
