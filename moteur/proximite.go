// ============================================================
//  moteur/proximite.go — LE FACTEUR DE PROXIMITE (§4bis)
//
//      quantite effective = ⌊ quantite x min(compte, nombre) / nombre ⌋
//
//  ⚠️⚠️ LE FACTEUR S'APPLIQUE A TOUT LE PALIER, pas a la ligne qui le porte. Un
//  abattoir a 3 bergeries sur 5 consomme 3 bovin au lieu de 5 ET livre 12 viande
//  au lieu de 20. Le temoin `abattoir_sans_regle` porte 3 et 12 en dur.
//
//  ⚠️ C'est la MEME mecanique que la satisfaction : un rapport exact, un arrondi
//  vers le bas, une seule fois, sur la quantite du cycle. Et comme la quantite
//  est deja reduite quand la satisfaction se calcule, LES DEUX NE SE MULTIPLIENT
//  PAS EN DOUBLE.
//
//  Combiner :
//    · OU  — plusieurs `tileIds` dans UNE regle : les cibles S'ADDITIONNENT. S1c.
//    · ET  — plusieurs regles sur une ligne : le MINIMUM des facteurs. S1d.
//
//  ⚠️⚠️ ON COMPTE SUR LE PLATEAU, PAS SUR LES ETATS (31/08 : le jeu produisait
//  247 nourriture, le serveur 0).
//
//  ⚠️ MIS EN CACHE PAR BATIMENT POUR LA DUREE D'UNE PASSE. Le facteur ne depend
//  que du terrain et de la tuile, et ni l'un ni l'autre ne bouge pendant un
//  `Avancer` — alors que le JS le recalculait QUATRE fois par batiment et par
//  cycle (groupes, indice, conso/prod, deQuoiTourner), chacune balayant une
//  fenetre de cases.
// ============================================================

package moteur

// mondeDesBatiments : le monde de REPLI, reconstruit depuis les seules cases
// baties. Exact tant que chaque tuile porte un etat — ce qui est vrai des
// scenarios et FAUX d'un vrai plateau, d'ou `MondeReel` qui l'emporte toujours.
func mondeDesBatiments(p *Plateau) Monde {
	table := map[[2]int]int{}
	for _, b := range p.Batiments {
		table[[2]int{b.X, b.Z}] = b.Tuile.Tid
	}
	for _, s := range p.Sol {
		table[[2]int{s.X, s.Z}] = s.Tid
	}
	return &MondeDeCases{table: table}
}

func (p *Plateau) monde() Monde {
	if p.MondeReel != nil {
		return p.MondeReel
	}
	if p.mondeReconstruit == nil {
		p.mondeReconstruit = mondeDesBatiments(p)
	}
	return p.mondeReconstruit
}

// Facteur : LE FACTEUR DU PALIER, en rapport exact `[num, den]` — jamais un
// pourcentage calcule d'avance, meme regle qu'au §5.
//
// `[1, 1]` quand la tuile ne porte aucune proximite : c'est le cas de presque
// tout le catalogue, et il ne coute rien.
func Facteur(p *Plateau, b *Batiment) [2]int {
	if b.proxFait {
		return b.prox
	}
	num, den := 1, 1
	vu := false
	if b.Tuile.aProximites {
		monde := p.monde()
		for _, champ := range [][]Ligne{b.Tuile.Utilisation, b.Tuile.Production} {
			for i := range champ {
				for _, pr := range champ[i].Proximites {
					voulu := pr.Nombre
					if voulu <= 0 {
						continue // une regle sans exigence ne dit rien
					}
					n := Compter(monde, b.X, b.Z, pr.Rayon, pr.TileIds)
					if n > voulu {
						n = voulu
					}
					// ⚠️ LE MINIMUM, compare EN CROIX — aucun arrondi ne decide.
					if !vu || n*den < num*voulu {
						num, den, vu = n, voulu, true
					}
				}
			}
		}
	}
	b.prox = [2]int{num, den}
	b.proxFait = true
	return b.prox
}

// QuantiteLigne : la quantite d'une ligne une fois le palier plafonne.
// ⚠️ Arrondi VERS LE BAS.
func QuantiteLigne(l *Ligne, prox [2]int) int {
	q := l.Quantite
	if q <= 0 {
		return 0
	}
	if prox[0] == prox[1] {
		return q // le cas courant : rien a faire
	}
	return q * prox[0] / prox[1]
}
