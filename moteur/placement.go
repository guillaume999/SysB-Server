// ============================================================
//  moteur/placement.go — peut-on poser cette tuile ICI, et a quel prix ?
//
//  Miroir de `PlacementValidator.cs` + `CoutConstruction.cs`. Cinq regles, un
//  seul objet : c'est le champ `regle` qui dit laquelle.
//
//    support      ce qui a le droit de se trouver SOUS la tuile
//    limite       combien au maximum, sur le plateau ou dans l'empire
//    gratuite     les N premieres sont offertes
//    batiments    il faut deja posseder N exemplaires d'un type
//    technologie  il faut avoir cherche telle techno, a tel niveau
//
//  ⚠️ CE N'EST PAS DU TEMPS. Rien ici ne s'integre : un verdict de pose se juge
//  a l'instant du clic.
//
//  ⚠️ UNE REGLE QUI NE DIT RIEN N'INTERDIT RIEN. Un support sans aucune tuile
//  cochee interdirait TOUT, PARTOUT : c'est une saisie inachevee cote site, pas
//  une regle de jeu. On le SIGNALE et on laisse passer.
// ============================================================

package moteur

import "fmt"

// ReglePose : une regle de pose, typee au chargement.
//
// ⚠️ EN JS C'ETAIT UN OBJET BRUT lu champ par champ (`r.regle`, `r.max`…), donc
// une faute de frappe dans le nom d'un champ passait inapercue. Ici la forme est
// fixee une fois, au chargement.
type ReglePose struct {
	Regle    string // "support" | "limite" | "gratuite" | "batiments" | "technologie"
	Base     string // support : "liste" | "tout"
	TileIds  []int
	Sauf     []int
	Max      int
	Portee   string // limite : "plateau" | "empire"
	Offerts  int
	Batiment int
	Nombre   int
	Techno   string
	Niveau   int
}

// Techno : ce que le placement en lit, et rien de plus. ⚠️ `entretien` et
// `effets` ne sont PAS ici : le moteur a cycles ne les porte pas (decision du
// 11/09, spec §8bis B). Quand l'entretien reviendra, par la SPEC d'abord, c'est
// le verrou ci-dessous qui se rebranchera.
type Techno struct {
	Code      string
	Brouillon bool
	Debloque  []int
}

func chargerReglePose(b Brut) ReglePose {
	r := ReglePose{
		Regle:    texte(b, "regle"),
		Base:     texte(b, "base"),
		TileIds:  entiers(b, "tileIds"),
		Sauf:     entiers(b, "sauf"),
		Max:      nombreOu(b, "max", 0),
		Portee:   texte(b, "portee"),
		Offerts:  nombreOu(b, "offerts", 0),
		Batiment: nombreOu(b, "batiment", 0),
		Nombre:   nombreOu(b, "nombre", 0),
		Techno:   texte(b, "techno"),
		Niveau:   nombreOu(b, "niveau", 0),
	}
	if r.Regle == "" {
		r.Regle = "support" // ⚠️ une regle vide EST un support, comme en JS
	}
	if r.Base == "" {
		r.Base = "liste"
	}
	if r.Portee == "" {
		r.Portee = "plateau"
	}
	return r
}

func ChargerTechno(b Brut) Techno {
	return Techno{
		Code:      texte(b, "code"),
		Brouillon: booleen(b, "brouillon"),
		Debloque:  entiers(b, "debloque"),
	}
}

// ─── La vue d'un plateau dont le placement a besoin ─────────────────────────
//
//  Deux encodages, une seule couche de jeu : le SOL dit ce qu'il y a sur chaque
//  case, les ETATS disent ou en est chaque batiment. Le « terrain » sous un
//  batiment, c'est simplement ce qui occupe la case avant la pose.

type Vue interface {
	TypePlateau() string
	Tid(x, z int) int
	Compter(tid int) int
	// ⚠️ UN CHANTIER NE COMPTE PAS comme un batiment possede : une regle « il te
	// faut 2 scieries » ne doit pas se satisfaire de deux trous. (`limite`, elle,
	// compte tout — un chantier occupe deja sa place.)
	CompterAcheves(tid int) int
}

// VueDuSol : depuis une liste `[{x, z, tid, chantier_fin}]` — ce que les
// vecteurs decrivent.
type VueDuSol struct {
	typ        string
	table      map[[2]int]int
	chantiers  []struct{ X, Z, Fin int }
	maintenant int
}

func NouvelleVueDuSol(sol []CaseSol, cases []Brut, typ string, maintenant int) *VueDuSol {
	v := &VueDuSol{typ: typ, table: map[[2]int]int{}, maintenant: maintenant}
	for _, c := range sol {
		v.table[[2]int{c.X, c.Z}] = c.Tid
	}
	for _, c := range cases {
		k := [2]int{nombreOu(c, "x", 0), nombreOu(c, "z", 0)}
		if _, y := v.table[k]; !y {
			v.table[k] = nombreOu(c, "tid", 0)
		}
		if f := nombreOu(c, "chantier_fin", 0); f > 0 {
			v.chantiers = append(v.chantiers, struct{ X, Z, Fin int }{k[0], k[1], f})
		}
	}
	return v
}

func (v *VueDuSol) TypePlateau() string { return v.typ }
func (v *VueDuSol) Tid(x, z int) int    { return v.table[[2]int{x, z}] }

func (v *VueDuSol) Compter(tid int) int {
	if tid == 0 {
		return 0
	}
	n := 0
	for _, t := range v.table {
		if t == tid {
			n++
		}
	}
	return n
}

func (v *VueDuSol) CompterAcheves(tid int) int {
	n := v.Compter(tid)
	for _, c := range v.chantiers {
		if c.Fin > v.maintenant && v.Tid(c.X, c.Z) == tid {
			n--
		}
	}
	if n < 0 {
		return 0
	}
	return n
}

func CompterEmpire(empire []Vue, tid int) int {
	n := 0
	for _, p := range empire {
		n += p.Compter(tid)
	}
	return n
}

// ─── Les regles ─────────────────────────────────────────────────────────────

// RegleUtile : le pendant exact du `regleUtile()` du site, pour que ce que
// l'ecran annonce comme « ignoree en jeu » le soit vraiment.
func RegleUtile(r *ReglePose) bool {
	switch r.Regle {
	case "gratuite":
		return r.Offerts > 0
	case "limite":
		return r.Max > 0
	case "batiments":
		return r.Batiment > 0 && r.Nombre > 0
	case "technologie":
		return r.Techno != ""
	}
	// support (regle vide comprise)
	if r.Base != "tout" {
		return len(r.TileIds) > 0
	}
	return len(r.Sauf) > 0
}

func contient(l []int, v int) bool {
	for _, x := range l {
		if x == v {
			return true
		}
	}
	return false
}

// AccepteLeSol — ⚠️ LA CASE VIDE (0) EST UNE VALEUR COCHABLE, pas un vide a
// ecarter : « se pose seulement sur une case vide » est la regle la plus
// courante du jeu.
func AccepteLeSol(r *ReglePose, tidDuSol int) bool {
	if r.Regle != "support" || !RegleUtile(r) {
		return true
	}
	if r.Base != "tout" {
		return contient(r.TileIds, tidDuSol)
	}
	return !contient(r.Sauf, tidDuSol)
}

// RefusHorsCase : les conditions qui **ne regardent pas la case**. Le magasin
// peut donc les poser AVANT que le joueur ait choisi ou construire (30/08).
//
// ⚠️ LA GRATUITE N'Y CHANGE RIEN : elle change le PRIX, pas les conditions. Un
// batiment offert dont le prerequis manque reste inconstructible.
func RefusHorsCase(r *ReglePose, v Vue, technos map[string]int) string {
	switch r.Regle {
	case "batiments":
		// ⚠️ On ne possede pas ce qui n'est pas fini : un chantier ne compte pas.
		possedes := v.CompterAcheves(r.Batiment)
		if possedes >= r.Nombre {
			return ""
		}
		return fmt.Sprintf("batiments:%d il en faut %d (tu en as %d)", r.Batiment, r.Nombre, possedes)
	case "technologie":
		acquis := technos[r.Techno]
		mini := r.Niveau
		if mini < 1 {
			mini = 1
		}
		if acquis >= mini {
			return ""
		}
		return fmt.Sprintf("technologie:%s niveau %d (acquis %d)", r.Techno, mini, acquis)
	}
	return ""
}

type Verdict struct {
	Blocages       []string
	Avertissements []string
}

// VerifierHorsCase : les conditions qui ne regardent PAS la case, jugees seules.
//
// ⚠️ C'est LA MEME fonction qui redige le refus pour les deux chemins
// (`RefusHorsCase`), pas une copie. Deux redactions du meme refus finissent
// toujours par diverger, et c'est alors le magasin qui ment.
func VerifierHorsCase(v Vue, catalogue map[int]*Tuile, tid int, technos map[string]int) Verdict {
	var out Verdict
	t := catalogue[tid]
	if tid == 0 || t == nil {
		return out
	}
	for i := range t.ReglesPose {
		r := &t.ReglesPose[i]
		if r.Regle != "batiments" && r.Regle != "technologie" {
			continue
		}
		if !RegleUtile(r) {
			out.Avertissements = append(out.Avertissements,
				fmt.Sprintf("regle « %s » incomplete, ignoree", r.Regle))
			continue
		}
		if refus := RefusHorsCase(r, v, technos); refus != "" {
			out.Blocages = append(out.Blocages, refus)
		}
	}
	return out
}

// ─── Le verrou de recherche ─────────────────────────────────────────────────
//
//  Un batiment cite dans le `debloque` d'au moins une techno (hors brouillon)
//  est VERROUILLE tant qu'aucune d'elles n'est acquise.
//
//  ⚠️ 11/09 — « ET EN MARCHE » A DISPARU D'ICI, avec l'entretien des technos :
//  le moteur a cycles ne le porte pas. Une techno est donc acquise ou pas, il
//  n'y a plus de troisieme etat. C'est ce qui tient V3 ouvert dans les vecteurs
//  (spec §8bis B) — et c'est ici que la veille se rebranchera.
//
//  ⚠️ CATALOGUE VIDE = PAS DE VERROU : les tests et le hors-ligne ne doivent pas
//  interdire tout le magasin.

func Verrouilleurs(technosCat []Techno, tid int) []Techno {
	var l []Techno
	for _, t := range technosCat {
		if !t.Brouillon && contient(t.Debloque, tid) {
			l = append(l, t)
		}
	}
	return l
}

func RaisonVerrou(technosCat []Techno, tid int, acquises map[string]int) string {
	verrous := Verrouilleurs(technosCat, tid)
	if len(verrous) == 0 {
		return ""
	}
	for _, t := range verrous {
		if acquises[t.Code] >= 1 { // « acquise » = niveau >= 1
			return ""
		}
	}
	return fmt.Sprintf("a debloquer par la recherche « %s »", verrous[0].Code)
}

// Verifier : aucun blocage = c'est permis.
func Verifier(v Vue, catalogue map[int]*Tuile, x, z, tid int, technos map[string]int, empire []Vue) Verdict {
	var out Verdict
	if tid == 0 {
		return out
	}
	t := catalogue[tid]
	if t == nil {
		out.Blocages = append(out.Blocages, fmt.Sprintf("type %d absent du catalogue", tid))
		return out
	}

	// ⚠️ Tolerant quand l'un des deux est vide : les catalogues de test et les
	// vieilles donnees n'ont pas toujours ce champ, et refuser sur une
	// information ABSENTE rendrait des tuiles impossibles a poser sans que
	// personne comprenne pourquoi.
	if t.TypeOfPlateau != "" && v.TypePlateau() != "" && t.TypeOfPlateau != v.TypePlateau() {
		out.Blocages = append(out.Blocages,
			fmt.Sprintf("tuile « %s » sur un plateau « %s »", t.TypeOfPlateau, v.TypePlateau()))
		return out
	}

	for i := range t.ReglesPose {
		r := &t.ReglesPose[i]
		if r.Regle == "" {
			continue
		}
		if !RegleUtile(r) {
			out.Avertissements = append(out.Avertissements,
				fmt.Sprintf("regle « %s » incomplete, ignoree", r.Regle))
			continue
		}

		switch r.Regle {
		case "support":
			if sol := v.Tid(x, z); !AccepteLeSol(r, sol) {
				out.Blocages = append(out.Blocages,
					fmt.Sprintf("support refuse — ici c'est %d", sol))
			}

		case "limite":
			veutEmpire := r.Portee == "empire"
			aEmpire := veutEmpire && len(empire) > 0
			if veutEmpire && !aEmpire {
				// Une limite trop stricte se voit, une limite muette ne se voit
				// jamais : on se rabat sur le plateau EN LE DISANT.
				out.Avertissements = append(out.Avertissements,
					"limite « empire » sans les autres plateaux : appliquee AU PLATEAU")
			}
			deja := v.Compter(tid)
			if aEmpire {
				deja = CompterEmpire(empire, tid)
			}
			// La case visee compte deja pour l'occupant qu'elle porte :
			// remplacer un exemplaire par un autre du meme type ne doit pas
			// buter sur la limite.
			if v.Tid(x, z) == tid {
				deja--
			}
			if deja >= r.Max {
				out.Blocages = append(out.Blocages,
					fmt.Sprintf("limite %d atteinte (tu en as %d)", r.Max, deja))
			}

		case "batiments", "technologie":
			if refus := RefusHorsCase(r, v, technos); refus != "" {
				out.Blocages = append(out.Blocages, refus)
			}

		case "gratuite":
			// Change le PRIX, pas les conditions. Rien a faire ici — et surtout
			// pas un avertissement, la regle est parfaitement valide.

		default:
			out.Avertissements = append(out.Avertissements,
				fmt.Sprintf("regle inconnue « %s », ignoree", r.Regle))
		}
	}
	return out
}

// Devis : ce qu'il faut payer, et pourquoi. `Interdiction` non vide = pas de
// prix a annoncer, la pose sera refusee de toute facon.
type Devis struct {
	Lignes       []Cout
	Offert       bool
	Offerts      int
	Deja         int
	Interdiction string
}

func Estimer(v Vue, catalogue map[int]*Tuile, tid, niveau int, empire []Vue,
	technosCat []Techno, acquises map[string]int) Devis {

	var devis Devis
	t := catalogue[tid]
	if tid == 0 || t == nil {
		return devis
	}
	if niveau < 1 {
		niveau = 1
	}
	devis.Lignes = append([]Cout(nil), t.Cout...)

	offerts, plafond, plafondEmpire := 0, 0, 0
	for i := range t.ReglesPose {
		r := &t.ReglesPose[i]
		if !RegleUtile(r) {
			continue
		}
		if r.Regle == "gratuite" && r.Offerts > offerts {
			offerts = r.Offerts
		}
		if r.Regle == "limite" {
			if r.Portee == "empire" {
				if plafondEmpire == 0 || r.Max < plafondEmpire {
					plafondEmpire = r.Max
				}
			} else if plafond == 0 || r.Max < plafond {
				plafond = r.Max
			}
		}
	}
	devis.Offerts = offerts
	devis.Deja = v.Compter(tid)

	// LE VERROU DE RECHERCHE, AVANT MEME LE PLAFOND : une carte verrouillee n'a
	// pas de prix a annoncer. Pose seulement (niveau 1) — une techno qui
	// s'endort met ses batiments « inconstructibles », elle n'arrete pas
	// d'ameliorer ceux qui sont deja debout.
	if niveau == 1 && len(technosCat) > 0 {
		if verrou := RaisonVerrou(technosCat, tid, acquises); verrou != "" {
			devis.Interdiction = verrou
			return devis
		}
	}

	// Le PLAFOND est verifie avant tout le reste : sinon on annoncerait un prix
	// pour une pose qui sera refusee.
	if niveau == 1 && plafond > 0 && devis.Deja >= plafond {
		devis.Interdiction = fmt.Sprintf("pas plus de %d sur ce plateau (tu en as %d)",
			plafond, devis.Deja)
		return devis
	}
	if niveau == 1 && plafondEmpire > 0 {
		dejaE := devis.Deja
		if len(empire) > 0 {
			dejaE = CompterEmpire(empire, tid)
		}
		if dejaE >= plafondEmpire {
			devis.Interdiction = fmt.Sprintf("pas plus de %d dans l'empire (tu en as %d)",
				plafondEmpire, dejaE)
			return devis
		}
	}

	// Ameliorer se paie TOUJOURS : la gratuite ne concerne que la POSE.
	if niveau > 1 || offerts <= 0 || devis.Deja >= offerts {
		return devis
	}

	devis.Offert = true
	// ⚠️ OFFERT NE VEUT PAS DIRE « SANS PERSONNEL ». On retire les lignes `paye`
	// — ce qui se prelevait — et on GARDE les lignes `mobilise` : une habitation
	// offerte a toujours besoin de ses habitants. L'ancien modele vidait tout,
	// ce qui donnait des batiments cadeaux qui tournaient sans main-d'oeuvre.
	var gardees []Cout
	for _, l := range devis.Lignes {
		if l.Mode == "mobilise" && l.Quantite > 0 && l.Ressource != CodeInconnu {
			gardees = append(gardees, l)
		}
	}
	devis.Lignes = gardees
	return devis
}

// ─── La vue d'une VRAIE partie (le tableau d'octets) ────────────────────────

// VueDunePartie : le cas reel — le sol EST le tableau d'octets du plateau.
//
// ⚠️⚠️ ON COMPTE SUR LES 10 000 OCTETS, PAS SUR LES CASES BATIES. Une limite
// « pas plus de trois » doit voir les trois, etat ou pas — c'est le pendant,
// cote pose, du piege du 31/08 sur les proximites.
type VueDunePartieT struct{ p *Partie }

func VueDunePartie(p *Partie) Vue { return &VueDunePartieT{p: p} }

func (v *VueDunePartieT) TypePlateau() string { return v.p.TypeOfPlateau }

func (v *VueDunePartieT) Tid(x, z int) int {
	i := IndexCase(v.p.Largeur, v.p.Hauteur, x, z)
	if i < 0 || i >= len(v.p.Tiles) {
		return 0
	}
	return v.p.Tiles[i]
}

func (v *VueDunePartieT) Compter(tid int) int {
	if tid == 0 {
		return 0
	}
	n := 0
	for _, t := range v.p.Tiles {
		if t == tid {
			n++
		}
	}
	return n
}

// CompterAcheves — ⚠️ UN CHANTIER NE COMPTE PAS comme un batiment possede : une
// regle « il te faut 2 scieries » ne doit pas se satisfaire de deux trous.
// (`limite`, elle, compte tout — un chantier occupe deja sa place.)
func (v *VueDunePartieT) CompterAcheves(tid int) int {
	n := v.Compter(tid)
	t := v.p.Plateau.T
	for _, b := range v.p.Plateau.Batiments {
		if b.EnChantier(t) && v.Tid(b.X, b.Z) == tid {
			n--
		}
	}
	if n < 0 {
		return 0
	}
	return n
}
