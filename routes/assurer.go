// ============================================================
//  routes/assurer.go — « JE N'AI PAS ENCORE DE PLATEAU ICI, FABRIQUE-LE »
//
//      POST /api/sysb/assurer   corps : { "type": "ground", "monde": "Jupiter" }
//
//  ⚠️⚠️ UN PLATEAU S'IDENTIFIE PAR DEUX ETIQUETTES DEPUIS LE 13/09, PAS UNE.
//  `type` (`typeOfPlateau` : ground / space / TPTplateau) dit SUR QUOI on joue ;
//  `monde` (`typeOfPlateau2` : Terre / Jupiter…) dit OU. Le joueur a donc une
//  colonie sur Terre ET une colonie sur Jupiter, et les deux sont des `ground`.
//  Ne comparer que le type ferait rendre la colonie TERRIENNE a qui clique sur
//  Jupiter — un 200 parfaitement tranquille, avec le mauvais plateau dedans.
//
//  ⚠️ LA COMPARAISON EST STRICTE, LE VIDE COMPRIS — la meme regle que le pinceau
//  du site (patch `type-plateau-2` du 13/09). Un `monde` vide ne se rapproche
//  QUE d'une etiquette vide : il n'y a pas de « monde par defaut » ici, parce
//  qu'un defaut voudrait dire deviner, et deviner rendrait le plateau d'ailleurs.
//  ⚠️ Consequence a connaitre : depuis que le patch `etiquette-terre` a marque
//  tout l'existant « Terre », un client qui n'envoie PAS de monde ne retrouve
//  plus rien et se fait refuser en 404. C'est voulu — il vaut mieux un refus qui
//  nomme le monde qu'un doublon fabrique en silence.
//
//  ELLE BOUCHE LE TROU OUVERT LE 04/09 ET REPORTE LE 12/09. Depuis que les
//  regles de `plateaux` sont fermees en `role='admin'`, le client ne peut plus
//  fabriquer son plateau lui-meme : c'est la SEULE porte par laquelle un compte
//  neuf en obtient un. Sans elle, « vider `plateaux` » (l'etape 1.2 du 11/09)
//  est un aller sans retour — et c'est justement ce qui a laisse les vieux
//  etats du moteur a debit trainer en base jusqu'au 13/09.
//
//  CE QU'ELLE N'EST PAS
//  --------------------
//  Ce n'est PAS un second `etat`. Elle CREE, puis dit « voila ton id ». Le
//  client enchaine sur `GET /api/sysb/etat?plateau=<id>`. Deux routes qui
//  rendraient le meme gros bloc, ce serait deux endroits ou le format diverge.
//
//  ⚠️⚠️ CE N'EST PAS UNE TRANSCRIPTION DE `assurer.pb.js` — ET C'EST LE POINT.
//  Le JS ecrivait le format d'AVANT le moteur a cycles : un `t` et un
//  `chantier` PAR ETAT, et aucun `t` sur le plateau. Le porter tel quel aurait
//  fabrique des plateaux neufs dans un format perime, c'est-a-dire exactement
//  la panne qu'on repare. Ici les etats preparés ne portent que ce qui vient du
//  MODELE (x, z, niveau, actif, stock) ; la forme §10 qui part en base est
//  ecrite par `Partie.Ecrire`, le seul ecrivain du projet.
//
//  ⚠️ PAS DE VERROU `ARMEE` : celui du JS valait `true` depuis le 09/09, et un
//  interrupteur qui n'a plus qu'une position est du code mort.
//
//  ⚠️ L'HORODATAGE EST TOUT L'INTERET DE LA FAIRE COTE SERVEUR. Les etats d'un
//  MODELE sont vieux de jours : recopier leur temps offrirait au nouveau joueur
//  toute cette production d'avance. Le plateau neuf part a l'heure du SERVEUR,
//  cycles a zero — et c'est l'heure que le client n'a pas le droit de choisir.
//
//  ⚠️ LE CHANTIER DE L'ADMIN NE SE LEGUE PAS. Un modele peut porter une case en
//  construction ; `chantier_fin` n'est simplement pas recopie.
//
//  ⚠️ ELLE N'ECRIT RIEN SI LE PLATEAU EXISTE. Le client n'appelle ici qu'apres
//  un echec, mais un appel de trop ne doit pas fabriquer un doublon. Le
//  controle se fait DANS la transaction : l'adaptateur y enferme toute la
//  route, donc deux ouvertures simultanees du jeu ne peuvent pas le franchir
//  toutes les deux.
// ============================================================

package routes

import (
	"fmt"

	"sysb/moteur"
)

// DepotCreateur : ce qu'`Assurer` demande EN PLUS du `Depot` commun.
//
// ⚠️ DEUX METHODES, PAS DEUX DE PLUS SUR `Depot` : `etat`, `passe` et `geste`
// n'ont aucune raison de savoir creer un record, et une interface qui grossit
// est une interface que les faux des essais copient sans la comprendre.
type DepotCreateur interface {
	Depot
	// Le modele d'un type SUR UNE PLANETE, dans `templates`. `nil, nil` = il
	// n'y en a pas — ce n'est pas une panne, c'est une saisie a faire sur le
	// site.
	//
	// ⚠️ LA PLANETE, PAS L'ETIQUETTE (14/09). Le filtre portait sur le texte
	// `typeOfPlateau2` ; il porte maintenant sur la relation `planete`. Ce qui
	// ne change PAS : on prend le PLUS ANCIEN du couple, donc un brouillon
	// cohabite toujours avec le modele en service — c'est simplement cadre a
	// une planete au lieu d'etre global, et c'est ca qui ferme le piege.
	//
	// ⚠️ UN MODELE NE SE CHERCHE PLUS ENTRE PLANETES : sans ce cadre, deux
	// modeles `ground` de deux joueurs se departageaient a la date de creation,
	// et le joueur debarquait chez quelqu'un d'autre sans un mot.
	ModeleDuType(typeVoulu, planeteId string) (moteur.Enregistrement, error)
	// Un record `plateaux` NEUF, pas encore ecrit. C'est `Sauver` qui l'ecrit.
	NouveauPlateau() (moteur.Enregistrement, error)
}

// NomParDefaut — ⚠️ MEME TABLE QU'EN C#. Un type inconnu ne s'appelle PAS « Ma
// colonie » : depuis le plateau TPT (29/08) il existe un troisieme type, et lui
// donner le nom du sol ferait croire a un doublon dans la liste du joueur.
//
// ⚠️ LE MONDE ENTRE DANS LE NOM POUR LA MEME RAISON, ET ELLE EST PLUS FORTE
// DEPUIS LE 13/09 : deux colonies `ground` coexistent maintenant chez le meme
// joueur, une par monde. Deux lignes « Ma colonie » dans sa liste, ce serait
// deux plateaux impossibles a departager a l'oeil.
// ⚠️ UN MONDE VIDE NE MET PAS DE PARENTHESE VIDE : les plateaux d'avant
// l'etiquetage s'appellent « Ma colonie » tout court, et ils continuent.
func NomParDefaut(typeVoulu, monde string) string {
	var base string
	switch typeVoulu {
	case "ground":
		base = "Ma colonie"
	case "space":
		base = "Ma station"
	default:
		base = fmt.Sprintf("Mon plateau (%s)", typeVoulu)
	}
	if monde == "" {
		return base
	}
	return fmt.Sprintf("%s (%s)", base, monde)
}

func resume(rec moteur.Enregistrement) map[string]any {
	return map[string]any{
		"id":            moteur.Texte(moteur.Champ(rec, "id")),
		"nom":           moteur.Texte(moteur.Champ(rec, "nom")),
		"typeOfPlateau": moteur.Texte(moteur.Champ(rec, "typeOfPlateau")),
		// ⚠️ RENDU MEME VIDE, et Unity s'en sert pour distinguer « ce plateau
		// n'a pas d'etiquette » (chaine vide) de « ce serveur ne connait pas
		// encore les mondes » (champ ABSENT, donc `null` cote client). C'est la
		// meme lecon que `refusees` le 13/09 : `[]` et `null` ne disent pas la
		// meme chose, et un client qui les confond fabrique des doublons.
		"typeOfPlateau2": moteur.Texte(moteur.Champ(rec, "typeOfPlateau2")),
		// ⚠️ RENDU MEME VIDE, pour la meme raison que l'etiquette juste au-dessus :
		// une chaine vide dit « ce plateau n'a pas encore de planete », `null`
		// dit « ce serveur ne connait pas les planetes ». Un client qui les
		// confond refabrique un plateau a chaque ouverture.
		"planete": moteur.Texte(moteur.Champ(rec, "planete")),
		"largeur": moteur.Entier(moteur.Champ(rec, "largeur"), 0),
		"hauteur": moteur.Entier(moteur.Champ(rec, "hauteur"), 0),
		"version": moteur.Entier(moteur.Champ(rec, "version"), 0),
	}
}

// gardeChampMonde — ⚠️⚠️ LE CHAMP `typeOfPlateau2` SUR `plateaux`, RELEVE A
// CHAQUE APPEL, ET SEULEMENT QUAND UN MONDE EST DEMANDE.
//
// Exactement le piege du champ `t` (13/09) : `Record.Set` sur un champ absent de
// la collection ne se plaint pas — PocketBase retombe sur `SetRaw`, garde la
// valeur dans le record, et `Get` la rend. Elle n'est perdue qu'au SAVE. Sans ce
// controle, la colonie de Jupiter serait ecrite SANS monde, la lecture suivante
// ne la reconnaitrait pas comme jupiterienne, et `Assurer` en fabriquerait une
// autre a chaque ouverture du jeu — sous des 200 tranquilles.
//
// ⚠️ UNE LISTE VIDE NE SE JUGE PAS : ne pas savoir lire le schema n'est pas une
// raison de refuser de jouer. Et un `monde` vide ne demande rien a ce champ :
// une base d'avant les mondes continue de servir comme avant.
func gardeChampMonde(d Depot, monde string) *Reponse {
	if monde == "" {
		return nil
	}
	champs := d.ChampsPlateau()
	if len(champs) == 0 {
		return nil
	}
	for _, n := range champs {
		if n == "typeOfPlateau2" {
			return nil
		}
	}
	r := erreur(500, "SCHEMA : la collection `plateaux` n'a pas de champ `typeOfPlateau2`. "+
		"C'est lui qui dit sur QUEL monde se joue un plateau : sans lui, la colonie "+
		"fabriquee ici serait ecrite sans monde et refabriquee a chaque ouverture. "+
		"Lance `patch-type-plateau-2-2026-09-13.js` (il pose le champ sur `tuiles`, "+
		"`templates` ET `plateaux`).",
		map[string]any{"champs_plateau": champs, "cree": false})
	return &r
}

// gardeChampPlanete — ⚠️ LE CHAMP `planete` SUR `plateaux`, RELEVE A CHAQUE
// APPEL. Meme piege que `t` et que `typeOfPlateau2` avant lui : `Record.Set`
// sur un champ absent de la collection ne se plaint pas, garde la valeur en
// memoire, et la perd au SAVE. Sans ce controle, le plateau fabrique ici serait
// ecrit SANS planete, la lecture suivante ne le retrouverait pas, et on en
// refabriquerait un a chaque ouverture — en silence, sous des 200.
//
// ⚠️ Une collection qui ne connait AUCUN champ (`ChampsPlateau` vide) ne se
// juge pas : on ne sait pas, donc on ne refuse pas.
func gardeChampPlanete(d Depot) *Reponse {
	champs := d.ChampsPlateau()
	if len(champs) == 0 {
		return nil
	}
	for _, n := range champs {
		if n == "planete" {
			return nil
		}
	}
	r := erreur(500, "SCHEMA : la collection `plateaux` n'a pas de champ `planete`. "+
		"C'est lui qui dit sur QUELLE planete se joue un plateau : sans lui, celui "+
		"fabrique ici serait ecrit sans planete et refabrique a chaque ouverture. "+
		"Lance `patch-relation-planete-2026-09-14.js`.", map[string]any{"cree": false})
	return &r
}

// planeteDemandee — la planete sur laquelle on veut jouer.
//
// ⚠️⚠️ DEUX FACONS DE LA DESIGNER, ET LA SECONDE A UNE DATE DE PEREMPTION :
//
//	· `planete` — son id. C'est la bonne, celle que le client enverra ;
//	· `monde`   — son NOM (« Terre », « Jupiter »), le temps que Unity bascule.
//
// Le pont existe pour qu'un client d'avant les planetes continue de jouer
// pendant la bascule. ⚠️ IL PART AVEC LA MISE A JOUR D'UNITY, le jour meme :
// un repli qui survit a sa raison d'etre devient un second chemin, donc un
// second verdict.
func planeteDemandee(planetes []moteur.Enregistrement, planeteId, monde string) moteur.Enregistrement {
	for _, p := range planetes {
		if planeteId != "" && moteur.Texte(moteur.Champ(p, "id")) == planeteId {
			return p
		}
		// ⚠️ Comparaison STRICTE du nom, la casse comprise — comme partout
		// depuis le 13/09. « jupiter » n'est pas « Jupiter ».
		if planeteId == "" && monde != "" && moteur.Texte(moteur.Champ(p, "nom")) == monde {
			return p
		}
	}
	return nil
}

// Assurer : le plateau de ce type SUR CETTE PLANETE existe, ou il est fabrique
// depuis le modele qui lui est rattache.
func Assurer(d DepotCreateur, uid, typeVoulu, monde, planeteId string) Reponse {
	if typeVoulu == "" {
		return erreur(400, `"type" attendu ("ground", "space"…) : `+
			`on fabrique UN type, jamais « tous ».`, map[string]any{"cree": false})
	}

	cat := d.Catalogue()
	// ⚠️ MEME GARDE-FOU QUE LA PASSE ET LE GESTE. Sur un catalogue illisible,
	// aucune case du plateau neuf ne serait reconnue : la dotation de depart
	// n'aurait nulle part ou aller et disparaitrait, sur un 200 tranquille.
	if r := gardeCatalogue(cat, "aucun plateau fabrique sur ces donnees"); r != nil {
		return *r
	}
	// ⚠️ Fabriquer un plateau dans une collection sans champ `t`, c'est fabriquer
	// un plateau qui ne vivra jamais : son temps ne sera pas range.
	if r := gardeSchema(d); r != nil {
		return *r
	}
	// ⚠️ AVANT TOUT LE RESTE : sans le champ, ce qu'on ecrirait plus bas serait
	// un plateau sans monde, et on le refabriquerait indefiniment.
	if r := gardeChampMonde(d, monde); r != nil {
		return *r
	}
	if r := gardeChampPlanete(d); r != nil {
		return *r
	}
	t := d.Maintenant()

	// ─── Quelle planete ? ──────────────────────────────────────────────────
	planetes, err := d.Planetes()
	if err != nil {
		return erreur(500, "lecture des planetes : "+err.Error(), map[string]any{"cree": false})
	}
	if len(planetes) == 0 {
		// ⚠️ PAS UNE PANNE : une base ou le patch n'est pas passe. Le dire
		// plutot que de repondre 404 « aucun modele », qui enverrait chercher
		// une saisie manquante la ou il manque une table.
		return erreur(404, "Aucune planete en base. Lance `patch-planetes-2026-09-14.js` : "+
			"c'est lui qui cree « Game », « Terre » et les autres.",
			map[string]any{"cree": false})
	}
	planete := planeteDemandee(planetes, planeteId, monde)
	if planete == nil {
		quoi := "l'identifiant « " + planeteId + " »"
		if planeteId == "" {
			quoi = "le nom « " + monde + " »"
		}
		if planeteId == "" && monde == "" {
			return erreur(400, `"planete" (son identifiant) attendu : on fabrique un plateau SUR `+
				`une planete, jamais « quelque part ».`, map[string]any{"cree": false})
		}
		return erreur(404, "Aucune planete ne porte "+quoi+". Verifie l'onglet Modeles du site.",
			map[string]any{"cree": false})
	}
	planeteId = moteur.Texte(moteur.Champ(planete, "id"))
	nomPlanete := moteur.Texte(moteur.Champ(planete, "nom"))

	records, err := d.PlateauxDe(uid)
	if err != nil {
		return erreur(500, "lecture des plateaux : "+err.Error(), map[string]any{"cree": false})
	}
	for _, rec := range records {
		if moteur.Texte(moteur.Champ(rec, "typeOfPlateau")) != typeVoulu {
			continue
		}
		// ⚠️ ET LA PLANETE AUSSI. Sans cette ligne, le joueur qui a deja sa
		// colonie terrienne se verrait rendre CELLE-LA en cliquant sur une autre
		// planete, avec un 200 et un « ce joueur a deja un plateau de ce type ».
		if moteur.Texte(moteur.Champ(rec, "planete")) != planeteId {
			continue
		}
		return Reponse{200, map[string]any{"ok": true, "ecrit": false, "cree": false,
			"t": t, "joueur": uid, "verdict": "Ce joueur a deja un plateau de ce type sur cette planete.",
			"plateau": resume(rec), "lecture": cat.Lecture, "alertes": cat.Alertes, "refusees": cat.RefuseesTriees()}}
	}

	modele, err := d.ModeleDuType(typeVoulu, planeteId)
	if err != nil {
		return erreur(500, "lecture du modele : "+err.Error(), map[string]any{"cree": false})
	}
	if modele == nil {
		// ⚠️ CE N'EST PAS UNE PANNE, C'EST UNE CONFIGURATION A FAIRE. Le
		// distinguer d'un 500 evite de chercher un bug la ou il manque une
		// saisie sur le site.
		// ⚠️ LE VERDICT NOMME LES DEUX ETIQUETTES : « aucun modele ground »
		// enverrait chercher un modele qui existe — il existe, mais sur un autre
		// monde, et c'est CA qu'il faut lire.
		// ⚠️ ET C'EST AUSSI LA REPONSE POUR UNE PLANETE QUI N'EN EST PAS UNE :
		// le porte-contenu commun (« Game ») n'a AUCUN modele, expres. Le
		// verdict le dit, sinon on chercherait une saisie qu'il ne faut surtout
		// pas faire.
		return erreur(404, fmt.Sprintf("Aucun modele « %s » rattache a la planete « %s » dans "+
			"la collection `templates` (la planete compte autant que le type). Cree-le depuis "+
			"le site, onglet Planetes — ou, s'il s'agit du porte-contenu commun, c'est normal : "+
			"il ne se joue pas.", typeVoulu, nomPlanete),
			map[string]any{"cree": false, "planete": planeteId, "monde": nomPlanete})
	}

	largeur := moteur.Entier(moteur.Champ(modele, "largeur"), 0)
	hauteur := moteur.Entier(moteur.Champ(modele, "hauteur"), 0)
	if largeur <= 0 || hauteur <= 0 {
		return erreur(422, fmt.Sprintf("Le modele « %s » de « %s » n'a pas de dimensions "+
			"exploitables (%d x %d).", typeVoulu, nomPlanete, largeur, hauteur),
			map[string]any{"cree": false, "planete": planeteId, "monde": nomPlanete})
	}
	tilesBase64 := moteur.Texte(moteur.Champ(modele, "tilesBase64"))
	// ⚠️ LA GRILLE SE VERIFIE AVANT, PAS APRES. Le JS l'eprouvait juste avant
	// d'ecrire, en levant : une grille fausse devenait un 500 « EXCEPTION » la
	// ou c'est un modele mal saisi. Ici c'est un refus qui NOMME le modele.
	// ⚠️ UN OU DEUX OCTETS PAR CASE (15/09) : les deux longueurs sont justes.
	if _, ok := moteur.LireGrille(tilesBase64, largeur, hauteur); !ok {
		n := len(moteur.Base64VersOctets(tilesBase64))
		return erreur(422, fmt.Sprintf("Le modele « %s » de « %s » a une grille de %d octets "+
			"pour %d x %d = %d cases (attendu %d ou %d).", typeVoulu, nomPlanete, n, largeur, hauteur,
			largeur*hauteur, largeur*hauteur, 2*largeur*hauteur),
			map[string]any{"cree": false, "planete": planeteId, "monde": nomPlanete})
	}

	// ─── Les etats du modele, recopies ─────────────────────────────────────
	//
	// ⚠️ EN `[]any` DE `map[string]any` AVEC DES `float64`, c'est-a-dire la
	// forme qu'un aller-retour json produit — celle que `ChargerPartie` lira en
	// base demain. Un intermediaire en `map[string]int` passerait les essais et
	// perdrait tous les stocks en vrai : `CreerBatiment` compare des types
	// EXACTS. C'est la lecon du 12/09, au meme endroit.
	etats := []any{}
	for _, brut := range moteur.ListeJson(moteur.Champ(modele, "etats")) {
		s, ok := brut.(map[string]any)
		if !ok || s == nil {
			continue
		}
		x, z := moteur.Entier(s["x"], -1), moteur.Entier(s["z"], -1)
		if x < 0 || z < 0 || x >= largeur || z >= hauteur {
			continue
		}
		niveau := moteur.Entier(s["niveau"], 1)
		if niveau < 1 {
			niveau = 1
		}
		actif := true
		if b, y := s["actif"].(bool); y {
			actif = b
		}
		stock := map[string]any{}
		for code, v := range moteur.ObjetJson(s["stock"]) {
			if q := moteur.Entier(v, 0); q > 0 {
				stock[code] = float64(q)
			}
		}
		etats = append(etats, map[string]any{
			"x": float64(x), "z": float64(z), "niveau": float64(niveau),
			"actif": actif, "stock": stock,
		})
	}

	rec, err := d.NouveauPlateau()
	if err != nil || rec == nil {
		return erreur(500, "creation du record impossible", map[string]any{"cree": false})
	}
	nom := NomParDefaut(typeVoulu, nomPlanete)
	rec.Set("ownerId", uid)
	rec.Set("nom", nom)
	rec.Set("typeOfPlateau", typeVoulu)
	// ⚠️ LE MONDE EST POSE ICI, ET C'EST LA SEULE FOIS. Il ne bouge plus ensuite :
	// ni la passe, ni le geste, ni rien n'a de raison de changer le monde d'un
	// plateau. `gardeChampMonde` a deja verifie que la collection le retient.
	rec.Set("planete", planeteId)
	// ⚠️⚠️ L'ETIQUETTE TEXTE EST ECRITE EN PLUS, ET C'EST TEMPORAIRE. Le site et
	// Unity lisent encore `typeOfPlateau2` : un plateau neuf sans etiquette leur
	// serait invisible pendant la bascule. On ecrit donc LE NOM de la planete,
	// qui est exactement ce que l'etiquette valait.
	// ⚠️ CETTE LIGNE PART AVEC LE PATCH DE MENAGE, avec la colonne. Deux verites
	// pour une meme chose ne se gardent que le temps d'une bascule.
	rec.Set("typeOfPlateau2", nomPlanete)
	rec.Set("largeur", largeur)
	rec.Set("hauteur", hauteur)
	rec.Set("tilesBase64", tilesBase64)
	rec.Set("etats", etats)
	// La reserve (les `FluxStock`, dont la monnaie) demarre VIDE : seule la
	// dotation peut y tomber.
	rec.Set("reserve", map[string]any{})
	// ⚠️ Le compteur de version (garantie n° 5) demarre a 1 : « 0 » voudrait
	// dire « jamais ecrit », et ce record est sur le point de l'etre.
	rec.Set("version", 1)
	rec.Set("t", t)

	partie := moteur.ChargerPartie(rec, cat, t)

	// ─── La dotation de depart ─────────────────────────────────────────────
	//
	// ⚠️ ELLE PASSE PAR `Crediter`, donc par les memes `Ranger` que le reste :
	// un `FluxStock` tombe dans la reserve, le reste dans les coffres qui
	// peuvent le tenir, plafonds respectes. ET CE QUI NE RENTRE PAS EST DIT :
	// un modele sans entrepot ferait sinon demarrer le joueur sans rien, sans
	// qu'aucun message n'existe pour l'expliquer.
	reg := cat.Genres().Reg
	montants := moteur.NouveauSac(reg)
	amorcage := moteur.ObjetJson(moteur.Champ(modele, "amorcage"))
	for _, d := range listeDe(amorcage["ressources_depart"]) {
		ligne, ok := d.(map[string]any)
		if !ok || ligne == nil {
			continue
		}
		code := moteur.Texte(ligne["ressource"])
		q := moteur.Entier(ligne["quantite"], 0)
		if code == "" || q <= 0 {
			continue
		}
		montants.Ajouter(reg.Inscrire(code), q)
	}
	perdu := moteur.NouveauSac(reg)
	if !montants.Vide() {
		perdu = moteur.Crediter(partie.Plateau, montants, t)
	}

	// ⚠️ LES MEMES GARDE-FOUS QUE LES AUTRES ROUTES, AVANT ECRITURE. Ils ne
	// jugent pas le jeu — il n'y a aucun calcul ici — mais la FORME de ce qui
	// part en base. Un plateau mal forme ne se remarquerait qu'a la premiere
	// ouverture du jeu, sans rien pour relier le symptome a cette route.
	if err := partie.Verifier(-1); err != nil {
		return erreur(500, "GARDE-FOU : "+err.Error(), map[string]any{"cree": false})
	}
	partie.Ecrire(rec)
	if err := d.Sauver(rec); err != nil {
		return erreur(500, "ecriture : "+err.Error(), map[string]any{"cree": false})
	}

	return Reponse{200, map[string]any{"ok": true, "ecrit": true, "cree": true,
		"t": t, "joueur": uid, "monde": monde,
		// ⚠️ LE NOM DE LA PLANETE, pas `monde` : un client qui envoie l'id
		// (la bonne cle) laisse `monde` vide, et le verdict disait « de «  » ».
		"verdict": fmt.Sprintf("Plateau « %s » fabrique depuis le modele « %s » de « %s ».",
			nom, typeVoulu, nomPlanete),
		// ⚠️ `plateau` EST LU PAR UNITY (`SysBApi.Assurer` -> `Resume`) : id,
		// nom, typeOfPlateau, typeOfPlateau2, largeur, hauteur, version. Ne pas
		// le renommer.
		"plateau":        resume(rec),
		"etats_recopies": len(etats),
		"cases_figees":   len(partie.Figes),
		"dotation":       map[string]any{"versee": sacNoms(cat, montants), "perdue": sacNoms(cat, perdu)},
		"lecture":        cat.Lecture, "alertes": cat.Alertes, "refusees": cat.RefuseesTriees()}}
}

func listeDe(v any) []any {
	if l, ok := v.([]any); ok {
		return l
	}
	return moteur.ListeJson(v)
}
