package routes

// ============================================================================
//  conception.go — CE QU'UN JOUEUR A LE DROIT DE CONCEVOIR (15/09).
//
//  Demande de Guillaume : « dans l'interface joueur, permettre de concevoir les
//  tuiles et le modele de plateaux qui leur est propre ». Ses reponses :
//    · il MODIFIE ses deux modeles (ground, space), il n'en cree ni n'en
//      supprime — le serveur les fabrique avec la planete ;
//    · il cree SES ressources et SES technos, et ses tuiles n'utilisent QUE
//      les siennes (le catalogue de sa planete, `catalogue_planete.go`) ;
//    · un onglet admin « Limites » borne, joueur par joueur ou pour tous, la
//      taille de ses plateaux et le nombre de tuiles / ressources / technos
//      qu'il peut creer ;
//    · les tileId passent sur deux octets et sont ATTRIBUES PAR LE SERVEUR.
//
//  ⚠️ CE FICHIER DECIDE, `pb/crochet_conception.go` LIT LA BASE ET APPELLE.
//  Meme frontiere que partout : rien de PocketBase ici, tout se teste.
//
//  ⚠️ L'ADMIN ET LE SUPERUSER NE PASSENT PAS PAR ICI (sauf l'attribution du
//  tileId) : ils rangent le catalogue du jeu. Les regles d'API disent la meme
//  chose ; le crochet est le second verrou, celui qui sait compter.
// ============================================================================

import (
	"fmt"

	"sysb/moteur"
)

// Limites : ce qu'un joueur peut concevoir. 0 = rien.
type Limites struct {
	LargeurMax, HauteurMax               int
	TuilesMax, RessourcesMax, TechnosMax int
	// D'ou elles viennent : "joueur", "general" ou "" (aucune fiche).
	Source string
}

// LimitesDe — LA REGLE DES FICHES `limites`.
//
//  1. les fiches qui NOMMENT le joueur (`joueurs`) passent en premier ;
//  2. sinon, les fiches GENERALES (`general = true`) ;
//  3. sinon, rien : 0 partout, il ne cree rien.
//
// ⚠️ PLUSIEURS FICHES DU MEME RANG : on prend LA PLUS LARGE, champ par champ.
// C'est le seul choix qui ne retire rien a quelqu'un parce qu'une seconde fiche
// le cite — ajouter un joueur a une fiche ne peut que l'ouvrir.
//
// ⚠️ UNE FICHE PERSONNELLE REMPLACE LA GENERALE EN ENTIER, meme plus petite :
// c'est ce qui permet de brider un joueur sous la regle commune.
func LimitesDe(uid string, fiches []moteur.Enregistrement) Limites {
	var perso, generales []moteur.Enregistrement
	for _, f := range fiches {
		nomme := false
		for _, id := range listeTextes(moteur.Champ(f, "joueurs")) {
			if id == uid && uid != "" {
				nomme = true
			}
		}
		if nomme {
			perso = append(perso, f)
		} else if moteur.Champ(f, "general") == true {
			generales = append(generales, f)
		}
	}
	choix, source := perso, "joueur"
	if len(choix) == 0 {
		choix, source = generales, "general"
	}
	if len(choix) == 0 {
		return Limites{}
	}
	l := Limites{Source: source}
	for _, f := range choix {
		l.LargeurMax = max(l.LargeurMax, positif(moteur.Champ(f, "largeur_max")))
		l.HauteurMax = max(l.HauteurMax, positif(moteur.Champ(f, "hauteur_max")))
		l.TuilesMax = max(l.TuilesMax, positif(moteur.Champ(f, "tuiles_max")))
		l.RessourcesMax = max(l.RessourcesMax, positif(moteur.Champ(f, "ressources_max")))
		l.TechnosMax = max(l.TechnosMax, positif(moteur.Champ(f, "technos_max")))
	}
	return l
}

func positif(v any) int {
	n := moteur.Entier(v, 0)
	if n < 0 {
		return 0
	}
	return n
}

// listeTextes : une relation multiple, quelle que soit la forme rendue.
//
// ⚠️ UNE RELATION A `maxSelect` 0 OU 1 ARRIVE EN TEXTE (panne du 15/09 sur
// `planetes_autorisees`) : on accepte les deux formes plutot que de lire
// « personne » en silence.
func listeTextes(v any) []string {
	switch x := v.(type) {
	case []string:
		return x
	case []any:
		out := make([]string, 0, len(x))
		for _, e := range x {
			if s := moteur.Texte(e); s != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		if x != "" {
			return []string{x}
		}
	}
	return nil
}

// QuotaDe : la limite qui s'applique a une collection.
func (l Limites) QuotaDe(collection string) int {
	switch collection {
	case "tuiles":
		return l.TuilesMax
	case "ressources":
		return l.RessourcesMax
	case "technologies":
		return l.TechnosMax
	}
	return 0
}

var motDe = map[string]string{"tuiles": "tuile", "ressources": "ressource", "technologies": "technologie"}

// RefusPlanete — un joueur n'ecrit QUE sur sa planete, et n'en demenage rien.
//
// `planeteAvant`/`propAvant` : la planete du record en base (vide a la
// creation). `planeteApres`/`propApres` : celle qu'il aura apres l'ecriture
// (vide a la suppression). `propX` = le proprietaire de la planete, "" pour une
// planete game, "?" pour une planete introuvable.
func RefusPlanete(action, uid, planeteAvant, propAvant, planeteApres, propApres string) string {
	switch action {
	case "create":
		if planeteApres == "" {
			return "Choisis ta planete : un joueur ne cree que sur la sienne."
		}
		if propApres != uid {
			return "Tu ne peux creer que sur TA planete."
		}
	case "update":
		if propAvant != uid {
			return "Ce n'est pas un element de ta planete : tu ne peux pas le modifier."
		}
		if planeteApres != planeteAvant {
			return "Un element ne change pas de planete."
		}
	case "delete":
		if propAvant != uid {
			return "Ce n'est pas un element de ta planete : tu ne peux pas le supprimer."
		}
	default:
		return "Action inconnue."
	}
	return ""
}

// RefusQuota — la creation qui depasserait la limite.
//
// `deja` = ce que sa planete porte deja dans cette collection.
func RefusQuota(collection string, deja int, l Limites) string {
	max := l.QuotaDe(collection)
	mot := motDe[collection]
	if max <= 0 {
		return fmt.Sprintf("L'administrateur ne t'a ouvert aucune %s a creer (onglet Limites).", mot)
	}
	if deja >= max {
		return fmt.Sprintf("Limite atteinte : %d %s(s) sur %d. Supprimes-en une, ou demande a l'administrateur.",
			deja, mot, max)
	}
	return ""
}

// RefusModele — ce qu'un joueur change dans SON modele de plateau.
//
// ⚠️ LA TAILLE : CHAQUE COTE NE DEPASSE PAS LE PLUS GRAND DE (sa valeur
// actuelle, la limite). Le serveur fabrique les modeles neufs en 60 x 60 : un
// joueur dont la limite est 30 doit pouvoir continuer a peindre le sien, et le
// REDUIRE un cote a la fois — juger les deux cotes contre la limite l'aurait
// bloque a 60 de haut tant qu'il n'a pas aussi retaille la largeur.
//
// ⚠️ LA GRILLE : chaque case porte 0 ou une tuile DE SA PLANETE. Sans ce
// controle, il peindrait les batiments du jeu sur son modele — et son plateau
// neuf les recevrait gratuitement.
func RefusModele(lAvant, hAvant, l, h int, lim Limites, grille string, permis map[int]bool) string {
	if l < 1 || h < 1 {
		return "Un plateau a au moins une case de large et de haut."
	}
	if l > max(lAvant, lim.LargeurMax) || h > max(hAvant, lim.HauteurMax) {
		return fmt.Sprintf("Taille refusee : %d x %d, ta limite est %d x %d (onglet Limites de l'administrateur).",
			l, h, lim.LargeurMax, lim.HauteurMax)
	}
	cases, ok := moteur.LireGrille(grille, l, h)
	if !ok {
		return fmt.Sprintf("La grille ne correspond pas a %d x %d cases.", l, h)
	}
	for i, c := range cases {
		if c != 0 && !permis[c] {
			return fmt.Sprintf("La case %d porte la tuile %d, qui n'est pas une tuile de ta planete.", i, c)
		}
	}
	return ""
}

// ProchainTileId — ⚠️ MAX + 1, JAMAIS UN TROU RECYCLE (13/09) : un plateau peut
// encore porter l'id d'une tuile supprimee, et il changerait de batiment.
func ProchainTileId(plusGrand int) (int, string) {
	if plusGrand < 0 {
		plusGrand = 0
	}
	n := plusGrand + 1
	if !moteur.TileIdValide(n) {
		return 0, fmt.Sprintf("Plus aucun numero de tuile libre (%d atteint).", moteur.TileIdMax)
	}
	return n, ""
}

// IconeLue : ce qu'une fiche `icones` dit d'elle-meme.
type IconeLue struct {
	Chemin, Usage string
	Partage       Partageable
}

// UsageIconeDe : la categorie d'icone qu'une collection concue attend.
var UsageIconeDe = map[string]string{"tuiles": "tuile", "ressources": "ressource", "technologies": "techno"}

// VignetteDeJoueur — la vignette d'un element de JOUEUR : le chemin a ecrire,
// ou le refus.
//
// ⚠️ `chemin_icone` EST CE QUE LIT UNITY, et c'est un TEXTE : si le joueur
// l'ecrivait, il y taperait le chemin de n'importe quelle icone du jeu et le
// partage des icones ne servirait a rien. Le serveur le RECOPIE donc de
// l'icone choisie (`icone`) — vide sans icone — et verifie que cette icone est
// de la bonne categorie et ouverte a sa planete.
//
// `icone` nil = aucune relation posee. `iconeDemandee` = l'id envoye (pour
// distinguer « aucune » d'une relation cassee).
func VignetteDeJoueur(collection, iconeDemandee string, icone *IconeLue, planeteId, proprietaire string) (chemin, refus string) {
	if iconeDemandee == "" {
		return "", ""
	}
	if icone == nil {
		return "", "Cette icone n'existe pas."
	}
	if attendu := UsageIconeDe[collection]; attendu != "" && icone.Usage != attendu {
		return "", fmt.Sprintf("Cette icone est rangee en « %s », pas en « %s ».", icone.Usage, attendu)
	}
	if !AutoriseeSur(icone.Partage, planeteId, proprietaire) {
		return "", "Cette icone n'est pas ouverte a ta planete. C'est l'administrateur qui la partage."
	}
	return icone.Chemin, ""
}
