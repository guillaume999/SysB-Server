// ============================================================
//  moteur/tuiles.go — le catalogue, lu et REFUSE
//
//  ⚠️ Go du 12/09, depuis `pb_hooks/moteur/cycles/tuiles.js`.
//  Ce fichier porte le §2ter (la saisie) et une partie du §3 (les unites).
//
//  ⚠️⚠️ LE CHARGEMENT EST LE SEUL MOMENT OU UNE RESSOURCE EST UNE CHAINE.
//  Passe cette porte, tout est `Code` (voir `sac.go`) : une ligne, un cout, une
//  regle d'appro et un coffre ne portent plus que des entiers. C'est ce qui
//  separe ce moteur du portage litteral du matin, qui passait 41 % de son temps
//  a hacher des noms de ressources.
//
//  ⚠️ Ce fichier REFUSE bruyamment, et c'est voulu : `par_minute`, `periode_s`
//  sur une ligne, `part`, une quantite decimale, un palier qui tourne sans
//  `cycle_minutes`, une regle d'appro incomplete. Chacun de ces refus a coute
//  une panne avant d'exister.
// ============================================================

package moteur

import (
	"fmt"
	"math"
	"sort"
)

// Brut : une tuile telle qu'elle sort du json, avant d'etre jugee. On garde la
// forme libre pour pouvoir distinguer « champ absent » de « champ a zero » —
// c'est toute la difference entre une tuile muette et une tuile REFUSEE.
type Brut = map[string]any

// CatalogueRefuse : une tuile que le moteur REFUSE de charger.
type CatalogueRefuse struct{ Message string }

func (e *CatalogueRefuse) Error() string { return "[catalogue] " + e.Message }

func refus(format string, a ...any) error {
	return &CatalogueRefuse{Message: fmt.Sprintf(format, a...)}
}

// ─── Les genres de ressource ────────────────────────────────────────────────
//
//   stock       s'accumule et se depense
//   flux        pareil, il ne differe que par « pas dans la barre »
//   mobilise    s'occupe et se rend (la population) ; ne transite par AUCUN
//               coffre — personne ne stocke des gens
//   indicateur  valeur CALCULEE (la satisfaction) ; ne se transporte pas, et
//               depuis le 11/09 ne se PRODUIT plus non plus (§5.4)
//   FluxStock   UNE SEULE RESERVE PAR PLATEAU, sans coffre de case, sans
//               plafond, sans navette : la monnaie.

type classe uint8

const (
	classeStock      classe = 0
	classeIndicateur classe = 1
	classeMobilise   classe = 2
	classeFluxStock  classe = 3
)

type Genres struct {
	Reg    *Registre
	bruts  map[string]string
	classe []classe
	// ⚠️ LES CODES DE LA TABLE DES GENRES, et EUX SEULS. Ce n'est pas la meme
	// chose que « tous les codes inscrits » : une ligne de tuile peut nommer une
	// ressource absente de la table. Le JS faisait
	// `Object.keys(plateau.genres.table)` pour la liste de repli d'une regle
	// d'appro sans ressources — donc une ressource hors table NE VOYAGE PAS.
	// Prendre tous les codes inscrits aurait elargi cette liste en silence.
	codesTable []Code
}

// CreerGenres : la table des genres, plus le registre qui en decoule.
//
// ⚠️ TOUS LES CODES CONNUS SONT INSCRITS ICI, avant toute tuile. Un code
// rencontre plus tard (un stock de scenario qui nomme une ressource absente de
// la table) s'inscrit quand meme — et vaut `stock`, exactement comme en JS ou
// `table[code]` valait `undefined` et ou `enCoffre` rendait donc `true`.
func CreerGenres(carte map[string]string) *Genres {
	g := &Genres{Reg: NouveauRegistre(), bruts: map[string]string{}}
	noms := make([]string, 0, len(carte))
	for k := range carte {
		noms = append(noms, k)
	}
	sort.Strings(noms)
	for _, k := range noms {
		g.bruts[k] = carte[k]
		g.codesTable = append(g.codesTable, g.Reg.Inscrire(k))
	}
	return g
}

// CodesTable : les codes DECLARES dans la table des genres, ordre alphabetique.
func (g *Genres) CodesTable() []Code { return g.codesTable }

func (g *Genres) assurer() {
	for len(g.classe) < g.Reg.Nb() {
		c := Code(len(g.classe))
		var cl classe
		switch g.bruts[g.Reg.Nom(c)] {
		case "indicateur":
			cl = classeIndicateur
		case "mobilise":
			cl = classeMobilise
		case "FluxStock":
			cl = classeFluxStock
		default:
			cl = classeStock
		}
		g.classe = append(g.classe, cl)
	}
}

func (g *Genres) de(c Code) classe {
	if c < 0 {
		return classeStock
	}
	if int(c) >= len(g.classe) {
		g.assurer()
	}
	if int(c) >= len(g.classe) {
		return classeStock
	}
	return g.classe[c]
}

func (g *Genres) EstIndicateur(c Code) bool { return g.de(c) == classeIndicateur }
func (g *Genres) EstMobilise(c Code) bool   { return g.de(c) == classeMobilise }
func (g *Genres) EstFluxStock(c Code) bool  { return g.de(c) == classeFluxStock }

// EnCoffre — ⚠️ MEME LISTE POUR DEUX QUESTIONS qu'on croyait distinctes :
// « est-ce que ca prend de la place ? » et « est-ce que ca monte en navette ? ».
// Un indicateur se calcule, un `mobilise` se compte sur les places declarees,
// un `FluxStock` vit dans la reserve : aucun des trois n'occupe un coffre, donc
// aucun des trois ne voyage.
func (g *Genres) EnCoffre(c Code) bool      { return g.de(c) == classeStock }
func (g *Genres) Transportable(c Code) bool { return g.de(c) == classeStock }

// Codes : tous les codes connus, dans l'ordre alphabetique de leur nom.
func (g *Genres) Codes() []Code { return g.Reg.Alpha() }

// ─── Le chargement d'une tuile ──────────────────────────────────────────────

type Proximite struct {
	TileIds []int
	Nombre  int
	Rayon   int
}

type Ligne struct {
	Ressource Code
	Quantite  int
	// §4 — « en direct » : pris sans navette, sans limite de distance, et
	// JAMAIS ALLE CHERCHER PAR UNE NAVETTE.
	Direct bool
	// §5.5 (13/09) — LIGNE BONUS, en pour cent. `0` = ligne ordinaire.
	//
	// ⚠️⚠️ CE N'EST PAS LE RETOUR DE `part`, ET LA DIFFERENCE EST TOUT :
	// `part` declarait le poids de CHAQUE ligne, si bien que des maisons
	// parfaitement nourries plafonnaient a la somme de leurs parts. Un bonus
	// ne se declare QUE sur les lignes en plus : il n'entre pas dans la
	// demande de base, il AJOUTE son pourcentage au prorata de ce qu'il
	// recoit, et il ne peut donc que monter (§5.5).
	//
	// ⚠️ Une ligne bonus NE BLOQUE JAMAIS un cycle (`DeQuoiTourner`), sans
	// quoi le « en plus » deviendrait un « obligatoire ».
	// ⚠️ Une ligne de PRODUCTION ne porte pas de bonus — refuse au chargement.
	Bonus int
	// Une ligne de PRODUCTION peut SUIVRE un indicateur ; elle ne le fabrique
	// jamais (§5.4).
	Indicateur string
	// ⚠️ DEJA TRIEES PAR SEUIL DECROISSANT au chargement. Le JS retriait la
	// liste a CHAQUE appel de `tranche()`, c'est-a-dire a chaque production
	// indexee de chaque cycle.
	Tranches   [][2]int
	Proximites []Proximite
}

type Cout struct {
	Ressource Code
	Quantite  int
	Mode      string // "paye" | "mobilise"
}

type Appro struct {
	Sens       string // "envoi" | "recolte"
	Cible      string
	TileIds    []int
	Rayon      int
	Ressources []Code
	Illimite   bool
	Navettes   int // debit.navettes
	Capacite   int // debit.quantite
	Crans      int // vitesse.crans
	PeriodeS   int // vitesse.periode_s

	// ⚠️ PRECALCULE AU CHARGEMENT : les ressources que cette regle peut porter,
	// dedoublonnees, filtrees sur « ca voyage », dans l'ordre alphabetique. Le
	// JS refaisait cette liste a chaque depart et a chaque arrivee.
	Transportables []Code
}

type Tuile struct {
	Tid         int
	Nom         string
	Utilisation []Ligne
	Production  []Ligne

	// Le coffre declare, indexe par `Code`. `-1` = non declare, ce qui n'est
	// PAS zero : c'est ce qui fait basculer sur le fourre-tout `*`.
	stockage      []int
	etoile        int    // le plafond `*`, 0 s'il n'y en a pas
	stockageCodes []Code // les codes declares (hors `*`), en ordre alphabetique
	declares      map[Code]int

	StockCommun bool
	// ☑️ « demarre avec ce qu'il y a » (§2ter, tranche le 11/09). DECOCHE PAR
	// DEFAUT : sans la case, c'est TOUT OU RIEN (§2).
	DemarrePartiel bool
	DureeChantier  int
	Cout           []Cout
	Appros         []Appro
	DureeCycleS    int

	// ⚠️ PRECALCULES : ils ne dependent que de la tuile, et le JS les recalculait
	// a chaque appel.
	commun       bool
	stocke       bool
	aProximites  bool
	aDirect      bool
	placesLogees int
	envoie       bool

	// Les champs du GESTE — ni produire ni acheminer ne les lit. Les oublier ne
	// fait pas planter le moteur : ca rend juste toute tuile destructible et
	// remplacable, en silence.
	ReglesPose       []ReglePose
	TypeOfPlateau    string
	NbNiveaux        int
	Indestructible   bool
	NonRemplacable   bool
	ApresDestruction int
}

func aChamp(b Brut, nom string) bool {
	v, ok := b[nom]
	if !ok || v == nil {
		return false
	}
	if s, estTexte := v.(string); estTexte && s == "" {
		return false
	}
	return true
}

func entierPositif(v any) bool {
	f, ok := v.(float64)
	return ok && !math.IsInf(f, 0) && !math.IsNaN(f) && math.Floor(f) == f && f >= 0
}

func nombreOu(b Brut, nom string, defaut int) int {
	if f, ok := b[nom].(float64); ok {
		return int(f)
	}
	return defaut
}

func texte(b Brut, nom string) string {
	if s, ok := b[nom].(string); ok {
		return s
	}
	return ""
}

func booleen(b Brut, nom string) bool {
	v, _ := b[nom].(bool)
	return v
}

func liste(b Brut, nom string) []any {
	if l, ok := b[nom].([]any); ok {
		return l
	}
	return nil
}

func entiers(b Brut, nom string) []int {
	var out []int
	for _, v := range liste(b, nom) {
		if f, ok := v.(float64); ok {
			out = append(out, int(f))
		}
	}
	return out
}

// ChargerLigne : une ligne de consommation ou de production, telle qu'elle sort
// du formulaire du §2ter : une RESSOURCE et une QUANTITE PAR CYCLE, rien d'autre.
//
// ⚠️ `par_minute` EST REFUSE. C'est l'ancienne notation, celle d'un modele ou le
// batiment etait un debit. Deux facons de saisir un rythme, c'est la faute qui a
// laisse passer « 10 nourriture par seconde » jusqu'au 08/09 (§2ter).
//
// ⚠️ `periode_s` SUR LA LIGNE EST REFUSE AUSSI. La periode est sur le PALIER,
// jamais sur la ligne : un batiment a UN rythme.
func ChargerLigne(g *Genres, brut Brut, ou string) (Ligne, error) {
	var l Ligne
	if _, y := brut["par_minute"]; y {
		return l, refus("%s : `par_minute` est l'ancienne notation — le cycle porte "+
			"le rythme, la ligne porte une quantite (§2ter)", ou)
	}
	if _, y := brut["periode_s"]; y {
		return l, refus("%s : `periode_s` sur une ligne — la periode est sur le "+
			"PALIER, jamais sur la ligne (§2ter)", ou)
	}
	if texte(brut, "ressource") == "" {
		return l, refus("%s : ligne sans ressource", ou)
	}
	if _, y := brut["part"]; y {
		return l, refus("%s : `part` n'existe plus. La satisfaction NE SE DECLARE PAS, "+
			"elle se constate (§5.4) — c'est la somme des `part` declarees qui faisait "+
			"rendre 60 %% a des maisons parfaitement nourries.", ou)
	}
	if !entierPositif(brut["quantite"]) {
		return l, refus("%s : quantite `%v` — une quantite par cycle est un ENTIER "+
			"d'unites reelles (§3, le 1/3600 a disparu)", ou, brut["quantite"])
	}

	if v, y := brut["bonus"]; y && v != nil && !entierPositif(v) {
		return l, refus("%s : bonus `%v` — un bonus de satisfaction est un "+
			"POURCENTAGE entier et positif (§5.5)", ou, v)
	}

	l.Ressource = g.Reg.Inscrire(texte(brut, "ressource"))
	l.Quantite = nombreOu(brut, "quantite", 0)
	l.Direct = booleen(brut, "direct")
	l.Bonus = nombreOu(brut, "bonus", 0)
	l.Indicateur = texte(brut, "indicateur")
	for _, t := range liste(brut, "tranches") {
		if paire, ok := t.([]any); ok && len(paire) >= 2 {
			a, _ := paire[0].(float64)
			b, _ := paire[1].(float64)
			l.Tranches = append(l.Tranches, [2]int{int(a), int(b)})
		}
	}
	// ⚠️ TRI DECROISSANT PAR SEUIL, ICI ET UNE SEULE FOIS. Tri STABLE, parce que
	// le `sort` de JS l'est aussi : deux tranches de meme seuil doivent rester
	// dans l'ordre de saisie, sinon un escalier mal saisi ne rend pas la meme
	// marche des deux cotes.
	sort.SliceStable(l.Tranches, func(i, j int) bool { return l.Tranches[i][0] > l.Tranches[j][0] })

	for _, p := range liste(brut, "proximites") {
		pb, ok := p.(Brut)
		if !ok {
			continue
		}
		var pr Proximite
		for _, t := range entiers(pb, "tileIds") {
			if t > 0 {
				pr.TileIds = append(pr.TileIds, t)
			}
		}
		pr.Nombre = nombreOu(pb, "nombre", 0)
		pr.Rayon = -1
		if aChamp(pb, "rayon") {
			pr.Rayon = nombreOu(pb, "rayon", -1)
		}
		l.Proximites = append(l.Proximites, pr)
	}
	return l, nil
}

// ChargerAppro : une regle d'appro. **Plus aucun defaut** (regle du 08/09) : une
// regle sans `illimite` et sans `debit` + `vitesse` complets decrit un monde qui
// n'existe pas — elle doit echouer bruyamment, pas circuler a 1 cran / 20 s en
// silence.
func ChargerAppro(g *Genres, a Brut, i int, nom string) (Appro, error) {
	ou := fmt.Sprintf("%s / appro #%d", nom, i)
	var ap Appro
	ap.Illimite = a["illimite"] == true
	ap.Sens = "recolte"
	if texte(a, "sens") == "envoi" {
		ap.Sens = "envoi"
	}
	ap.Cible = texte(a, "cible")
	ap.TileIds = entiers(a, "tileIds")
	ap.Rayon = -1
	if aChamp(a, "rayon") {
		ap.Rayon = nombreOu(a, "rayon", -1)
	}
	for _, v := range liste(a, "ressources") {
		if s, ok := v.(string); ok {
			ap.Ressources = append(ap.Ressources, g.Reg.Inscrire(s))
		}
	}

	exige := func(nomBloc string, champs ...string) ([]int, error) {
		bloc, ok := a[nomBloc].(Brut)
		if !ok || bloc == nil {
			return nil, refus("%s : `%s` manquant", ou, nomBloc)
		}
		out := make([]int, 0, len(champs))
		for _, c := range champs {
			v, y := bloc[c]
			if !y || v == nil {
				return nil, refus("%s : `%s.%s` manquant", ou, nomBloc, c)
			}
			if s, estTexte := v.(string); estTexte && s == "" {
				return nil, refus("%s : `%s.%s` manquant", ou, nomBloc, c)
			}
			f, _ := v.(float64)
			out = append(out, int(f))
		}
		return out, nil
	}

	if ap.Illimite {
		ap.Navettes, ap.Capacite = 0, 0
		ap.Crans, ap.PeriodeS = 0, 1
		return ap, nil
	}
	d, err := exige("debit", "navettes", "quantite")
	if err != nil {
		return ap, err
	}
	v, err := exige("vitesse", "crans", "periode_s")
	if err != nil {
		return ap, err
	}
	ap.Navettes, ap.Capacite = d[0], d[1]
	ap.Crans, ap.PeriodeS = v[0], v[1]
	return ap, nil
}

// ChargerTuile : `cycle_minutes` est un ENTIER DE MINUTES -> `DureeCycleS = X * 60`.
//
// ⚠️ Corollaire assume : le cycle le plus court est UNE MINUTE (§2ter).
func ChargerTuile(g *Genres, brut Brut) (*Tuile, error) {
	nom := "tuile ?"
	if v, ok := brut["tid"].(float64); ok && v != 0 {
		nom = fmt.Sprintf("tuile %d", int(v))
	} else if s := texte(brut, "nom"); s != "" {
		nom = "tuile " + s
	}

	t := &Tuile{
		Tid:            nombreOu(brut, "tid", 0),
		Nom:            texte(brut, "nom"),
		StockCommun:    booleen(brut, "stock_commun"),
		DemarrePartiel: booleen(brut, "demarre_partiel"),
		NbNiveaux:      1,
	}
	if d := nombreOu(brut, "duree_construction_s", 0); d > 0 {
		t.DureeChantier = d
	}

	// Les regles de pose, typees ici et une seule fois.
	for _, r := range liste(brut, "placements") {
		if rb, ok := r.(Brut); ok {
			t.ReglesPose = append(t.ReglesPose, chargerReglePose(rb))
		}
	}
	// ⚠️ DEUX ORTHOGRAPHES, et c'est voulu : les vecteurs ecrivent
	// `type_de_plateau` / `nb_niveaux` / `apres_destruction`, la vraie base
	// ecrit `typeOfPlateau` / `tileId_apres_destruction`. Lire les deux ici
	// evite un adaptateur de plus a l'etape 3.
	t.TypeOfPlateau = texte(brut, "type_de_plateau")
	if t.TypeOfPlateau == "" {
		t.TypeOfPlateau = texte(brut, "typeOfPlateau")
	}
	if n := nombreOu(brut, "nb_niveaux", 0); n > 1 {
		t.NbNiveaux = n
	}
	t.Indestructible = booleen(brut, "indestructible")
	t.NonRemplacable = booleen(brut, "non_remplacable")
	t.ApresDestruction = nombreOu(brut, "apres_destruction", 0)
	if t.ApresDestruction == 0 {
		t.ApresDestruction = nombreOu(brut, "tileId_apres_destruction", 0)
	}

	for _, l := range liste(brut, "utilisation") {
		lb, _ := l.(Brut)
		ligne, err := ChargerLigne(g, lb, nom+" / utilisation")
		if err != nil {
			return nil, err
		}
		t.Utilisation = append(t.Utilisation, ligne)
	}
	for _, l := range liste(brut, "production") {
		lb, _ := l.(Brut)
		ligne, err := ChargerLigne(g, lb, nom+" / production")
		if err != nil {
			return nil, err
		}
		t.Production = append(t.Production, ligne)
	}

	// Le stockage : `*` a part, le reste indexe par code.
	declares := map[Code]int{}
	if st, ok := brut["stockage"].(Brut); ok {
		for code, v := range st {
			f, _ := v.(float64)
			if code == "*" {
				t.etoile = int(f)
				continue
			}
			declares[g.Reg.Inscrire(code)] = int(f)
		}
	}

	for _, c := range liste(brut, "cout") {
		cb, _ := c.(Brut)
		mode := "paye"
		if texte(cb, "mode") == "mobilise" {
			mode = "mobilise"
		}
		t.Cout = append(t.Cout, Cout{
			Ressource: g.Reg.Inscrire(texte(cb, "ressource")),
			Quantite:  nombreOu(cb, "quantite", 0),
			Mode:      mode,
		})
	}

	for i, a := range liste(brut, "appros") {
		ab, _ := a.(Brut)
		ap, err := ChargerAppro(g, ab, i, nom)
		if err != nil {
			return nil, err
		}
		t.Appros = append(t.Appros, ap)
	}

	// ⚠️ §5.4 — LA SATISFACTION NE SE DECLARE PAS, ELLE SE CONSTATE.
	for _, l := range t.Production {
		if l.Indicateur != "" && len(l.Tranches) == 0 {
			return nil, refus("%s : la ligne « %s » suit l'indicateur « %s » sans aucune "+
				"tranche — une ligne SUIT un indicateur, elle ne le fabrique pas (§5.4)",
				nom, g.Reg.Nom(l.Ressource), l.Indicateur)
		}
		// ⚠️ §5.5 — un bonus se CONSOMME. Sur une production il n'aurait aucun
		// sens, et il se lirait comme « ce batiment fabrique de la
		// satisfaction » — precisement ce que le §5.4 a supprime.
		if l.Bonus != 0 {
			return nil, refus("%s : la ligne « %s » porte un bonus — un bonus de "+
				"satisfaction se declare sur une CONSOMMATION, jamais sur une "+
				"production (§5.5)", nom, g.Reg.Nom(l.Ressource))
		}
	}

	// La periode est sur le palier, et elle n'a de sens que s'il se passe
	// quelque chose. Une tuile decorative n'a pas de cycle a declarer.
	tourne := len(t.Utilisation) > 0 || len(t.Production) > 0
	cm, aCycle := brut["cycle_minutes"]
	if !aCycle || cm == nil {
		if tourne {
			return nil, refus("%s : consomme ou produit sans `cycle_minutes` — un "+
				"batiment a UN rythme, et il se declare (§2ter)", nom)
		}
		t.DureeCycleS = 0
	} else {
		if !entierPositif(cm) || cm.(float64) < 1 {
			return nil, refus("%s : `cycle_minutes` = %v — c'est un ENTIER de minutes "+
				">= 1 ; le cycle le plus court est UNE minute (§2ter)", nom, cm)
		}
		t.DureeCycleS = int(cm.(float64)) * 60
	}

	t.figer(g, declares)
	return t, nil
}

// figer : tout ce qui ne depend que de la tuile se calcule ICI, une fois.
//
// ⚠️ A rappeler si un code est inscrit APRES le chargement de cette tuile — le
// tableau de stockage serait trop court. `Catalogue.Finir` s'en charge.
func (t *Tuile) figer(g *Genres, declares map[Code]int) {
	n := g.Reg.Nb()
	t.stockage = make([]int, n)
	for i := range t.stockage {
		t.stockage[i] = -1 // ⚠️ -1 = NON DECLARE, ce qui n'est pas zero
	}
	t.stockageCodes = t.stockageCodes[:0]
	for c, v := range declares {
		if int(c) < n {
			t.stockage[c] = v
		}
	}
	for _, c := range g.Reg.Alpha() {
		if int(c) < n && t.stockage[c] >= 0 {
			t.stockageCodes = append(t.stockageCodes, c)
		}
	}
	t.declares = declares

	// aProximites : la tuile porte-t-elle la moindre regle de proximite ? Si
	// non, `Facteur` rend [1,1] sans jamais toucher au monde — c'est le cas de
	// presque tout le catalogue.
	t.aDirect = false
	for i := range t.Utilisation {
		if t.Utilisation[i].Direct {
			t.aDirect = true
		}
	}

	t.aProximites = false
	for _, champ := range [][]Ligne{t.Utilisation, t.Production} {
		for i := range champ {
			if len(champ[i].Proximites) > 0 {
				t.aProximites = true
			}
		}
	}

	// Stocke : un plafond declare > 0, `*` compris.
	t.stocke = t.etoile > 0
	if !t.stocke {
		for _, v := range declares {
			if v > 0 {
				t.stocke = true
				break
			}
		}
	}
	t.commun = t.StockCommun && t.stocke

	// PlacesLogees : le premier `mobilise` declare, dans l'ordre alphabetique.
	//
	// ⚠️⚠️ LE JS LISAIT L'ORDRE DE SAISIE, une map Go n'en a pas. En pratique
	// aucune tuile ne declare DEUX `mobilise`, donc les deux coincident ; le
	// jour ou l'une le ferait, JS et Go pourraient diverger en silence. C'est
	// un trou a trancher (spec §11), pas une liberte prise ici.
	t.placesLogees = 1
	for _, c := range t.stockageCodes {
		if t.stockage[c] > 0 && g.EstMobilise(c) {
			t.placesLogees = t.stockage[c]
			break
		}
	}

	// Envoie : une tuile qui ENVOIE est un entrepot — son contenu est
	// DEPENSABLE (26/08).
	t.envoie = false
	for i := range t.Appros {
		if t.Appros[i].Sens == "envoi" {
			t.envoie = true
		}
		// Les ressources transportables de la regle, une fois pour toutes.
		a := &t.Appros[i]
		candidats := a.Ressources
		if len(candidats) == 0 {
			// ⚠️ LA TABLE DES GENRES, pas tous les codes inscrits — voir
			// `Genres.codesTable`.
			candidats = g.CodesTable()
		}
		vus := map[Code]bool{}
		a.Transportables = a.Transportables[:0]
		for _, c := range candidats {
			if vus[c] || !g.Transportable(c) {
				continue
			}
			vus[c] = true
		}
		// ⚠️ DANS L'ORDRE ALPHABETIQUE DES NOMS — le JS faisait `codes.sort()`,
		// et cet ordre decide quelle ressource remplit une navette en premier.
		for _, c := range g.Reg.Alpha() {
			if vus[c] {
				a.Transportables = append(a.Transportables, c)
			}
		}
	}
}

// Refiger : a appeler quand TOUT le catalogue est charge.
//
// ⚠️⚠️ INDISPENSABLE, ET LE PIEGE EST SILENCIEUX : une tuile chargee AVANT
// qu'un code n'existe porte un tableau de stockage trop court, et une regle
// « toutes les ressources » qui en ignore la moitie. Le JS n'avait pas ce
// probleme — ses cles etaient des chaines et arrivaient quand elles voulaient.
// Ici, tout est indexe : il faut refermer le catalogue avant de jouer.
func Refiger(g *Genres, tuiles []*Tuile) {
	for _, t := range tuiles {
		t.figer(g, t.declares)
	}
}

// ─── Ce qu'on lit sur une tuile ─────────────────────────────────────────────

// MaxStocke : le plafond de coffre pour une ressource. `*` est le fourre-tout.
//
// ⚠️ UN CODE DECLARE A ZERO VAUT ZERO — il ne retombe PAS sur `*`. C'est le
// piege du `debitRenseigne` du 06/09, ou un zero voulait dire ILLIMITE.
func (t *Tuile) MaxStocke(c Code) int {
	v := t.etoile
	if c >= 0 && int(c) < len(t.stockage) && t.stockage[c] >= 0 {
		v = t.stockage[c]
	}
	if v < 0 {
		return 0
	}
	return v
}

func (t *Tuile) Stocke() bool { return t.stocke }

// Commun : les tuiles de ce type partagent-elles un coffre ? Cocher la case
// sans declarer le moindre plafond ne veut rien dire : une bourse de capacite
// nulle avalerait les transferts en silence.
func (t *Tuile) Commun() bool { return t.commun }

// PlacesLogees : combien d'habitants cette tuile loge — LU sur ses places
// declarees, jamais produit ni ecrit. C'est le poids de sa voix dans
// l'escalier (§5).
func (t *Tuile) PlacesLogees() int { return t.placesLogees }

// StockageCodes : les codes declares (hors `*`), en ordre alphabetique.
func (t *Tuile) StockageCodes() []Code { return t.stockageCodes }

func (t *Tuile) Etoile() int { return t.etoile }

// Envoie : une tuile qui ENVOIE est un entrepot.
func (t *Tuile) Envoie() bool { return t.envoie }

// Gere : une regle sans liste de ressources les gere TOUTES.
func (a *Appro) Gere(c Code) bool {
	if len(a.Ressources) == 0 {
		return true
	}
	for _, x := range a.Ressources {
		if x == c {
			return true
		}
	}
	return false
}

// Vise : cette regle accepte-t-elle d'avoir affaire a CE type de tuile ?
func (a *Appro) Vise(tileId int) bool {
	if a.Cible != "tuiles" {
		return true
	}
	for _, t := range a.TileIds {
		if t == tileId {
			return true
		}
	}
	return false
}
