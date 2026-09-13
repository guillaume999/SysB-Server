// ============================================================
//  moteur/indice.go — LA SATISFACTION ET L'ESCALIER (§5)
//
//      satisfaction = servi / demande_au_max,  exprime EN POUR CENT
//                   + les lignes BONUS, qui ajoutent sans etre demandees
//
//  ⚠️⚠️ C'EST UN RAPPORT RECALCULE, PAS UNE DONNEE STOCKEE.
//
//  ⚠️⚠️ ELLE PEUT DEPASSER 100 (13/09). Une ligne de consommation peut porter
//  un `bonus` : elle N'ENTRE PAS dans la demande de base et AJOUTE son
//  pourcentage au prorata de ce qu'elle recoit. 10 nourriture servies + 5
//  gibier servis, bonus 20 -> 120 %. C'est la seule facon de depasser 100, et
//  ce n'est PAS le retour de `part` : les lignes ordinaires ne declarent
//  toujours rien.
//
//  ⚠️ CE SURPLUS NE SERT QUE PAR L'ESCALIER. Une production ORDINAIRE reste
//  plafonnee a sa quantite declaree (`ConsoProd`) ; pour que 120 % paie, il
//  faut une tranche au-dessus de 100 — « de 120 -> 130 % ». Aucun plafond
//  n'est impose : c'est le catalogue qui borne, en ne saisissant pas de
//  tranche plus haut.
//
//  ⚠️⚠️ AUCUN ARRONDI N'ENTRE DANS LE CALCUL. On ne calcule JAMAIS le
//  pourcentage pour le comparer ensuite a un seuil — ON MULTIPLIE EN CROIX :
//
//      ✗  satisfaction = arrondi(100 x servi / demande)   puis   >= seuil
//      ✓  servi x 100 >= seuil x demande
//
//  ⚠️ Go tient ces entiers dans de vrais entiers 64 bits la ou le JS avait des
//  flottants (exacts jusqu'a 2^53). Tant que le pgcd fait son travail les deux
//  coincident ; au-dela, le JS perdrait de la precision en silence la ou Go
//  deborderait franchement.
// ============================================================

package moteur

func pgcd(a, b int) int {
	if a < 0 {
		a = -a
	}
	if b < 0 {
		b = -b
	}
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// Rapport : l'accumulateur exact.
//
// Un batiment peut consommer plusieurs ressources. Sa satisfaction est la
// moyenne des rapports de ses lignes, PONDEREE PAR CE QUE CHAQUE LIGNE
// DEMANDE :
//
//	satisfaction = SOMME( (s_l / D_l) x p_l ) / SOMME( p_l )
//
// Pour une ligne ordinaire, le rapport est deja celui de la ligne (`p_l = D_l`)
// et tout se simplifie en SOMME(servi)/SOMME(demande) — le cas courant. Pour
// une ligne « en direct », le rapport est celui du GROUPE (§4) alors que le
// poids reste la demande PROPRE du batiment : les deux different, et c'est la
// qu'un arrondi entrerait si on n'y prenait pas garde.
//
// ⚠️ `BN/BD` porte a part la somme des BONUS, en fraction de 1 (un bonus de
// 20 pleinement servi vaut 20/100). Elle s'ajoute a la base a la toute fin —
// deux fractions exactes, jamais un pourcentage arrondi en chemin.
type Rapport struct {
	Num, P, W int
	BN, BD    int
}

func CreerRapport() Rapport { return Rapport{Num: 0, P: 1, W: 0, BN: 0, BD: 1} }

// Ajouter une ligne de rapport `s / dl`, pesant `p`.
// Une ligne qui ne demande rien (`dl <= 0`) n'entre pas : elle n'a pas d'avis.
func (acc *Rapport) Ajouter(s, dl, p int) {
	if dl <= 0 || p <= 0 {
		return
	}
	if p == dl {
		// Le cas courant : le poids EST la demande, rien a multiplier.
		acc.Num += s * acc.P
		acc.W += p
		return
	}
	acc.Num = acc.Num*dl + s*p*acc.P
	acc.P = acc.P * dl
	acc.W = acc.W + p
	if g := pgcd(acc.Num, acc.P); g > 1 {
		acc.Num /= g
		acc.P /= g
	}
}

// AjouterBonus : une ligne BONUS, servie `s` sur `dl` demandes, valant `bonus`
// pour cent (13/09).
//
// ⚠️⚠️ ELLE N'ENTRE PAS DANS LA DEMANDE DE BASE — c'est toute la difference
// avec `Ajouter`, et c'est ce qui permet de depasser 100. Une ligne bonus ne
// tire donc JAMAIS la satisfaction vers le bas : au pire elle n'ajoute rien.
//
//	BN/BD += (bonus x s) / (100 x dl)
func (acc *Rapport) AjouterBonus(s, dl, bonus int) {
	if dl <= 0 || bonus <= 0 || s <= 0 {
		return
	}
	d := 100 * dl
	acc.BN = acc.BN*d + bonus*s*acc.BD
	acc.BD = acc.BD * d
	if g := pgcd(acc.BN, acc.BD); g > 1 {
		acc.BN /= g
		acc.BD /= g
	}
}

// base : la fraction des seules lignes ORDINAIRES.
//
// ⚠️ Aucune ligne du tout -> (0, 0), et l'appelant en fait « pas de votant »,
// comme avant le bonus. Mais QUE des lignes bonus -> (1, 1) : le batiment ne
// demande rien d'essentiel, il est donc parfaitement servi, et son bonus
// s'ajoute a 100 au lieu de se perdre.
func (acc Rapport) base() (num, den int) {
	if acc.W <= 0 {
		if acc.BN == 0 {
			return 0, 0
		}
		return 1, 1
	}
	return acc.Num, acc.P * acc.W
}

// Le couple d'entiers (servi, demande) — c'est lui qu'on compare en croix.
// ⚠️ Sans aucun bonus, `BN = 0` et `BD = 1` : on retombe EXACTEMENT sur
// (Num, P x W), l'ecriture d'avant le 13/09.
func (acc Rapport) Servi() int {
	n, d := acc.base()
	return n*acc.BD + acc.BN*d
}

func (acc Rapport) Demande() int {
	_, d := acc.base()
	return d * acc.BD
}

// ─── L'escalier ─────────────────────────────────────────────────────────────

// Tranche : quelle tranche pour un rapport `servi / demande` ?
//
// ⚠️ MULTIPLICATION EN CROIX, jamais un pourcentage calcule puis compare.
// ⚠️⚠️ UN BATIMENT QUI NE DEMANDE RIEN VAUT 100 %, PAS LA TRANCHE DU HAUT
// (corrige le 13/09). Tant que rien ne montait au-dessus de 100 les deux se
// confondaient ; depuis les bonus, « tranche du haut » lui offrirait le bonus
// maximal sans qu'il consomme quoi que ce soit.
// ⚠️ LA FALAISE N'EST PAS UN BUG : 79 % -> 60 %, 80 % -> 100 %. C'est la nature
//
//	d'un escalier a marches. Si elle gene, c'est le NOMBRE de tranches qu'il
//	faut augmenter (§5).
//
// ⚠️ `l.Tranches` EST DEJA TRIEE (seuil decroissant) depuis le chargement. Le JS
// la retriait ici, a chaque appel.
func Tranche(l *Ligne, s, d int) int {
	if len(l.Tranches) == 0 {
		return 100
	}
	if d <= 0 {
		s, d = 1, 1
	}
	for _, p := range l.Tranches {
		if s*100 >= p[0]*d {
			return p[1]
		}
	}
	return l.Tranches[len(l.Tranches)-1][1]
}

// RendementEscalier — LECTURE B : chacun sa tranche, moyenne PONDEREE PAR LA
// POPULATION.
//
//	rendement = SOMME( population_i x tranche(satisfaction_i) ) / SOMME( population_i )
//
// ⚠️ Ce n'est PAS `tranche(moyenne)`. Les mal servis tirent le rendement vers
// le bas SUR LEUR PART DE POPULATION SEULEMENT ; prendre la moyenne d'abord
// lisserait une famine locale en un chiffre confortable.
//
// ⚠️ Qui vote : tout batiment VIVANT qui DEMANDE quelque chose.
// ⚠️ Personne ne vote -> 100 %. Un plateau sans consommateur n'est pas affame.
func RendementEscalier(p *Plateau, l *Ligne, maintenant int) int {
	if l.Indicateur == "" {
		return 100
	}
	pop, acc := 0, 0
	for _, b := range p.Ordonnes() {
		if !b.Vivant(maintenant) || b.Demande <= 0 {
			continue
		}
		n := b.Tuile.PlacesLogees()
		if n <= 0 {
			continue
		}
		pop += n
		acc += n * Tranche(l, b.Servi, b.Demande)
	}
	if pop <= 0 {
		return Tranche(l, 1, 1)
	}
	// ⚠️ ARRONDI VERS LE BAS — la seule division du chemin, et elle est en bout
	// de chaine, apres que toutes les comparaisons ont ete faites en croix.
	return acc / pop
}

// PourCent : le pourcentage a AFFICHER, et nulle part ailleurs.
// ⚠️ IL PEUT DEPASSER 100 depuis les bonus (13/09) : ne pas le borner ici, ni
// dans l'ecran qui le lit — un 120 % ramene a 100 mentirait sur ce que
// l'escalier, lui, voit vraiment.
func PourCent(s, d int) int {
	if d <= 0 {
		return 100
	}
	return s * 100 / d
}
