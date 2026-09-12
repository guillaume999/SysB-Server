// ============================================================
//  moteur/indice.go — LA SATISFACTION ET L'ESCALIER (§5)
//
//      satisfaction = servi / demande_au_max,  exprime EN POUR CENT
//
//  ⚠️⚠️ C'EST UN RAPPORT RECALCULE, PAS UNE DONNEE STOCKEE.
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
type Rapport struct{ Num, P, W int }

func CreerRapport() Rapport { return Rapport{Num: 0, P: 1, W: 0} }

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

// Le couple d'entiers (servi, demande) — c'est lui qu'on compare en croix.
func (acc Rapport) Servi() int   { return acc.Num }
func (acc Rapport) Demande() int { return acc.P * acc.W }

// ─── L'escalier ─────────────────────────────────────────────────────────────

// Tranche : quelle tranche pour un rapport `servi / demande` ?
//
// ⚠️ MULTIPLICATION EN CROIX, jamais un pourcentage calcule puis compare.
// ⚠️ Un batiment qui ne demande rien est PARFAITEMENT servi : tranche du haut.
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
		return l.Tranches[0][1]
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
func PourCent(s, d int) int {
	if d <= 0 {
		return 100
	}
	return s * 100 / d
}
