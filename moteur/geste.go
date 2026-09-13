// ============================================================
//  moteur/geste.go — POSER, DETRUIRE  (moteur a cycles)
//
//  UN GESTE, C'EST TROIS TEMPS, TOUJOURS DANS CET ORDRE : on juge, on paie, on
//  ecrit. L'ordre n'est pas du style — c'est lui qui garantit qu'on ne paie
//  jamais pour un non-evenement.
//
//  ⚠️ CE N'EST PAS DU TEMPS. Comme le placement, un geste se juge a l'instant du
//  clic. Ce qu'il LAISSE, en revanche, est du temps — un chantier, qui rend la
//  case INERTE jusqu'a `ChantierFin` (§6bis).
//
//  ⚠️ CE MODULE MUTE. Tout le reste du moteur lit ; ici on ecrit dans le sol et
//  dans la liste des batiments. C'est pour ca que l'appelant lui donne un
//  TERRAIN et pas une vue : une vue de placement est en lecture seule, expres.
//
//  ⚠️ LE PRIX SORT DES COFFRES QUI ENVOIENT, jamais d'un tresor global — il n'y
//  en a pas —, et un `FluxStock` sort de la reserve du plateau, par les memes
//  `Lire` / `Retirer` que tout le reste : « il n'y a pas de chemin parallele ».
//
//  ⚠️ LE NON-EVENEMENT SE JUGE AVANT DE PAYER (11/09). L'ancien code payait puis
//  remboursait si la case portait deja ca ; un remboursement qui echoue (coffre
//  plein entre-temps) fait fondre les ressources du joueur en silence.
//
//  ⚠️ NON PORTE : `ameliorer` (niveau > 1). A dire tant que ce n'est pas fait.
// ============================================================

package moteur

import "fmt"

// Terrain : une vue de placement, mais qui accepte d'etre modifiee.
type Terrain interface {
	Vue
	DansLePlateau(x, z int) bool
	Mettre(x, z, tid int)
	Plateau() *Plateau
	BatimentAt(x, z int) *Batiment
}

// TerrainDesCases : depuis une liste `[{x, z, tid}]` — ce que les vecteurs
// decrivent. Le plateau du moteur est fourni par l'appelant, deja monte.
type TerrainDesCases struct {
	typ     string
	table   map[[2]int]int
	plateau *Plateau
}

func NouveauTerrainDesCases(sol []CaseSol, p *Plateau, typ string) *TerrainDesCases {
	t := &TerrainDesCases{typ: typ, table: map[[2]int]int{}, plateau: p}
	for _, c := range sol {
		t.table[[2]int{c.X, c.Z}] = c.Tid
	}
	for _, b := range p.Batiments {
		k := [2]int{b.X, b.Z}
		if _, y := t.table[k]; !y {
			t.table[k] = b.Tuile.Tid
		}
	}
	return t
}

func (t *TerrainDesCases) TypePlateau() string           { return t.typ }
func (t *TerrainDesCases) Tid(x, z int) int              { return t.table[[2]int{x, z}] }
func (t *TerrainDesCases) DansLePlateau(x, z int) bool   { return true }
func (t *TerrainDesCases) Mettre(x, z, tid int)          { t.table[[2]int{x, z}] = tid }
func (t *TerrainDesCases) Plateau() *Plateau             { return t.plateau }
func (t *TerrainDesCases) BatimentAt(x, z int) *Batiment { return t.plateau.BatimentEn(x, z) }

func (t *TerrainDesCases) Compter(tid int) int {
	if tid == 0 {
		return 0
	}
	n := 0
	for _, v := range t.table {
		if v == tid {
			n++
		}
	}
	return n
}

// CompterAcheves — ⚠️ un chantier ne compte pas comme un batiment possede.
func (t *TerrainDesCases) CompterAcheves(tid int) int {
	n := t.Compter(tid)
	for _, b := range t.plateau.Batiments {
		if b.EnChantier(t.plateau.T) && t.Tid(b.X, b.Z) == tid {
			n--
		}
	}
	if n < 0 {
		return 0
	}
	return n
}

// ─── Ce qui merite une ligne json ───────────────────────────────────────────

// PorteUnEtat : miroir de `TuileRecord.PorteUnEtat`. Une case ne prend un etat
// que si elle a quelque chose a RETENIR.
//
// ⚠️ NE PAS DEDUIRE CA DE « elle a un palier » : le site en donne un a toute
// tuile, herbe comprise. Un plateau de 10 000 cases ferait 700 Ko de json dont
// 95 % de repetitions du meme objet vide.
func PorteUnEtat(t *Tuile) bool {
	if t == nil {
		return false
	}
	if t.Stocke() || len(t.Appros) > 0 {
		return true // coffre, recolte, envoi
	}
	if t.NbNiveaux > 1 || t.DureeChantier > 0 {
		return true
	}
	// Un cycle A RETENIR : `TCycle` et `EnMarche` n'existent que la.
	if t.DureeCycleS > 0 && (len(t.Production) > 0 || len(t.Utilisation) > 0) {
		return true
	}
	// Mobiliser, c'est pouvoir etre mis en veille — donc un etat a retenir.
	for _, c := range t.Cout {
		if c.Mode == "mobilise" && c.Quantite > 0 && c.Ressource != CodeInconnu {
			return true
		}
	}
	return false
}

// ─── Les refus MECANIQUES (ni prix, ni regles de pose) ──────────────────────

func RefusDePose(terrain Terrain, catalogue map[int]*Tuile, x, z, tid int) string {
	if !terrain.DansLePlateau(x, z) {
		return fmt.Sprintf("(%d,%d) est hors du plateau", x, z)
	}
	// Ecrire un id inconnu donnerait une case definitivement vide a l'ecran,
	// sans que rien ne le dise a la sauvegarde.
	if tid != 0 && catalogue[tid] == nil {
		return fmt.Sprintf("le type %d n'est pas dans le catalogue", tid)
	}
	occupant := terrain.Tid(x, z)
	to := catalogue[occupant]
	if occupant != tid && to != nil && to.NonRemplacable {
		return fmt.Sprintf("« %s » en place est figee : on ne peut rien poser a sa place", to.Nom)
	}
	if occupant == tid {
		return "rien n'a change sur cette case"
	}
	return ""
}

func RefusDeRetrait(terrain Terrain, catalogue map[int]*Tuile, x, z int) string {
	if !terrain.DansLePlateau(x, z) {
		return fmt.Sprintf("(%d,%d) est hors du plateau", x, z)
	}
	ancien := terrain.Tid(x, z)
	if ancien == 0 {
		return "la case est deja vide"
	}
	if t := catalogue[ancien]; t != nil && t.Indestructible {
		return fmt.Sprintf("« %s » ne peut pas etre detruit", t.Nom)
	}
	return ""
}

// ─── La mutation nue ────────────────────────────────────────────────────────

// ecrire : retire ce qui etait la, pose le nouveau. ⚠️ `Detruire` emporte les
// navettes de la case avec elle (elles sont imbriquees, §10) et rend au stock
// commun ce qui peut l'etre.
func ecrire(terrain Terrain, x, z, nouveau, maintenant int, neuf *Batiment) Sac {
	p := terrain.Plateau()
	perdu := NouveauSac(p.Genres.Reg)
	if ancien := terrain.BatimentAt(x, z); ancien != nil {
		_, perdu = Detruire(p, ancien, maintenant)
	}
	terrain.Mettre(x, z, nouveau)
	if neuf != nil {
		p.Batiments = append(p.Batiments, neuf)
	}
	// ⚠️ LE TERRAIN A CHANGE : les caches de passe (ordre, geometrie, proximite)
	// ne valent plus rien. Une passe n'y touche pas ; un geste, si.
	p.ViderCaches()
	return perdu
}

// BatimentNeuf : le batiment neuf d'une pose — jamais ajoute au plateau avant
// d'avoir paye.
func BatimentNeuf(g *Genres, catalogue map[int]*Tuile, x, z, tid, maintenant int) *Batiment {
	t := catalogue[tid]
	if t == nil || !PorteUnEtat(t) {
		return nil
	}
	return CreerBatiment(g, Brut{"x": float64(x), "z": float64(z), "niveau": 1.0,
		"actif": true, "t_cycle": float64(maintenant), "en_marche": false}, t)
}

// ─── Payer la pose (§6bis) ──────────────────────────────────────────────────

type ManqueLigne struct {
	Ressource Code
	Manque    int
	Libre     bool
}

type ComptePose struct {
	Paye       bool
	Manque     []ManqueLigne
	Depense    Sac
	Immobilise Sac
}

// PayerLaPose — ⚠️ LE COUT EST PRELEVE AU MOMENT DU GESTE, PAS A LA FIN DU
// CHANTIER. Le joueur pose, il paie, et le batiment entre en chantier — inerte
// jusqu'a `ChantierFin`.
//
// `lignes` : le DEVIS (`Estimer`), pas `b.Tuile.Cout` — une pose offerte ne
// garde que ses lignes `mobilise` (OFFERT NE VEUT PAS DIRE « SANS PERSONNEL »).
//
// ⚠️⚠️ LE DEVIS EST LA SEULE VERITE, MEME VIDE — ET C'EST TOUT LE BUG DU 13/09.
// Il y avait ici un repli « `lignes == nil` -> `b.Tuile.Cout` ». Or `Estimer`
// construit la liste offerte par `append` sur une tranche nulle : une tuile
// OFFERTE qui n'a AUCUNE ligne `mobilise` (une habitation payee en bois, et
// rien d'autre) rendait `Lignes == nil`. Le repli relisait alors le cout PLEIN
// de la tuile, et le cadeau se facturait — « il manque 150 bois » sur une carte
// qui annonce OFFERT. `nil` et `[]` disent la MEME chose ici : rien a payer.
// Ne jamais redonner a `nil` un second sens.
//
// ⚠️ DEUX BOURSES, DEUX REGLES. Ce qui se PAIE se compare au DISPONIBLE et en
// sort par `Retirer`. Ce qui se MOBILISE se compare au LIBRE et n'est JAMAIS
// preleve : il est immobilise tant que le batiment tourne, parce que
// `Mobilise()` le compte, et rendu par la veille parce qu'il cesse de le
// compter. Un indicateur ne se paie pas.
//
// ⚠️ `b` NE DOIT PAS ENCORE ETRE DANS LE PLATEAU : son propre cout `mobilise` se
// compterait sinon deux fois.
func PayerLaPose(p *Plateau, b *Batiment, t int, lignes []Cout) ComptePose {
	g := p.Genres
	dispo := Disponible(p, t)
	occupe := Mobilise(p, t)
	out := ComptePose{Depense: NouveauSac(g.Reg), Immobilise: NouveauSac(g.Reg)}
	aPayer := NouveauSac(g.Reg)

	for _, l := range lignes {
		if l.Ressource == CodeInconnu || l.Quantite <= 0 || g.EstIndicateur(l.Ressource) {
			continue
		}
		if l.Mode == "mobilise" {
			lib := Libre(dispo, occupe, l.Ressource)
			if lib < l.Quantite {
				manque := l.Quantite
				if lib > 0 {
					manque = l.Quantite - lib
				}
				out.Manque = append(out.Manque, ManqueLigne{l.Ressource, manque, true})
			} else {
				out.Immobilise.Ajouter(l.Ressource, l.Quantite)
			}
			continue
		}
		aPayer.Ajouter(l.Ressource, l.Quantite)
	}
	for _, c := range aPayer.NonNuls() {
		if dispo.Get(c) < aPayer.Get(c) {
			out.Manque = append(out.Manque, ManqueLigne{c, aPayer.Get(c) - dispo.Get(c), false})
		}
	}
	// On verifie TOUT avant de prelever QUOI QUE CE SOIT.
	if len(out.Manque) > 0 {
		return out
	}
	if ok, _ := Debiter(p, aPayer, t); !ok {
		out.Manque = append(out.Manque, ManqueLigne{CodeInconnu, 0, false})
		return out
	}
	if b.Tuile.DureeChantier > 0 {
		fin := t + b.Tuile.DureeChantier
		b.ChantierFin = &fin
	}
	out.Paye = true
	out.Depense = aPayer
	return out
}

// MettreEnVeille : ce qui etait `mobilise` est rendu INTEGRALEMENT, et sans
// rien deplacer — `Mobilise()` ne compte que les batiments vivants, un batiment
// eteint cesse simplement d'etre compte. Ce qui etait `paye` est perdu.
func MettreEnVeille(b *Batiment, t int) {
	b.Actif = false
	b.EnMarche = false
	b.TCycle = t
}

// ─── Poser ──────────────────────────────────────────────────────────────────

type OptionsGeste struct {
	Technos          map[string]int
	CatalogueTechnos []Techno
	Empire           []Vue
}

type ResultatPose struct {
	Ok             bool
	Refus          string
	Blocages       []string
	Avertissements []string
	Paye           Sac
	Mobilise       Sac
	Perdu          Sac
	Offert         bool
	Chantier       int
}

// Poser : juge, paie, ecrit. Ne leve jamais — rend un compte rendu.
func Poser(terrain Terrain, catalogue map[int]*Tuile, x, z, tid, maintenant int, o OptionsGeste) ResultatPose {
	p := terrain.Plateau()
	res := ResultatPose{Paye: NouveauSac(p.Genres.Reg), Mobilise: NouveauSac(p.Genres.Reg),
		Perdu: NouveauSac(p.Genres.Reg)}

	if refus := RefusDePose(terrain, catalogue, x, z, tid); refus != "" {
		res.Refus = refus
		return res
	}

	v := Verifier(terrain, catalogue, x, z, tid, o.Technos, o.Empire)
	res.Blocages = append(res.Blocages, v.Blocages...)
	res.Avertissements = append(res.Avertissements, v.Avertissements...)
	if len(res.Blocages) > 0 {
		res.Refus = joindre(res.Blocages)
		return res
	}

	devis := Estimer(terrain, catalogue, tid, 1, o.Empire, o.CatalogueTechnos, o.Technos)
	if devis.Interdiction != "" {
		res.Refus = devis.Interdiction
		return res
	}
	res.Offert = devis.Offert

	// ⚠️ LE BATIMENT NEUF N'EST PAS ENCORE DANS LE PLATEAU quand on paie : son
	// propre cout `mobilise` se compterait sinon deux fois, et il n'a de toute
	// facon rien dans son coffre.
	neuf := BatimentNeuf(p.Genres, catalogue, x, z, tid, maintenant)
	cible := neuf
	if cible == nil {
		cible = CreerBatiment(p.Genres, Brut{"x": float64(x), "z": float64(z)}, catalogue[tid])
	}
	compte := PayerLaPose(p, cible, maintenant, devis.Lignes)
	if !compte.Paye {
		for _, m := range compte.Manque {
			suffixe := ""
			if m.Libre {
				suffixe = " libre(s)"
			}
			res.Blocages = append(res.Blocages,
				fmt.Sprintf("il manque %d %s%s", m.Manque, p.Genres.Reg.Nom(m.Ressource), suffixe))
		}
		res.Refus = joindre(res.Blocages)
		return res
	}

	res.Perdu = ecrire(terrain, x, z, tid, maintenant, neuf)
	res.Paye = compte.Depense
	res.Mobilise = compte.Immobilise
	if neuf != nil && neuf.ChantierFin != nil {
		res.Chantier = *neuf.ChantierFin
	}
	res.Ok = true
	if len(res.Perdu.NonNuls()) > 0 {
		res.Avertissements = append(res.Avertissements,
			"la case portait deja un batiment : son contenu est perdu")
	}
	return res
}

func joindre(l []string) string {
	out := ""
	for i, s := range l {
		if i > 0 {
			out += " ; "
		}
		out += s
	}
	return out
}

// ─── Detruire ───────────────────────────────────────────────────────────────

type ResultatDestruction struct {
	Ok             bool
	Refus          string
	Avertissements []string
	Perdu          Sac
	Rendu          Sac
	Ancien, Apres  int
}

// DetruireCase : la case ne devient pas vide — elle prend le `apres_destruction`
// declare par le type detruit (0 = vide).
//
// ⚠️ Detruire en plein chantier ne rembourse rien (§6bis), et ce qui etait dans
// le coffre local est PERDU — sauf pour un stock commun, ou la marchandise
// revient aux membres restants et seul le SURPLUS est ecrete (§7).
func DetruireCase(terrain Terrain, catalogue map[int]*Tuile, x, z, maintenant int) ResultatDestruction {
	p := terrain.Plateau()
	res := ResultatDestruction{Perdu: NouveauSac(p.Genres.Reg), Rendu: NouveauSac(p.Genres.Reg)}

	if refus := RefusDeRetrait(terrain, catalogue, x, z); refus != "" {
		res.Refus = refus
		return res
	}

	ancien := terrain.Tid(x, z)
	res.Ancien = ancien

	apres := 0
	if t := catalogue[ancien]; t != nil {
		apres = t.ApresDestruction
	}
	// Une tuile qui se redonne elle-meme serait indestructible : erreur de
	// saisie cote site, pas un cas de jeu. On vide, et on le dit.
	if apres == ancien && apres != 0 {
		res.Avertissements = append(res.Avertissements,
			fmt.Sprintf("le type %d se redonne lui-meme apres destruction : la case est videe", ancien))
		apres = 0
	}
	// Un residu inconnu laisserait une case invisible et non reconstructible.
	if apres != 0 && catalogue[apres] == nil {
		res.Avertissements = append(res.Avertissements,
			fmt.Sprintf("le residu %d du type %d n'est pas au catalogue : la case est videe", apres, ancien))
		apres = 0
	}
	res.Apres = apres

	if batiment := terrain.BatimentAt(x, z); batiment != nil {
		res.Rendu, res.Perdu = Detruire(p, batiment, maintenant)
	}
	terrain.Mettre(x, z, apres)
	if neuf := BatimentNeuf(p.Genres, catalogue, x, z, apres, maintenant); neuf != nil {
		p.Batiments = append(p.Batiments, neuf)
	}
	p.ViderCaches()

	res.Ok = true
	if perdus := res.Perdu.NonNuls(); len(perdus) > 0 {
		dit := ""
		for i, c := range perdus {
			if i > 0 {
				dit += ", "
			}
			dit += fmt.Sprintf("%d %s", res.Perdu.Get(c), p.Genres.Reg.Nom(c))
		}
		res.Avertissements = append(res.Avertissements, "perdu a la destruction : "+dit)
	}
	return res
}

// ─── Le terrain d'une VRAIE partie ──────────────────────────────────────────

// TerrainDunePartie : sur un vrai plateau, le sol EST le tableau d'octets de la
// partie, et on y ECRIT. La reserve et les coffres vivent dans `partie.Plateau`,
// mutes en place — la route les reecrit ensuite.
type TerrainDunePartieT struct {
	VueDunePartieT
	p *Partie
}

func TerrainDunePartie(p *Partie) Terrain {
	return &TerrainDunePartieT{VueDunePartieT: VueDunePartieT{p: p}, p: p}
}

func (t *TerrainDunePartieT) DansLePlateau(x, z int) bool {
	i := IndexCase(t.p.Largeur, t.p.Hauteur, x, z)
	return i >= 0 && i < len(t.p.Tiles)
}

func (t *TerrainDunePartieT) Mettre(x, z, tid int) {
	if i := IndexCase(t.p.Largeur, t.p.Hauteur, x, z); i >= 0 && i < len(t.p.Tiles) {
		t.p.Tiles[i] = tid
	}
}

func (t *TerrainDunePartieT) Plateau() *Plateau { return t.p.Plateau }

func (t *TerrainDunePartieT) BatimentAt(x, z int) *Batiment {
	return t.p.Plateau.BatimentEn(x, z)
}
