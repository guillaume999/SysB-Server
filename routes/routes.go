// ============================================================
//  routes — CE QUE LES ROUTES DECIDENT, SANS POCKETBASE
//
//  ⚠️⚠️ POCKETBASE N'APPARAIT PAS DANS CE PAQUET, ET C'EST LE POINT. Tout ce
//  qui peut se tromper — les garde-fous, l'ordre des operations, le jeton de
//  concurrence, ce qu'on ecrit et quand — vit ici, se compile et se teste. Ce
//  qui reste dans l'adaptateur (`pb/`) est de la plomberie HTTP : un routeur,
//  une authentification, une transaction.
//
//  C'est aussi ce qui rend le portage verifiable : l'adaptateur ne peut pas
//  etre compile depuis une session Claude (le module PocketBase depend de
//  `modernc.org/sqlite`, injoignable). Il fallait donc que la frontiere tombe
//  au bon endroit — et le bon endroit, c'est juste avant la premiere decision.
//
//  ⚠️ CES ROUTES NE LEVENT PAS. Elles rendent un code HTTP et un corps. Une
//  panne se raconte, elle ne s'echappe pas : les pannes les plus cheres de ce
//  projet ont toutes REPONDU 200.
// ============================================================

package routes

import (
	"fmt"

	"sysb/moteur"
)

// Depot : ce que les routes demandent a la base, et rien de plus.
//
// ⚠️ L'implementation PocketBase est liee a UNE TRANSACTION : `Sauver` ecrit
// dans la meme que `PlateauParId`. C'est l'adaptateur qui le garantit ; ici on
// suppose seulement que les deux parlent de la meme chose.
type Depot interface {
	Catalogue() *moteur.CatalogueCharge
	Maintenant() int
	// Les plateaux d'un joueur, tries par `typeOfPlateau` — l'ordre du JS.
	PlateauxDe(uid string) ([]moteur.Enregistrement, error)
	PlateauParId(id string) (moteur.Enregistrement, error)
	Utilisateur(uid string) (moteur.Enregistrement, error)
	Sauver(r moteur.Enregistrement) error
	// ChampsPlateau : les noms des champs que la collection `plateaux` retient
	// REELLEMENT. Vide = on ne sait pas (et alors on ne juge pas).
	//
	// ⚠️⚠️ C'EST LA SEULE FACON DE VOIR QU'UN CHAMP MANQUE. `Record.Set` sur un
	// champ absent de la collection ne se plaint pas : PocketBase retombe sur
	// `SetRaw`, garde la valeur dans le record, et `Get` la rend (source
	// v0.39.2). Elle n'est perdue qu'au SAVE. Reposer la valeur puis la relire
	// — ce que faisait `Partie.Ecrire` — ne prouve donc RIEN.
	ChampsPlateau() []string
}

// Reponse : ce que l'adaptateur rend tel quel.
type Reponse struct {
	Code  int            // le code HTTP
	Corps map[string]any // le json
}

func erreur(code int, verdict string, extra map[string]any) Reponse {
	c := map[string]any{"ok": false, "ecrit": false, "verdict": verdict}
	for k, v := range extra {
		c[k] = v
	}
	return Reponse{Code: code, Corps: c}
}

// gardeCatalogue — un moteur qui n'a rien compris ressemble a un moteur qui n'a
// rien a faire : sans ce refus, une base illisible rendrait « 0 gagne » au lieu
// d'une erreur.
func gardeCatalogue(cat *moteur.CatalogueCharge, quoi string) *Reponse {
	if !cat.Illisible() {
		return nil
	}
	r := erreur(500, "CATALOGUE ILLISIBLE — "+quoi+".", map[string]any{"lecture": cat.Lecture})
	return &r
}

// gardeSchema — ⚠️ LE CHAMP NOMBRE `t` SUR `plateaux`, RELEVE A CHAQUE APPEL.
//
// C'est le plateau qui porte le temps depuis le 11/09 (spec §1). Sans ce champ,
// `Ecrire` pose un `t` que le SAVE jette : chaque lecture repart alors de
// « maintenant », `Avancer` n'a jamais rien a avancer, et **plus rien ne
// produit** — sous des 200 parfaitement tranquilles. Mesure du 13/09 : la
// collection ne l'avait toujours pas, un mois apres.
//
// ⚠️ UNE LISTE VIDE NE SE JUGE PAS : on ne sait pas lire le schema, ce n'est pas
// une raison de refuser de jouer.
func gardeSchema(d Depot) *Reponse {
	champs := d.ChampsPlateau()
	if len(champs) == 0 {
		return nil
	}
	for _, n := range champs {
		if n == "t" {
			return nil
		}
	}
	r := erreur(500, "SCHEMA : la collection `plateaux` n'a pas de champ `t`. "+
		"C'est le plateau qui porte le temps depuis le 11/09 : sans ce champ rien "+
		"n'avance et rien ne produit. Ajoute un champ NOMBRE `t` dans l'admin.",
		map[string]any{"champs_plateau": champs})
	return &r
}

func technosDuJoueur(d Depot, uid string) map[string]int {
	if u, err := d.Utilisateur(uid); err == nil && u != nil {
		return moteur.TechnosDe(u)
	}
	return map[string]int{}
}

// ─── GET /api/sysb/etat ─────────────────────────────────────────────────────

// Etat : lit, rattrape EN MEMOIRE, et n'ecrit RIEN. C'est la seule route dont
// on est sur qu'elle ne peut pas abimer une partie.
func Etat(d Depot, uid, plateauVoulu string) Reponse {
	cat := d.Catalogue()
	if r := gardeCatalogue(cat, "aucun etat rendu sur ces donnees"); r != nil {
		return *r
	}
	if r := gardeSchema(d); r != nil {
		return *r
	}
	t := d.Maintenant()
	records, err := d.PlateauxDe(uid)
	if err != nil {
		return erreur(500, "lecture des plateaux : "+err.Error(), nil)
	}
	technos := technosDuJoueur(d, uid)

	if plateauVoulu == "" {
		// La LISTE, sans rattrapage : c'est un menu, pas une partie.
		//
		// ⚠️ `largeur` / `hauteur` SONT DANS LA LISTE, et ce n'est pas du confort :
		// c'est le seul endroit ou l'ecran de choix les lit (« nom (l x h --
		// type) »), et depuis le 12/09 c'est aussi par la que le jeu retrouve
		// l'id d'un plateau DE CE TYPE — l'ancien `?type=` du JS n'existe plus.
		// Les omettre rendait « 0x0 » avec un 200 tranquille.
		var liste []any
		for _, rec := range records {
			liste = append(liste, map[string]any{
				"id":            moteur.Texte(moteur.Champ(rec, "id")),
				"nom":           moteur.Texte(moteur.Champ(rec, "nom")),
				"typeOfPlateau": moteur.Texte(moteur.Champ(rec, "typeOfPlateau")),
				"largeur":       moteur.Entier(moteur.Champ(rec, "largeur"), 0),
				"hauteur":       moteur.Entier(moteur.Champ(rec, "hauteur"), 0),
				"version":       moteur.Entier(moteur.Champ(rec, "version"), 0),
				"t":             moteur.Entier(moteur.Champ(rec, "t"), 0),
			})
		}
		return Reponse{200, map[string]any{"ok": true, "ecrit": false, "t": t,
			"joueur": uid, "plateaux": liste, "lecture": cat.Lecture,
			"alertes": cat.Alertes}}
	}

	for _, rec := range records {
		if moteur.Texte(moteur.Champ(rec, "id")) != plateauVoulu {
			continue
		}
		partie := moteur.ChargerPartie(rec, cat, t)
		// ⚠️ COMPTE LES CASES AVANT LE RATTRAPAGE : c'est « combien de cases
		// le moteur a fait avancer », pas « combien il en reste ».
		cases := len(partie.Plateau.Batiments)
		partie.Avancer(t, 0)

		// ⚠️ LES MEMES GARDE-FOUS QU'AVANT ECRITURE, APPLIQUES AVANT REPONSE.
		// Cette route n'ecrit rien, mais une LECTURE qui rend un plateau
		// amoindri fait afficher un terrain faux au joueur — et le joueur pose
		// dessus. Un etat qui ne peut pas etre vrai ne se montre pas plus qu'il
		// ne se range.
		if err := partie.Verifier(-1); err != nil {
			return erreur(500, "GARDE-FOU : "+err.Error(),
				map[string]any{"plateau": partie.Id})
		}

		return Reponse{200, map[string]any{"ok": true, "ecrit": false, "t": t,
			"joueur": uid, "lecture": cat.Lecture, "alertes": cat.Alertes,
			// ⚠️ CE QUI S'EST PASSE PENDANT L'ABSENCE, et c'est CETTE route qui
			// le dit au client : `passe` ne le met que dans ses `rapports`, un
			// par plateau. C'est le « pendant ton absence... » de l'ouverture.
			"rattrapage": map[string]any{
				"depuis":       partie.TAvant,
				"absence_s":    t - partie.TAvant,
				"cases":        cases,
				"cases_figees": len(partie.Figes),
			},
			"etat": moteur.FaireBloc(rec, partie, technos,
				moteur.Indicateurs(partie.Plateau, cat.GenresBrut, partie.Plateau.T))}}
	}
	return erreur(404, "plateau introuvable, ou il n'est pas a ce joueur", nil)
}

// ─── POST /api/sysb/passe ───────────────────────────────────────────────────

// Passe : LA SEULE ROUTE AUTORISEE A FAIRE AVANCER LE TEMPS EN BASE.
//
// ⚠️ ELLE NE TOUCHE PAS A `version`. Ce compteur dit « le TERRAIN a-t-il
// change ? » — une passe n'y touche jamais. L'incrementer faisait refuser le
// geste d'un joueur par son propre rafraichissement (09/09).
//
// ⚠️ ELLE ECRIT DES QUE LE TEMPS A AVANCE, meme si rien n'a bouge dans les
// coffres : `t` EST l'information. Sans ca, la meme minute serait recalculee a
// chaque appel.
//
// `budget` : 0 = tout le chemin. Sinon on s'arrete et `fini` est faux — le
// client rappelle. Voir la spec §11.1.
func Passe(d Depot, uid, plateauVoulu string, budget int) Reponse {
	cat := d.Catalogue()
	if r := gardeCatalogue(cat, "aucune ecriture"); r != nil {
		return *r
	}
	if r := gardeSchema(d); r != nil {
		return *r
	}
	t := d.Maintenant()
	records, err := d.PlateauxDe(uid)
	if err != nil {
		return erreur(500, "lecture des plateaux : "+err.Error(), nil)
	}
	if plateauVoulu != "" {
		var garde []moteur.Enregistrement
		for _, r := range records {
			if moteur.Texte(moteur.Champ(r, "id")) == plateauVoulu {
				garde = append(garde, r)
			}
		}
		if len(garde) == 0 {
			return erreur(404, "plateau introuvable, ou il n'est pas a ce joueur", nil)
		}
		records = garde
	}

	var rapports []any
	ecritures := 0
	var bloc any
	toutFini := true

	for _, rec := range records {
		partie := moteur.ChargerPartie(rec, cat, t)

		// ⚠️ UN PLATEAU HORODATE DANS LE FUTUR NE SE RATTRAPE PAS : ou l'horloge
		// a recule, ou quelqu'un a ecrit a la main. Dans les deux cas, avancer
		// ferait n'importe quoi.
		if partie.TAvant > t+60 {
			rapports = append(rapports, map[string]any{
				"id": partie.Id, "ecrit": false,
				"raison": fmt.Sprintf("plateau horodate dans le futur (t = %d) — passe refusee",
					partie.TAvant)})
			continue
		}

		progres := partie.Avancer(t, budget)
		if !progres.Fini {
			toutFini = false
		}

		if err := partie.Verifier(-1); err != nil {
			return erreur(500, "GARDE-FOU : "+err.Error(),
				map[string]any{"plateau": partie.Id})
		}
		changements := partie.Changements()
		reserveBouge := moteur.DifferenceReserve(cat.Genres(), partie.ReserveAvant, partie.Plateau.Reserve)

		// ⚠️ ON ECRIT DES QUE LE TEMPS A AVANCE, meme sans changement visible.
		ecrire := partie.Plateau.T > partie.TAvant
		if ecrire {
			partie.Ecrire(rec)
			if err := d.Sauver(rec); err != nil {
				return erreur(500, "ecriture : "+err.Error(), map[string]any{"plateau": partie.Id})
			}
			ecritures++
		}

		if plateauVoulu != "" {
			bloc = moteur.FaireBloc(rec, partie, technosDuJoueur(d, uid),
				moteur.Indicateurs(partie.Plateau, cat.GenresBrut, partie.Plateau.T))
		}

		rapports = append(rapports, map[string]any{
			"id": partie.Id, "nom": partie.Nom, "ecrit": ecrire,
			"version": partie.Version, "cases": len(partie.VersEtats()),
			"cases_figees": len(partie.Figes),
			"depuis":       partie.TAvant, "absence_s": t - partie.TAvant,
			// ⚠️ `fini` faux = il reste du chemin, rappelle la passe (§11.1).
			"fini": progres.Fini, "evenements": progres.Evenements,
			"reserve": moteur.VersReserve(partie.Plateau), "reserve_bouge": reserveBouge,
			"changements": premiers(changements, 40), "changements_total": len(changements),
		})
	}

	return Reponse{200, map[string]any{"ok": true, "ecrit": ecritures > 0, "t": t,
		"joueur": uid, "plateaux_ecrits": ecritures, "fini": toutFini,
		"lecture": cat.Lecture, "rapports": rapports, "etat": bloc,
		"alertes": cat.Alertes}}
}

func premiers(l []moteur.Changement, n int) []moteur.Changement {
	if len(l) > n {
		return l[:n]
	}
	return l
}

// ─── POST /api/sysb/geste ───────────────────────────────────────────────────

type DemandeGeste struct {
	Plateau string
	Action  string // "poser" | "detruire"
	X, Z    int
	Tuile   int
	// ⚠️ LE JETON DE CONCURRENCE, facultatif. `nil` = le client ne le fournit
	// pas et accepte de jouer a l'aveugle.
	Version *int
}

// Geste : juge, paie, ecrit — et n'ecrit qu'apres avoir verifie que le sol n'a
// bouge QUE sur la case visee.
//
// ⚠️ `version` MONTE A CHAQUE GESTE, REFUS COMPRIS : le client doit savoir que
// sa lecture est perimee, meme si le geste n'a rien fait.
func Geste(d Depot, uid string, dem DemandeGeste) Reponse {
	if dem.Action != "poser" && dem.Action != "detruire" {
		return erreur(400, `"action" doit valoir "poser" ou "detruire".`, nil)
	}
	if dem.X < 0 || dem.Z < 0 {
		return erreur(400, `"x" et "z" attendus.`, nil)
	}
	if dem.Action == "poser" && !(dem.Tuile > 0 && dem.Tuile < 256) {
		return erreur(400, `"tuile" attendu, entre 1 et 255.`, nil)
	}
	if dem.Plateau == "" {
		return erreur(400, `"plateau" attendu : un geste vise UN plateau, jamais « le premier ».`, nil)
	}

	cat := d.Catalogue()
	if r := gardeCatalogue(cat, "aucun geste juge sur ces donnees"); r != nil {
		return *r
	}
	if r := gardeSchema(d); r != nil {
		return *r
	}
	t := d.Maintenant()

	rec, err := d.PlateauParId(dem.Plateau)
	if err != nil || rec == nil {
		return erreur(404, "plateau introuvable", nil)
	}
	if moteur.Texte(moteur.Champ(rec, "ownerId")) != uid {
		return erreur(403, "ce plateau n'est pas a ce joueur", nil)
	}
	versionCourante := moteur.Entier(moteur.Champ(rec, "version"), 0)

	partie := moteur.ChargerPartie(rec, cat, t)
	octetsAvant := append([]int(nil), partie.Tiles...)
	casesAvant := len(partie.Plateau.Batiments)

	// Le rattrapage AVANT le geste : on juge sur l'etat a jour, jamais sur une
	// photo vieille d'une heure.
	partie.Avancer(t, 0)

	technos := technosDuJoueur(d, uid)
	catalogueGeste := map[int]*moteur.Tuile{}
	for tid := range cat.ParTileId {
		if tu := cat.TuilePour(tid, 1); tu != nil {
			catalogueGeste[tid] = tu
		}
	}

	// ⚠️ L'EMPIRE, pour une limite de portee « empire » : les AUTRES plateaux du
	// joueur, celui-ci compris (il est deja rattrape).
	var empire []moteur.Vue
	if autres, err := d.PlateauxDe(uid); err == nil {
		for _, r := range autres {
			if moteur.Texte(moteur.Champ(r, "id")) == partie.Id {
				empire = append(empire, moteur.VueDunePartie(partie))
				continue
			}
			autre := moteur.ChargerPartie(r, cat, t)
			autre.Avancer(t, 0)
			empire = append(empire, moteur.VueDunePartie(autre))
		}
	}

	terrain := moteur.TerrainDunePartie(partie)
	opt := moteur.OptionsGeste{Technos: technos, CatalogueTechnos: technosCatalogue(cat), Empire: empire}

	var pose moteur.ResultatPose
	var destr moteur.ResultatDestruction
	perime := false

	if dem.Version != nil && *dem.Version != versionCourante {
		// ⚠️ LE JETON DE CONCURRENCE. Le plateau a ete ecrit depuis la derniere
		// lecture du client : on refuse, et on rend l'etat FRAIS pour qu'il
		// refasse son geste dessus.
		perime = true
	} else if dem.Action == "poser" {
		pose = moteur.Poser(terrain, catalogueGeste, dem.X, dem.Z, dem.Tuile, t, opt)
	} else {
		destr = moteur.DetruireCase(terrain, catalogueGeste, dem.X, dem.Z, t)
	}
	ok := pose.Ok || destr.Ok

	// Le batiment neuf doit pouvoir demarrer son premier cycle tout de suite, et
	// une destruction peut en debloquer d'autres. `Avancer` jusqu'au meme
	// instant ne fait que ca : depart et reprise.
	if ok {
		partie.Avancer(t, 0)
	}

	// ⚠️ LES GARDE-FOUS DU SOL. Un geste change UNE case, celle qui est visee.
	var bouges []int
	for i := range octetsAvant {
		if i < len(partie.Tiles) && octetsAvant[i] != partie.Tiles[i] {
			bouges = append(bouges, i)
		}
	}
	attendu := 0
	if ok {
		attendu = 1
	}
	if len(bouges) != attendu {
		return erreur(500, fmt.Sprintf("GARDE-FOU : %d case(s) du sol modifiee(s), %d attendue(s)",
			len(bouges), attendu), nil)
	}
	if len(bouges) == 1 && bouges[0] != dem.Z*partie.Largeur+dem.X {
		return erreur(500, "GARDE-FOU : le sol a bouge AILLEURS que sur la case visee", nil)
	}
	apres := len(partie.Plateau.Batiments)
	if apres-casesAvant > 1 || casesAvant-apres > 1 {
		return erreur(500, fmt.Sprintf("GARDE-FOU : %d etats avant, %d apres", casesAvant, apres), nil)
	}
	if err := partie.Verifier(apres + len(partie.Figes)); err != nil {
		return erreur(500, "GARDE-FOU : "+err.Error(), nil)
	}

	changements := partie.Changements()
	reserveBouge := moteur.DifferenceReserve(cat.Genres(), partie.ReserveAvant, partie.Plateau.Reserve)

	partie.Ecrire(rec)
	versionApres := versionCourante + 1
	rec.Set("version", versionApres)
	if ok {
		rec.Set("tilesBase64", moteur.OctetsVersBase64(partie.Tiles))
	}
	if err := d.Sauver(rec); err != nil {
		return erreur(500, "ecriture : "+err.Error(), nil)
	}

	refus := pose.Refus
	if dem.Action == "detruire" {
		refus = destr.Refus
	}
	if perime {
		refus = fmt.Sprintf("le plateau a change depuis ta derniere lecture "+
			"(version %d, tu avais %d) : voici l'etat a jour, refais ton geste dessus",
			versionCourante, *dem.Version)
	}
	verdict := "Geste REFUSE : " + refus
	if ok {
		verdict = "Pose acceptee."
		if dem.Action == "detruire" {
			verdict = "Destruction acceptee."
		}
	}

	bloc := moteur.FaireBloc(rec, partie, technos,
		moteur.Indicateurs(partie.Plateau, cat.GenresBrut, partie.Plateau.T))
	// Le bloc a ete compose AVANT la sauvegarde, donc avec l'ancienne version
	// lue sur le record : on remet celle qui vient d'etre ecrite.
	bloc.Plateau.Version = versionApres
	if ok {
		bloc.Plateau.TilesBase64 = moteur.OctetsVersBase64(partie.Tiles)
	}

	g := map[string]any{"accepte": ok, "refus": refus, "perime": perime,
		"blocages": pose.Blocages, "offert": pose.Offert, "chantier": pose.Chantier,
		"ancien": destr.Ancien, "apres": destr.Apres}
	if dem.Action == "poser" {
		g["avertissements"] = pose.Avertissements
		g["paye"] = sacNoms(cat, pose.Paye)
		g["mobilise"] = sacNoms(cat, pose.Mobilise)
		g["perdu"] = sacNoms(cat, pose.Perdu)
	} else {
		g["avertissements"] = destr.Avertissements
		g["perdu"] = sacNoms(cat, destr.Perdu)
		g["rendu"] = sacNoms(cat, destr.Rendu)
	}

	return Reponse{200, map[string]any{"ok": true, "ecrit": true, "t": t,
		"joueur": uid, "plateau": dem.Plateau, "geste": g, "verdict": verdict,
		"rattrapage": map[string]any{"depuis": partie.TAvant, "absence_s": t - partie.TAvant,
			"cases": len(partie.VersEtats())},
		"changements": premiers(changements, 40), "changements_total": len(changements),
		"reserve_bouge": reserveBouge, "etat": bloc,
		"lecture": cat.Lecture, "alertes": cat.Alertes}}
}

func technosCatalogue(cat *moteur.CatalogueCharge) []moteur.Techno {
	var out []moteur.Techno
	for _, t := range cat.Technos {
		out = append(out, moteur.Techno{Code: t.Code, Brouillon: t.Brouillon, Debloque: t.Debloque})
	}
	return out
}

func sacNoms(cat *moteur.CatalogueCharge, s moteur.Sac) map[string]int {
	out := map[string]int{}
	for _, c := range s.NonNuls() {
		if q := s.Get(c); q != 0 {
			out[cat.Genres().Reg.Nom(c)] = q
		}
	}
	return out
}
