// ============================================================
//  routes/assurer.go — « JE N'AI PAS ENCORE DE PLATEAU DE CE TYPE, FABRIQUE-LE »
//
//      POST /api/sysb/assurer      corps : { "type": "ground" }
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
	// Le modele d'un type, dans `templates`. `nil, nil` = il n'y en a pas —
	// ce n'est pas une panne, c'est une saisie a faire sur le site.
	ModeleDuType(typeVoulu string) (moteur.Enregistrement, error)
	// Un record `plateaux` NEUF, pas encore ecrit. C'est `Sauver` qui l'ecrit.
	NouveauPlateau() (moteur.Enregistrement, error)
}

// NomParDefaut — ⚠️ MEME TABLE QU'EN C#. Un type inconnu ne s'appelle PAS « Ma
// colonie » : depuis le plateau TPT (29/08) il existe un troisieme type, et lui
// donner le nom du sol ferait croire a un doublon dans la liste du joueur.
func NomParDefaut(typeVoulu string) string {
	switch typeVoulu {
	case "ground":
		return "Ma colonie"
	case "space":
		return "Ma station"
	}
	return fmt.Sprintf("Mon plateau (%s)", typeVoulu)
}

func resume(rec moteur.Enregistrement) map[string]any {
	return map[string]any{
		"id":            moteur.Texte(moteur.Champ(rec, "id")),
		"nom":           moteur.Texte(moteur.Champ(rec, "nom")),
		"typeOfPlateau": moteur.Texte(moteur.Champ(rec, "typeOfPlateau")),
		"largeur":       moteur.Entier(moteur.Champ(rec, "largeur"), 0),
		"hauteur":       moteur.Entier(moteur.Champ(rec, "hauteur"), 0),
		"version":       moteur.Entier(moteur.Champ(rec, "version"), 0),
	}
}

// Assurer : le plateau de ce type existe, ou il est fabrique depuis le modele.
func Assurer(d DepotCreateur, uid, typeVoulu string) Reponse {
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
	t := d.Maintenant()

	records, err := d.PlateauxDe(uid)
	if err != nil {
		return erreur(500, "lecture des plateaux : "+err.Error(), map[string]any{"cree": false})
	}
	for _, rec := range records {
		if moteur.Texte(moteur.Champ(rec, "typeOfPlateau")) != typeVoulu {
			continue
		}
		return Reponse{200, map[string]any{"ok": true, "ecrit": false, "cree": false,
			"t": t, "joueur": uid, "verdict": "Ce joueur a deja un plateau de ce type.",
			"plateau": resume(rec), "lecture": cat.Lecture, "alertes": cat.Alertes}}
	}

	modele, err := d.ModeleDuType(typeVoulu)
	if err != nil {
		return erreur(500, "lecture du modele : "+err.Error(), map[string]any{"cree": false})
	}
	if modele == nil {
		// ⚠️ CE N'EST PAS UNE PANNE, C'EST UNE CONFIGURATION A FAIRE. Le
		// distinguer d'un 500 evite de chercher un bug la ou il manque une
		// saisie sur le site.
		return erreur(404, fmt.Sprintf("Aucun modele « %s » dans la collection `templates`. "+
			"Cree-le depuis le site d'administration avant de lancer une partie.", typeVoulu),
			map[string]any{"cree": false})
	}

	largeur := moteur.Entier(moteur.Champ(modele, "largeur"), 0)
	hauteur := moteur.Entier(moteur.Champ(modele, "hauteur"), 0)
	if largeur <= 0 || hauteur <= 0 {
		return erreur(422, fmt.Sprintf("Le modele « %s » n'a pas de dimensions exploitables "+
			"(%d x %d).", typeVoulu, largeur, hauteur), map[string]any{"cree": false})
	}
	tilesBase64 := moteur.Texte(moteur.Champ(modele, "tilesBase64"))
	// ⚠️ LA GRILLE SE VERIFIE AVANT, PAS APRES. Le JS l'eprouvait juste avant
	// d'ecrire, en levant : une grille fausse devenait un 500 « EXCEPTION » la
	// ou c'est un modele mal saisi. Ici c'est un refus qui NOMME le modele.
	if n := len(moteur.Base64VersOctets(tilesBase64)); n != largeur*hauteur {
		return erreur(422, fmt.Sprintf("Le modele « %s » a une grille de %d octets pour "+
			"%d x %d = %d cases.", typeVoulu, n, largeur, hauteur, largeur*hauteur),
			map[string]any{"cree": false})
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
	nom := NomParDefaut(typeVoulu)
	rec.Set("ownerId", uid)
	rec.Set("nom", nom)
	rec.Set("typeOfPlateau", typeVoulu)
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
	// ⚠️ `Ecrire` verifie AUSSI que la collection retient `t` : sans le champ
	// nombre `t` sur `plateaux` (l'etape 1.1 du 11/09), l'ecriture est avalee en
	// silence et chaque passe repartirait du meme instant.
	if err := partie.Ecrire(rec); err != nil {
		return erreur(500, err.Error(), map[string]any{"cree": false})
	}
	if err := d.Sauver(rec); err != nil {
		return erreur(500, "ecriture : "+err.Error(), map[string]any{"cree": false})
	}

	return Reponse{200, map[string]any{"ok": true, "ecrit": true, "cree": true,
		"t": t, "joueur": uid,
		"verdict": fmt.Sprintf("Plateau « %s » fabrique depuis le modele « %s ».", nom, typeVoulu),
		// ⚠️ `plateau` EST LU PAR UNITY (`SysBApi.Assurer` -> `Resume`) : id,
		// nom, typeOfPlateau, largeur, hauteur, version. Ne pas le renommer.
		"plateau":        resume(rec),
		"etats_recopies": len(etats),
		"cases_figees":   len(partie.Figes),
		"dotation":       map[string]any{"versee": sacNoms(cat, montants), "perdue": sacNoms(cat, perdu)},
		"lecture":        cat.Lecture, "alertes": cat.Alertes}}
}

func listeDe(v any) []any {
	if l, ok := v.([]any); ok {
		return l
	}
	return moteur.ListeJson(v)
}
