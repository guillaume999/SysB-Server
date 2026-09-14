package routes

// ============================================================================
//  maplanete.go — POST /api/sysb/ma-planete
//
//  LA PLANETE D'UN JOUEUR SE CREE PAR UN GESTE EXPLICITE DE SA PART : il la
//  nomme, il choisit son apparence. Ce n'est PAS fait a l'inscription, et c'est
//  un choix : une planete creee d'office serait, jusqu'a ce que l'admin lui
//  ouvre quelque chose, une planete sans nom, sans apparence et INJOUABLE — le
//  joueur la trouverait vide sans savoir pourquoi.
//
//  CE QU'ELLE FAIT, EN UNE FOIS :
//    1. la planete (proprietaire = lui) ;
//    2. SES DEUX MODELES DE PLATEAU, vides, rattaches a elle et partages a lui.
//
//  ⚠️⚠️ LES DEUX ENSEMBLE, ET C'EST TOUT L'INTERET DE LA ROUTE. `templates` est
//  en creation ADMIN dans les regles d'API — exprès. C'est donc le serveur
//  (superuser) qui les pose. Desserrer la regle pour ce cas la desserrerait
//  pour tous, et un joueur pourrait creer un modele chez quelqu'un d'autre.
//
//  ⚠️ UNE SEULE PLANETE PERSO PAR JOUEUR. L'index unique de `planetes` ne dit
//  PAS ca (il ne porte que le nom) : la regle se tient ICI, et nulle part
//  ailleurs. Un ecran qui cacherait le bouton ne serait pas une regle.
//
//  ⚠️ CE QU'ELLE NE FAIT PAS : ouvrir des modeles 3D ou des icones a cette
//  planete. C'est l'administrateur, et lui seul. Une planete neuve ne peut donc
//  porter AUCUNE tuile tant qu'un socle ne lui est pas ouvert — la reponse le
//  DIT, plutot que de laisser le joueur devant un editeur sans pinceau.
// ============================================================================

import (
	"fmt"
	"strings"

	"sysb/moteur"
)

// DepotMaPlanete : ce que cette route demande en plus du `Depot` commun.
//
// ⚠️ DEUX CREATEURS, PAS DEUX DE PLUS SUR `Depot` : `etat`, `passe` et `geste`
// n'ont aucune raison de savoir fabriquer une planete.
type DepotMaPlanete interface {
	Depot
	NouvellePlanete() (moteur.Enregistrement, error)
	NouveauTemplate() (moteur.Enregistrement, error)
}

// Les surfaces qu'une planete neuve recoit.
//
// ⚠️ LES DEUX, PARCE QU'EarthScene OUVRE LES DEUX. N'en creer qu'une ferait un
// 404 a l'ouverture, sur la surface manquante, et le joueur chercherait une
// panne la ou il manque une saisie.
var TypesDeDepart = []string{"ground", "space"}

// La taille d'un plateau neuf.
//
// ⚠️ ELLE EST ECRITE ICI ET NULLE PART AILLEURS. Un modele sans dimensions se
// fait refuser par `assurer` (422) : ce n'est pas un detail d'affichage, c'est
// ce qui rend la planete jouable. 60 x 60 = 3 600 cases, soit 4 800 caracteres
// de base64 — tres loin du plafond de 100 000 du champ.
const (
	LargeurDeDepart = 60
	HauteurDeDepart = 60
)

// NomDeModeleDeDepart — le libelle des deux modeles crees avec la planete.
func NomDeModeleDeDepart(nomPlanete, typeDePlateau string) string {
	return fmt.Sprintf("%s — %s", nomPlanete, typeDePlateau)
}

// MaPlanete : le joueur cree SA planete.
func MaPlanete(d DepotMaPlanete, uid, nom, modele3d, icone string) Reponse {
	nom = strings.TrimSpace(nom)
	if nom == "" {
		return erreur(400, "Donne un nom a ta planete : c'est lui qu'on lira sur sa fiche, et "+
			"c'est par lui qu'on la cherchera.", map[string]any{"cree": false})
	}
	if len(nom) > 100 {
		return erreur(400, "Ce nom est trop long (100 caracteres au plus).",
			map[string]any{"cree": false})
	}

	planetes, err := d.Planetes()
	if err != nil {
		return erreur(500, "lecture des planetes : "+err.Error(), map[string]any{"cree": false})
	}

	for _, p := range planetes {
		// ⚠️ LA REGLE « UNE SEULE », TENUE PAR LE SERVEUR. L'index ne la dit pas.
		if moteur.Texte(moteur.Champ(p, "proprietaire")) == uid {
			return erreur(409, fmt.Sprintf("Tu as deja ta planete : « %s ». On n'en a qu'une.",
				moteur.Texte(moteur.Champ(p, "nom"))),
				map[string]any{"cree": false, "planete": moteur.Texte(moteur.Champ(p, "id"))})
		}
		// ⚠️ COMPARAISON STRICTE, LA CASSE COMPRISE — comme partout. L'index
		// unique de `planetes.nom` refuserait de toute facon, mais avec un
		// message de base de donnees ; celui-ci se lit.
		if moteur.Texte(moteur.Champ(p, "nom")) == nom {
			return erreur(409, fmt.Sprintf("Une planete s'appelle deja « %s ». Trouve-lui un "+
				"autre nom : c'est par le nom qu'on la cherche.", nom),
				map[string]any{"cree": false})
		}
	}

	// ─── La planete ─────────────────────────────────────────────────────────
	rec, err := d.NouvellePlanete()
	if err != nil || rec == nil {
		return erreur(500, "creation de la planete impossible", map[string]any{"cree": false})
	}
	rec.Set("nom", nom)
	rec.Set("proprietaire", uid)
	if modele3d != "" {
		rec.Set("modele3d", modele3d)
	}
	if icone != "" {
		rec.Set("icone", icone)
	}
	if err := d.Sauver(rec); err != nil {
		return erreur(500, "ecriture de la planete : "+err.Error(), map[string]any{"cree": false})
	}
	planeteId := moteur.Texte(moteur.Champ(rec, "id"))

	// ─── Ses deux modeles, vides ────────────────────────────────────────────
	//
	// ⚠️ UNE GRILLE DE ZEROS EST UNE GRILLE VALIDE : `0` est la case vide, et le
	// joueur peindra par-dessus. Ce qui ne serait PAS valide, c'est une grille
	// dont la longueur ne fait pas largeur x hauteur — `assurer` le refuse, et
	// c'est pour ca qu'on la fabrique ici plutot que de laisser le champ vide.
	vide := moteur.OctetsVersBase64(make([]int, LargeurDeDepart*HauteurDeDepart))
	crees := []any{}
	for _, t := range TypesDeDepart {
		tpl, err := d.NouveauTemplate()
		if err != nil || tpl == nil {
			return erreur(500, "creation d'un modele impossible", map[string]any{"cree": false})
		}
		tpl.Set("nom", NomDeModeleDeDepart(nom, t))
		tpl.Set("typeOfPlateau", t)
		tpl.Set("planete", planeteId)
		// ⚠️ L'ETIQUETTE TEXTE EN PLUS, le temps de la bascule : le site et Unity
		// d'avant lisent encore `typeOfPlateau2`. Elle part avec le menage.
		tpl.Set("typeOfPlateau2", nom)
		tpl.Set("largeur", LargeurDeDepart)
		tpl.Set("hauteur", HauteurDeDepart)
		tpl.Set("tilesBase64", vide)
		tpl.Set("etats", []any{})
		// ⚠️ C'EST `partages` QUI LUI DONNE LE CRAYON : la regle d'API d'`update`
		// sur `templates` passe si le joueur y figure. Sans cette ligne il
		// aurait une planete qu'il ne pourrait pas dessiner.
		tpl.Set("partages", []string{uid})
		tpl.Set("actif", true)
		if err := d.Sauver(tpl); err != nil {
			return erreur(500, "ecriture d'un modele : "+err.Error(),
				map[string]any{"cree": false, "planete": planeteId})
		}
		crees = append(crees, map[string]any{
			"id": moteur.Texte(moteur.Champ(tpl, "id")), "typeOfPlateau": t,
		})
	}

	return Reponse{200, map[string]any{
		"ok": true, "ecrit": true, "cree": true, "joueur": uid,
		"planete": map[string]any{
			"id": planeteId, "nom": nom, "proprietaire": uid,
			"modele3d": modele3d, "icone": icone,
		},
		"modeles": crees,
		// ⚠️ LA PHRASE QUI EVITE L'ECRAN MUET. Une planete neuve n'a aucune
		// tuile, et son editeur de plateaux n'aura donc AUCUN pinceau. Le dire
		// ici, au moment de la creation, vaut mieux que de le laisser decouvrir.
		"verdict": fmt.Sprintf("Planete « %s » creee, avec ses deux modeles vides. "+
			"Prochaine etape : cree tes tuiles — il te faut un modele 3D ET une icone "+
			"ouverts a ta planete, sinon l'editeur n'aura aucun pinceau.", nom),
	}}
}
