// ============================================================
//  moteur/catalogue.go — des records PocketBase aux objets du moteur
//
//  Miroir de `TuileRecord.cs` + `TuileModele.cs` cote Unity, et de
//  `src/lib/tuiles.ts` cote site. Les trois lisent LE MEME json : renommer d'un
//  cote sans les autres casse le jeu en silence — la tuile se charge, avec des
//  regles vides.
//
//  ⚠️ POURQUOI CETTE COUCHE EST SEPAREE DU MOTEUR : les vecteurs de test
//  alimentent le modele DIRECTEMENT. Un echec de vecteur accuse donc
//  l'algorithme, jamais l'analyse du json — et reciproquement. Melanger les deux
//  rendrait chaque panne illisible.
//
//  ⚠️ CE FICHIER NETTOIE, IL NE JUGE PAS : c'est `ChargerTuile` qui REFUSE une
//  tuile mal saisie (`par_minute`, `periode_s` sur une ligne, `part`, un palier
//  qui tourne sans `cycle_minutes`, une regle d'appro incomplete). On lui passe
//  donc les champs TELS QU'EN BASE, anciens compris : les effacer ici ferait
//  charger en silence une tuile que le moteur aurait du refuser.
//
//  ⚠️⚠️ UNE DIFFERENCE AVEC LE JS, ET ELLE EST STRUCTURELLE. Le JS fabriquait
//  ses tuiles A LA DEMANDE (`tuilePour(tid, niveau)` avec un cache). Ici, une
//  ressource est un ENTIER attribue au chargement : une tuile construite APRES
//  qu'un code soit apparu porterait un tableau de stockage trop court, en
//  silence. On construit donc TOUS les couples (tuile, niveau) d'un coup, puis
//  on ferme le catalogue avec `Refiger`. C'est d'ailleurs ce que le JS voulait
//  deja — « on juge TOUT le catalogue tout de suite » — il ne le faisait qu'au
//  niveau 1.
// ============================================================

package moteur

import "fmt"

// SourceRecords : d'ou viennent les records. PocketBase n'apparait pas ici.
type SourceRecords interface {
	Tous(collection string) []Enregistrement
}

// ─── Les formes brutes, telles qu'elles sont en base ────────────────────────

type palierBrut struct {
	Niveau             int
	DureeConstructionS int
	CycleMinutes       any // ⚠️ TRANSMIS TEL QUEL : c'est le moteur qui exige un entier >= 1
	DemarrePartiel     bool
	Cout               []any
	Utilisation        []any
	Production         []any
}

type tuileBrute struct {
	TileId           int
	Nom              string
	Code             string
	TypeOfPlateau    string
	Actif            bool
	Indestructible   bool
	NonRemplacable   bool
	ApresDestruction int
	Paliers          []palierBrut
	Stockage         map[string]any
	Appros           []any
	StockCommun      bool
	Placements       []any
}

// TechnoBrute : ce que la base porte d'une techno.
//
// ⚠️ `entretien` et `effets` NE SONT PLUS LUS (11/09, decision de Guillaume :
// les technos sont HORS du moteur a cycles pour l'instant). Ils restent
// saisissables sur le site, qui le dit en orange ; ils reviendront par la SPEC
// d'abord — `ECARTS-2026-09-11.md` §1 porte les trois questions a trancher.
type TechnoBrute struct {
	Code, Nom       string
	Batiment        int
	Niveaux         int
	BatimentsRequis []int
	TechnosRequises []string
	DebloqueTechnos []string
	Debloque        []int
	Brouillon       bool
	Achat           []Cout
}

// ─── Lectures ───────────────────────────────────────────────────────────────

func listeDe(v any) []any {
	if l, ok := v.([]any); ok {
		return l
	}
	return nil
}

func objetDe(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}

// ligneBrute : une ligne de palier, transmise au moteur sans etre jugee.
//
// ⚠️ `par_minute`, `periode_s` et `part` SONT TRANSMIS s'ils sont en base, pour
// que le moteur REFUSE la tuile. Les effacer ici, c'est la faute des 08/09 et
// 09/09 a l'envers : deux lecteurs du catalogue qui ne disent pas la meme chose.
func ligneBrute(v any, consommation bool) Brut {
	d := objetDe(v)
	if d == nil {
		return Brut{}
	}
	l := Brut{"ressource": Texte(d["ressource"])}
	// ⚠️ Un « 2,5 » saisi doit arriver DECIMAL au moteur, qui le refuse.
	// L'arrondir ici le ferait passer en silence.
	if f, ok := d["quantite"].(float64); ok {
		l["quantite"] = f
	} else if s, ok := d["quantite"].(string); ok && s != "" {
		l["quantite"] = float64(Entier(s, 0))
	} else {
		l["quantite"] = 0.0
	}
	l["proximites"] = proximitesBrutes(d["proximites"])

	if consommation {
		l["direct"] = d["direct"] == true
	} else {
		l["indicateur"] = Texte(d["indicateur"])
		// ⚠️ Le site ecrit `{seuil, rendement}` ; le moteur lit des COUPLES.
		var tr []any
		for _, t := range listeDe(d["tranches"]) {
			if tb := objetDe(t); tb != nil {
				tr = append(tr, []any{float64(Entier(tb["seuil"], 0)), float64(Entier(tb["rendement"], 0))})
			}
		}
		l["tranches"] = tr
	}
	for _, mort := range []string{"par_minute", "periode_s", "part"} {
		if v, y := d[mort]; y && v != nil && v != "" {
			l[mort] = v
		}
	}
	return l
}

// proximitesBrutes : les regles qui ne disent rien sont ecartees ICI, une bonne
// fois — le moteur n'a jamais a se demander si une regle est utile.
//
// ⚠️ Un id hors 1..255 est ecarte : un octet de plateau ne peut pas le porter,
// donc aucune case ne le montrerait jamais.
func proximitesBrutes(v any) []any {
	var out []any
	for _, p := range listeDe(v) {
		pb := objetDe(p)
		if pb == nil {
			continue
		}
		var ids []any
		vus := map[int]bool{}
		for _, id := range listeDe(pb["tileIds"]) {
			n := Entier(id, 0)
			if n > 0 && n < 256 && !vus[n] {
				vus[n] = true
				ids = append(ids, float64(n))
			}
		}
		nombre := Entier(pb["nombre"], 0)
		if nombre <= 0 || len(ids) == 0 {
			continue // regle vide = ignoree
		}
		rayon := -1
		if pb["rayon"] != nil {
			rayon = Entier(pb["rayon"], -1)
		}
		out = append(out, Brut{"tileIds": ids, "nombre": float64(nombre), "rayon": float64(rayon)})
	}
	return out
}

// ⚠️ Un mode vide vaut `paye` (TuileModele.cs : `EstPaye => mode != "mobilise"`).
func coutBrut(v any) Brut {
	d := objetDe(v)
	if d == nil {
		return Brut{}
	}
	mode := "paye"
	if Texte(d["mode"]) == "mobilise" {
		mode = "mobilise"
	}
	return Brut{"ressource": Texte(d["ressource"]),
		"quantite": float64(Entier(d["quantite"], 0)), "mode": mode}
}

func palierDe(v any) palierBrut {
	d := objetDe(v)
	p := palierBrut{Niveau: Entier(d["niveau"], 1),
		DureeConstructionS: Entier(d["duree_construction_s"], 0),
		DemarrePartiel:     d["demarre_partiel"] == true}
	if cm, y := d["cycle_minutes"]; y && cm != nil && cm != "" {
		if f, ok := cm.(float64); ok {
			p.CycleMinutes = f
		} else {
			p.CycleMinutes = float64(Entier(cm, 0))
		}
	}
	for _, c := range listeDe(d["cout"]) {
		p.Cout = append(p.Cout, coutBrut(c))
	}
	for _, l := range listeDe(d["utilisation"]) {
		p.Utilisation = append(p.Utilisation, ligneBrute(l, true))
	}
	for _, l := range listeDe(d["production"]) {
		p.Production = append(p.Production, ligneBrute(l, false))
	}
	return p
}

// regleApproBrute — ⚠️ AUCUN DEFAUT : c'est `ChargerAppro` qui exige `debit` et
// `vitesse` complets hors `illimite` et REFUSE la tuile sinon (« regle sans
// vitesse est une erreur », 08/09). Un defaut pose ici ferait circuler a une
// vitesse que personne n'a choisie.
//
// ⚠️ `rayon` : -1, absent ou null = tout le plateau. Jamais 0 pour ca — zero a
// le sens legitime de « la case elle-meme ».
//
// ⚠️ `sens` : le catalogue ecrit « entrant » / « envoi » ; le moteur normalise
// tout ce qui n'est pas « envoi » en recolte, UNE fois, au chargement.
func regleApproBrute(v any) Brut {
	d := objetDe(v)
	if d == nil {
		return Brut{}
	}
	nombres := func(bloc any, champs ...string) any {
		b := objetDe(bloc)
		if b == nil {
			return nil
		}
		o := Brut{}
		for _, c := range champs {
			x := b[c]
			if x == nil || x == "" {
				o[c] = nil
				continue
			}
			n := Entier(x, -1)
			if n < 0 {
				o[c] = nil
			} else {
				o[c] = float64(n)
			}
		}
		return o
	}
	var ids []any
	for _, id := range listeDe(d["tileIds"]) {
		ids = append(ids, float64(Entier(id, 0)))
	}
	var res []any
	for _, r := range listeDe(d["ressources"]) {
		res = append(res, Texte(r))
	}
	sens := "entrant"
	if Texte(d["sens"]) == "envoi" {
		sens = "envoi"
	}
	rayon := -1.0
	if d["rayon"] != nil {
		rayon = float64(Entier(d["rayon"], -1))
	}
	return Brut{"sens": sens, "cible": Texte(d["cible"]), "tileIds": ids,
		"rayon": rayon, "ressources": res, "illimite": d["illimite"] == true,
		"debit":   nombres(d["debit"], "navettes", "quantite"),
		"vitesse": nombres(d["vitesse"], "crans", "periode_s")}
}

func tuileDepuisRecord(r Enregistrement) tuileBrute {
	t := tuileBrute{
		TileId: Entier(Champ(r, "tileId"), 0),
		Nom:    Texte(Champ(r, "nom")),
		Code:   Texte(Champ(r, "code")),
		// ⚠️ LES TROIS CHAMPS DU GESTE. Ils ne servent ni a produire ni a
		// acheminer : ils disent ce que le JOUEUR a le droit de faire de la
		// case. Les oublier ne fait pas planter le moteur — ca rend juste toute
		// tuile destructible et remplacable, en silence.
		Indestructible:   Champ(r, "indestructible") == true,
		NonRemplacable:   Champ(r, "non_remplacable") == true,
		ApresDestruction: Entier(Champ(r, "tileId_apres_destruction"), 0),
		TypeOfPlateau:    Texte(Champ(r, "typeOfPlateau")),
		Actif:            Champ(r, "actif") != false,
		Stockage:         map[string]any{},
	}
	for _, p := range ListeJson(Champ(r, "niveaux")) {
		t.Paliers = append(t.Paliers, palierDe(p))
	}
	log := objetDe(LireJson(Champ(r, "logistique")))
	// Le stockage passe d'une LISTE (le json) a une TABLE (ce que lit le moteur).
	for _, l := range listeDe(log["stockage"]) {
		lb := objetDe(l)
		if lb == nil || Texte(lb["ressource"]) == "" {
			continue
		}
		m := Entier(lb["max"], 0)
		if m < 0 {
			m = 0
		}
		t.Stockage[Texte(lb["ressource"])] = float64(m)
	}
	for _, a := range listeDe(log["appros"]) {
		t.Appros = append(t.Appros, regleApproBrute(a))
	}
	t.StockCommun = log["stock_commun"] == true
	// ⚠️ Le champ json s'appelle `placement` au SINGULIER en base ; la liste
	// qu'il contient, elle, est au pluriel partout dans le code.
	t.Placements = ListeJson(Champ(r, "placement"))
	return t
}

func technoDepuisRecord(r Enregistrement) TechnoBrute {
	nom := Texte(Champ(r, "nom"))
	code := Texte(Champ(r, "code"))
	if nom == "" {
		nom = code // le nom affiche, jamais vide : il ne sert qu'a REDIGER les refus
	}
	niveaux := Entier(Champ(r, "niveaux"), 1)
	if niveaux < 1 {
		niveaux = 1
	}
	t := TechnoBrute{Code: code, Nom: nom,
		Batiment: Entier(Champ(r, "batiment"), 0), Niveaux: niveaux}
	t.Brouillon = t.Batiment <= 0

	entiersValides := func(v any) []int {
		var out []int
		for _, x := range listeDe(LireJson(v)) {
			if n := Entier(x, 0); n > 0 && n < 256 {
				out = append(out, n)
			}
		}
		return out
	}
	textesValides := func(v any) []string {
		var out []string
		for _, x := range listeDe(LireJson(v)) {
			if s := Texte(x); s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	// Les batiments qu'il faut POSSEDER (acheves) pour la chercher.
	t.BatimentsRequis = entiersValides(Champ(r, "batiments_requis"))
	// ⚠️ LES DEUX BOUTS DE LA MEME FLECHE. « A debloque B » veut dire « B exige
	// A » : lire `technos_requises` seul rendrait la moitie des aretes
	// invisibles. C'est la route de recherche qui en fait l'union.
	t.TechnosRequises = textesValides(Champ(r, "technos_requises"))
	t.DebloqueTechnos = textesValides(Champ(r, "debloque_technos"))
	// Les batiments que cette recherche rend constructibles.
	t.Debloque = entiersValides(Champ(r, "debloque"))
	return t
}

// ─── Charger tout ───────────────────────────────────────────────────────────

// CatalogueCharge : le catalogue complet, lu une fois par passe.
type CatalogueCharge struct {
	genres     *Genres
	GenresBrut map[string]string
	ParTileId  map[int]tuileBrute
	Technos    []TechnoBrute
	// Une ALERTE n'est pas une erreur — c'est un refus de calculer faux en
	// silence.
	Alertes  []string
	Communs  []string
	Refusees map[int]string

	tuiles  map[[2]int]*Tuile // (tid, niveau) -> tuile chargee, ou absente si refusee
	Lecture LectureCatalogue
}

type LectureCatalogue struct {
	Ressources, Tuiles, TuilesAvecPalier, TuilesRefusees, Technologies, TypesAStockCommun int
}

func (c *CatalogueCharge) Genres() *Genres { return c.genres }

// TuilePour — ⚠️ REND nil POUR UNE TUILE REFUSEE, et la raison est dans
// `Alertes` (une fois par tuile). L'appelant ne fait PAS jouer une case dont la
// tuile est refusee, et il ne l'efface pas non plus : `ChargerPartie` la garde
// telle quelle. Une faute de saisie sur le site ne doit pas vider les cases des
// joueurs au premier rafraichissement.
//
// ⚠️ Une tuile ABSENTE du catalogue (inconnue, ou decochee `actif`) rend aussi
// nil, sans alerte : c'est un etat du catalogue, pas une faute.
func (c *CatalogueCharge) TuilePour(tid, niveau int) *Tuile {
	if t, ok := c.tuiles[[2]int{tid, niveau}]; ok {
		return t
	}
	// Un niveau non saisi retombe sur le premier palier, comme en JS.
	if t, ok := c.tuiles[[2]int{tid, 1}]; ok {
		return t
	}
	return nil
}

// Illisible : un moteur qui n'a rien compris ressemble a un moteur qui n'a rien
// a faire. Sans ce garde, une base illisible rendrait « 0 gagne » au lieu d'une
// erreur.
func (c *CatalogueCharge) Illisible() bool {
	return c.Lecture.Ressources == 0 || c.Lecture.Tuiles == 0 || c.Lecture.TuilesAvecPalier == 0
}

func ChargerCatalogue(src SourceRecords) *CatalogueCharge {
	c := &CatalogueCharge{
		GenresBrut: map[string]string{},
		ParTileId:  map[int]tuileBrute{},
		Refusees:   map[int]string{},
		tuiles:     map[[2]int]*Tuile{},
	}

	// Les genres, pour savoir ce qui se transporte, s'occupe, ou se calcule.
	for _, r := range src.Tous("ressources") {
		code := Texte(Champ(r, "code"))
		if code == "" {
			continue
		}
		genre := Texte(Champ(r, "genre"))
		if genre == "" {
			genre = "stock"
		}
		c.GenresBrut[code] = genre
	}
	c.genres = CreerGenres(c.GenresBrut)

	// Les tuiles, indexees par tileId.
	for _, r := range src.Tous("tuiles") {
		t := tuileDepuisRecord(r)
		if t.TileId <= 0 || t.TileId > 255 || !t.Actif {
			continue // hors plateau, ou brouillon du site
		}
		c.ParTileId[t.TileId] = t
		// Le stock commun est MODELISE depuis le 31/08 (bourses). On ne le
		// signale plus comme un manque — une alerte qui ment est pire que pas
		// d'alerte — mais on continue de DIRE qui le declare.
		if t.StockCommun {
			c.Communs = append(c.Communs, fmt.Sprintf("%d (%s)", t.TileId, t.Nom))
		}
	}

	for _, r := range src.Tous("technologies") {
		t := technoDepuisRecord(r)
		if t.Code != "" && t.Batiment > 0 {
			c.Technos = append(c.Technos, t)
		}
	}

	// ⚠️ TOUS LES COUPLES (tuile, niveau) D'UN COUP. Voir l'en-tete : un code de
	// ressource decouvert apres coup laisserait des tableaux trop courts.
	var chargees []*Tuile
	avecPalier := 0
	for _, tid := range triEntiers(c.ParTileId) {
		t := c.ParTileId[tid]
		if len(t.Paliers) > 0 {
			avecPalier++
		}
		niveaux := len(t.Paliers)
		if niveaux < 1 {
			niveaux = 1
		}
		for niveau := 1; niveau <= niveaux; niveau++ {
			tuile, err := c.construire(tid, t, niveau)
			if err != nil {
				if _, deja := c.Refusees[tid]; !deja {
					c.Refusees[tid] = err.Error()
					c.Alertes = append(c.Alertes, fmt.Sprintf(
						"TUILE REFUSEE %d (%s) — %s — ses cases ne jouent pas tant "+
							"qu'elle n'est pas corrigee sur le site.", tid, t.Nom, err.Error()))
				}
				break // inutile d'essayer les autres paliers
			}
			c.tuiles[[2]int{tid, niveau}] = tuile
			chargees = append(chargees, tuile)
		}
	}
	// ⚠️ ON FERME LE CATALOGUE. Sans ca, une tuile chargee avant qu'un code
	// n'existe porterait un tableau de stockage trop court, en silence.
	Refiger(c.genres, chargees)

	c.Lecture = LectureCatalogue{
		Ressources: len(c.GenresBrut), Tuiles: len(c.ParTileId),
		TuilesAvecPalier: avecPalier, TuilesRefusees: len(c.Refusees),
		Technologies: len(c.Technos), TypesAStockCommun: len(c.Communs),
	}
	return c
}

func (c *CatalogueCharge) construire(tid int, t tuileBrute, niveau int) (*Tuile, error) {
	var choisi *palierBrut
	for i := range t.Paliers {
		if t.Paliers[i].Niveau == niveau {
			choisi = &t.Paliers[i]
			break
		}
	}
	if choisi == nil && len(t.Paliers) > 0 {
		choisi = &t.Paliers[0]
	}

	brut := Brut{
		"tid": float64(tid), "nom": t.Nom,
		"stockage": t.Stockage, "appros": t.Appros, "stock_commun": t.StockCommun,
		// Ce qui ne sert qu'au GESTE.
		"placements": t.Placements, "typeOfPlateau": t.TypeOfPlateau,
		"indestructible": t.Indestructible, "non_remplacable": t.NonRemplacable,
		"tileId_apres_destruction": float64(t.ApresDestruction),
		// ⚠️ « Ameliorable » se lit sur le NOMBRE de paliers saisis — pas sur un
		// champ `nbNiveaux` qui n'existe pas en base.
		"nb_niveaux": float64(max1(len(t.Paliers))),
	}
	if choisi != nil {
		brut["production"] = choisi.Production
		brut["utilisation"] = choisi.Utilisation
		brut["cout"] = choisi.Cout
		brut["demarre_partiel"] = choisi.DemarrePartiel
		brut["duree_construction_s"] = float64(choisi.DureeConstructionS)
		if choisi.CycleMinutes != nil {
			brut["cycle_minutes"] = choisi.CycleMinutes
		}
	}
	return ChargerTuile(c.genres, brut)
}

func max1(n int) int {
	if n < 1 {
		return 1
	}
	return n
}

// triEntiers : les tileIds dans l'ordre croissant.
//
// ⚠️ L'ORDRE COMPTE, et pas pour l'affichage : c'est lui qui decide l'ordre
// d'inscription des codes de ressources au registre. Une map Go se parcourt au
// hasard — deux chargements du meme catalogue auraient donne deux numerotations,
// donc deux etats ecrits differents pour la meme partie.
func triEntiers(m map[int]tuileBrute) []int {
	out := make([]int, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
