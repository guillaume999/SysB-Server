package routes

// ============================================================================
//  planetejoueur.go — LA PLANETE D'UN JOUEUR, CREEE D'OFFICE (15/09).
//
//  Decision du 15/09 : a la creation d'un compte, le joueur RECOIT sa planete.
//  Elle porte son pseudo et comprend deux modeles de plateau, `ground` et
//  `space`. Il n'y a plus de geste « creer ma planete » : la route
//  `POST /api/sysb/ma-planete` et l'ecran du site sont retires.
//
//  QUI APPELLE :
//    · le crochet `users` apres creation (pb/planete_joueur.go) ;
//    · le rattrapage au demarrage du serveur, pour les comptes d'avant.
//  Les deux passent par `AssurerPlaneteDe`, qui est IDEMPOTENTE : une planete
//  deja la n'est pas recreee, un modele qui manque est ajoute.
//
//  ⚠️⚠️ LE CHAMP `appartient` DES MODELES (15/09). Texte, deux familles :
//      "game"      → modele du jeu (Terre, Jupiter…)
//      <id joueur> → modele de la planete de ce joueur
//  ⚠️ JAMAIS VIDE sur un modele neuf. Vide ne veut PAS dire « game » : c'est
//  un modele que personne n'a range. D'ou le pseudo « game » interdit
//  (`PseudoReserve`) — sinon deux lectures du meme mot.
//
//  ⚠️ `appartient` dit a qui c'est, il ne donne AUCUN droit d'ecriture : le
//  partage de modele (`templates.partages`) a ete retire le 15/09, seul
//  l'admin modifie un modele.
//
//  ⚠️ UNE SEULE PLANETE PERSO PAR JOUEUR. L'index unique de `planetes` ne porte
//  que le nom : la regle se tient ICI.
//
//  ⚠️ CE QU'ELLE NE FAIT PAS : ouvrir des modeles 3D ou des icones a la
//  planete. C'est l'administrateur, depuis l'onglet Modeles du site.
// ============================================================================

import (
	"fmt"
	"strings"

	"sysb/moteur"
)

// AppartientGame : la valeur d'`appartient` pour un modele du jeu.
const AppartientGame = "game"

// DepotPlaneteJoueur : ce que la creation demande en plus du `Depot` commun.
type DepotPlaneteJoueur interface {
	Depot
	NouvellePlanete() (moteur.Enregistrement, error)
	NouveauTemplate() (moteur.Enregistrement, error)
	// Les modeles rattaches a une planete.
	TemplatesDe(planeteId string) ([]moteur.Enregistrement, error)
	// Les champs que `templates` retient VRAIMENT — voir `gardeChampAppartient`.
	ChampsTemplates() []string
}

// Les surfaces qu'une planete neuve recoit.
//
// ⚠️ LES DEUX, PARCE QU'EarthScene OUVRE LES DEUX. N'en creer qu'une ferait un
// 404 a l'ouverture, sur la surface manquante.
var TypesDeDepart = []string{"ground", "space"}

// La taille d'un plateau neuf.
//
// ⚠️ Un modele sans dimensions se fait refuser par `assurer` (422).
// 60 x 60 = 3 600 cases, soit 4 800 caracteres de base64.
const (
	LargeurDeDepart = 60
	HauteurDeDepart = 60
)

// NomDeModeleDeDepart — le libelle des deux modeles crees avec la planete.
func NomDeModeleDeDepart(nomPlanete, typeDePlateau string) string {
	return fmt.Sprintf("%s — %s", nomPlanete, typeDePlateau)
}

// PseudoReserve — « game » (casse et espaces ignores) ne peut pas etre un
// pseudo : c'est la valeur d'`appartient` des modeles du jeu, et le nom de la
// planete qui porte le contenu commun.
func PseudoReserve(pseudo string) bool {
	return strings.EqualFold(strings.TrimSpace(pseudo), AppartientGame)
}

// VerdictPseudoReserve : la phrase du refus, la meme partout.
const VerdictPseudoReserve = "« game » est reserve au jeu : choisis un autre pseudo."

// NomDePlaneteDe — le nom que prend la planete d'un joueur : son pseudo.
//
// ⚠️ Repli sur le debut de l'email, puis sur l'id : un compte sans pseudo doit
// quand meme avoir une planete.
// ⚠️ Jamais « game », et jamais un nom deja pris (casse ignoree) : on ajoute
// « (2) », « (3) »… Refuser ferait echouer la creation en silence.
func NomDePlaneteDe(pseudo, email, uid string, pris []string) string {
	base := strings.TrimSpace(pseudo)
	if base == "" {
		base = strings.TrimSpace(strings.SplitN(email, "@", 2)[0])
	}
	if base == "" {
		base = "Planete " + uid
	}
	if len([]rune(base)) > 90 {
		base = string([]rune(base)[:90])
	}
	occupe := func(n string) bool {
		if PseudoReserve(n) {
			return true
		}
		for _, p := range pris {
			if strings.EqualFold(strings.TrimSpace(p), n) {
				return true
			}
		}
		return false
	}
	if !occupe(base) {
		return base
	}
	for i := 2; ; i++ {
		n := fmt.Sprintf("%s (%d)", base, i)
		if !occupe(n) {
			return n
		}
	}
}

// gardeChampAppartient — ⚠️ LE PIEGE DU CHAMP `t`, TROISIEME EDITION.
// `Record.Set` sur un champ absent garde la valeur en memoire et la perd au
// SAVE : sans ce controle, les modeles partiraient sans proprietaire, sous un
// 200. On refuse bruyamment en nommant le patch.
func gardeChampAppartient(d DepotPlaneteJoueur) *Reponse {
	for _, n := range d.ChampsTemplates() {
		if n == "appartient" {
			return nil
		}
	}
	r := erreur(500, "SCHEMA : la collection `templates` n'a pas de champ `appartient`. "+
		"Lance `patch-appartient-2026-09-15.js` avant de creer des planetes de joueur.",
		map[string]any{"cree": false})
	return &r
}

// AssurerPlaneteDe : le joueur a-t-il sa planete et ses deux modeles ? Sinon,
// on les cree. Rien n'est ecrit si tout est deja la.
func AssurerPlaneteDe(d DepotPlaneteJoueur, uid string) Reponse {
	if uid == "" {
		return erreur(400, "joueur non nomme", map[string]any{"cree": false})
	}
	if r := gardeChampAppartient(d); r != nil {
		return *r
	}
	u, err := d.Utilisateur(uid)
	if err != nil || u == nil {
		return erreur(404, "compte introuvable : "+uid, map[string]any{"cree": false})
	}

	planetes, err := d.Planetes()
	if err != nil {
		return erreur(500, "lecture des planetes : "+err.Error(), map[string]any{"cree": false})
	}

	// ─── La planete : la sienne, ou une neuve ──────────────────────────────
	var planete moteur.Enregistrement
	pris := make([]string, 0, len(planetes))
	for _, p := range planetes {
		if planete == nil && moteur.Texte(moteur.Champ(p, "proprietaire")) == uid {
			planete = p
		}
		pris = append(pris, moteur.Texte(moteur.Champ(p, "nom")))
	}

	creePlanete := false
	if planete == nil {
		nom := NomDePlaneteDe(moteur.Texte(moteur.Champ(u, "pseudo")),
			moteur.Texte(moteur.Champ(u, "email")), uid, pris)
		rec, err := d.NouvellePlanete()
		if err != nil || rec == nil {
			return erreur(500, "creation de la planete impossible", map[string]any{"cree": false})
		}
		rec.Set("nom", nom)
		rec.Set("proprietaire", uid)
		if err := d.Sauver(rec); err != nil {
			return erreur(500, "ecriture de la planete : "+err.Error(), map[string]any{"cree": false})
		}
		planete = rec
		creePlanete = true
	}
	planeteId := moteur.Texte(moteur.Champ(planete, "id"))
	nom := moteur.Texte(moteur.Champ(planete, "nom"))

	// ─── Ses deux modeles : ceux qui manquent ───────────────────────────────
	existants, err := d.TemplatesDe(planeteId)
	if err != nil {
		return erreur(500, "lecture des modeles : "+err.Error(), map[string]any{"cree": false})
	}
	aDeja := map[string]bool{}
	for _, t := range existants {
		aDeja[moteur.Texte(moteur.Champ(t, "typeOfPlateau"))] = true
	}

	// ⚠️ UNE GRILLE DE ZEROS EST UNE GRILLE VALIDE : `0` est la case vide.
	vide := moteur.EcrireGrille(make([]int, LargeurDeDepart*HauteurDeDepart))
	crees := []any{}
	for _, t := range TypesDeDepart {
		if aDeja[t] {
			continue
		}
		tpl, err := d.NouveauTemplate()
		if err != nil || tpl == nil {
			return erreur(500, "creation d'un modele impossible", map[string]any{"cree": false})
		}
		tpl.Set("nom", NomDeModeleDeDepart(nom, t))
		tpl.Set("typeOfPlateau", t)
		tpl.Set("planete", planeteId)
		tpl.Set("appartient", uid)
		// ⚠️ L'etiquette texte en plus : le pinceau du site filtre encore dessus.
		tpl.Set("typeOfPlateau2", nom)
		tpl.Set("largeur", LargeurDeDepart)
		tpl.Set("hauteur", HauteurDeDepart)
		tpl.Set("tilesBase64", vide)
		tpl.Set("etats", []any{})
		tpl.Set("actif", true)
		if err := d.Sauver(tpl); err != nil {
			return erreur(500, "ecriture d'un modele : "+err.Error(),
				map[string]any{"cree": false, "planete": planeteId})
		}
		crees = append(crees, map[string]any{
			"id": moteur.Texte(moteur.Champ(tpl, "id")), "typeOfPlateau": t,
		})
	}

	verdict := fmt.Sprintf("Planete « %s » deja en place.", nom)
	switch {
	case creePlanete:
		verdict = fmt.Sprintf("Planete « %s » creee, avec ses deux modeles vides.", nom)
	case len(crees) > 0:
		verdict = fmt.Sprintf("Planete « %s » : %d modele(s) manquant(s) ajoute(s).", nom, len(crees))
	}
	return Reponse{200, map[string]any{
		"ok": true, "ecrit": creePlanete || len(crees) > 0,
		"cree": creePlanete, "joueur": uid,
		"planete": map[string]any{"id": planeteId, "nom": nom, "proprietaire": uid},
		"modeles": crees,
		"verdict": verdict,
	}}
}
