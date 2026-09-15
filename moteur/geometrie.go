// ============================================================
//  moteur/geometrie.go — la distance sur une grille hexagonale
//
//  ⚠️ PORTAGE Go du 12/09, depuis `pb_hooks/moteur/geometrie.js`.
//
//  Les cases sont rangees en QUINCONCE (« odd-r offset ») : une ligne sur
//  deux est decalee d'une demi-case. Compter des cases en (dx, dz) donnerait
//  donc un voisinage faux — la case juste au-dessus a droite n'est pas a la
//  meme distance selon la parite de la ligne.
//
//  On passe donc en coordonnees AXIALES, ou la distance hexagonale s'ecrit
//  simplement. C'est le meme calcul que `PlacementValidator.Distance` cote C#
//  et que `dist()` dans le miroir Python : les trois doivent rendre le meme
//  nombre, sinon un rayon de recolte ne couvre pas les memes cases dans le jeu
//  et sur le serveur — et le joueur voit son entrepot « oublier » une ferme
//  qu'il voit pourtant a portee.
// ============================================================

package moteur

// Axial convertit (x, z) en quinconce vers (q, r) axial.
//
// ⚠️ `z - (z & 1)` est TOUJOURS PAIR, donc la division entiere de Go (qui
// tronque vers zero) rend exactement le meme resultat que le `Math.floor` du
// JS, y compris pour un z negatif. Ce n'est pas une coincidence heureuse, il
// fallait le verifier : partout ailleurs dans ce portage, une division qui
// pourrait porter sur un negatif est un piege.
func Axial(x, z int) (int, int) {
	return x - (z-(z&1))/2, z
}

// Distance hexagonale entre deux cases.
func Distance(x1, z1, x2, z2 int) int {
	q1, r1 := Axial(x1, z1)
	q2, r2 := Axial(x2, z2)
	dq, dr := q1-q2, r1-r2
	return (abs(dq) + abs(dq+dr) + abs(dr)) / 2
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// APortee : `rayon < 0` = tout le plateau.
func APortee(rayon, x1, z1, x2, z2 int) bool {
	return rayon < 0 || Distance(x1, z1, x2, z2) <= rayon
}

// ─── LE MONDE : ce sur quoi la proximite compte ─────────────────────────────
//
//  ⚠️⚠️ ON COMPTE SUR LE PLATEAU, PAS SUR LES ETATS.
//
//  Releve le 2026-08-31 en comparant le moteur serveur au jeu sur une vraie
//  colonie : le jeu produisait 247 nourriture et mettait 1 case en penurie, le
//  serveur 0 nourriture et 6 cases en penurie. Sur un vrai plateau, 8 etats
//  pour 10 000 cases : forets, herbe, decor et tout batiment sans etat sont
//  INVISIBLES a un compte sur les etats.
//
//  ⚠️ AUCUN DES 68 VECTEURS NE POUVAIT L'ATTRAPER : dans tous, chaque tuile
//  porte un etat. Le monde des scenarios etait plus petit que le vrai. Le
//  scenario S10 existe depuis pour que ca ne repasse jamais.

// Monde : ce qui occupe chaque case. Deux implementations — le tableau
// d'octets du vrai plateau, et le repli reconstruit depuis les cases baties.
type Monde interface {
	Tid(x, z int) int
	CompterTout(vise map[int]bool) int
}

// MondeDuPlateau : le monde vu depuis un plateau charge (tableau d'octets).
// LE cas reel.
type MondeDuPlateau struct {
	Largeur, Hauteur int
	Tiles            []int
}

func (m *MondeDuPlateau) Tid(x, z int) int {
	if x < 0 || z < 0 || x >= m.Largeur || z >= m.Hauteur {
		return 0
	}
	i := z*m.Largeur + x
	if i >= len(m.Tiles) {
		return 0
	}
	return m.Tiles[i]
}

func (m *MondeDuPlateau) CompterTout(vise map[int]bool) int {
	n := 0
	for _, t := range m.Tiles {
		if vise[t] {
			n++
		}
	}
	return n
}

// MondeDeCases : le monde reconstruit depuis les seules cases baties.
// **Repli**, exact tant que chaque tuile porte un etat — ce qui est vrai des
// scenarios de test et FAUX d'un vrai plateau.
type MondeDeCases struct {
	table map[[2]int]int
}

func (m *MondeDeCases) Tid(x, z int) int { return m.table[[2]int{x, z}] }

func (m *MondeDeCases) CompterTout(vise map[int]bool) int {
	n := 0
	for _, t := range m.table {
		if vise[t] {
			n++
		}
	}
	return n
}

// Compter : combien de cases parmi `tileIds` sont dans le rayon.
//
// ⚠️ Les cibles d'une regle sont un OU : on ADDITIONNE leurs presences
// (30/08). 3 paturages + 2 bergeries remplissent « 5 au total parmi les
// deux ». Une case n'est jamais comptee deux fois — elle ne porte qu'un tid.
//
// ⚠️ Un id hors 1..65535 ne trouve rien : une case ne peut pas le porter.
func Compter(monde Monde, x, z, rayon int, tileIds []int) int {
	vise := map[int]bool{}
	for _, t := range tileIds {
		if TileIdValide(t) {
			vise[t] = true
		}
	}
	if len(vise) == 0 {
		return 0
	}

	// `rayon < 0` = tout le plateau : on balaie, sans passer par la geometrie.
	if rayon < 0 {
		return monde.CompterTout(vise)
	}

	// Une fenetre large autour de la case — c'est le test de distance qui
	// tranche, la fenetre ne fait qu'eviter de balayer le plateau entier.
	marge := rayon + (rayon >> 1) + 1
	n := 0
	for zz := z - rayon; zz <= z+rayon; zz++ {
		for xx := x - marge; xx <= x+marge; xx++ {
			if Distance(x, z, xx, zz) > rayon {
				continue
			}
			if vise[monde.Tid(xx, zz)] {
				n++
			}
		}
	}
	return n
}
