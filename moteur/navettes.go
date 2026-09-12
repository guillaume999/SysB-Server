// ============================================================
//  moteur/navettes.go — ARRIVEE et DEPART (§6)
//
//  Quatre regles, et trois modeles sont morts avant elles (07/09) :
//
//   · L'ALLOCATION SE FAIT AU DEPART, juste avant le cycle suivant : on regarde
//     ce qui est REELLEMENT disponible, et ON N'ENVOIE PAS UNE NAVETTE QUI
//     ARRIVERAIT A VIDE.
//   · A L'ARRIVEE, ON PREND CE QU'IL RESTE.
//   · LA FLOTTE SE PARTAGE A PARTS EGALES entre les cibles a portee.
//   · UN VOYAGE EST ENTIER, et il dure `2 x distance x periode_s / crans`
//     pour l'aller-retour.
//
//  ⚠️⚠️ UNE RESSOURCE CONSOMMEE EN DIRECT N'EST JAMAIS ALLEE CHERCHER PAR UNE
//  NAVETTE (§4). Envoyer quand meme une navette, ce serait payer deux fois le
//  meme approvisionnement.
//
//  ⚠️ C'EST LE FICHIER LE PLUS CHAUD DU MOTEUR : 81 % du temps du premier
//  portage y passait. Trois choses ont ete sorties de la boucle, et AUCUNE ne
//  change une regle — la geometrie des cibles, la liste des ressources d'une
//  regle, et les ressources « en direct » d'un batiment.
// ============================================================

package moteur

import "sort"

// directesDe : les ressources que CE batiment consomme en direct — elles ne
// voyagent pas. ⚠️ Calcule une fois par batiment, pas a chaque appel.
func directesDe(p *Plateau, b *Batiment) Drapeaux {
	if !b.directesFait {
		b.directes = NouveauxDrapeaux(p.Genres.Reg.Nb())
		for i := range b.Tuile.Utilisation {
			l := &b.Tuile.Utilisation[i]
			if l.Direct && l.Quantite > 0 {
				b.directes.Mettre(l.Ressource)
			}
		}
		b.directesFait = true
	}
	return b.directes
}

// DureeTrajet : la duree d'UN trajet, en secondes entieres. `crans <= 0`
// BLOQUE (08/09).
//
// ⚠️ L'ARRONDI EST SUR LA JAMBE, PAS SUR L'ALLER-RETOUR — c'est ce que faisait
// le JS (`Math.ceil`), et le §6 ne dit pas ou il tombe. Porte tel quel plutot
// que « corrige » : une seconde de decalage ici deplace chaque arrivee.
func DureeTrajet(a *Appro, dist int) int {
	if a.Crans <= 0 || a.PeriodeS <= 0 {
		return -1
	}
	n := dist * a.PeriodeS
	return (n + a.Crans - 1) / a.Crans // ceil, sur des entiers positifs
}

// enVol : combien de navettes de cette regle sont deja en vol.
func enVol(b *Batiment, i int) int {
	n := 0
	for _, nav := range b.Navettes {
		if nav.Regle == i {
			n++
		}
	}
	return n
}

// accepte — ⚠️⚠️ UNE CASE ACCEPTE-T-ELLE QU'ON LUI LIVRE CETTE RESSOURCE ? (§6)
//
// Un `envoi` ne choisit pour cible qu'une case qui porte elle-meme une regle
// ENTRANTE couvrant la ressource. Sans ca, un entrepot qui porte les deux
// regles — le cas courant du catalogue — RENVOIE A LA FERME tout ce qu'il vient
// d'y prendre, a l'instant meme. Mesure le 11/09 en rejouant F1 : il ramassait
// 0 au lieu de 450.
//
// ⚠️ L'INVERSE N'EST PAS VRAI : une regle entrante se sert chez n'importe qui, y
// compris une tuile sans la moindre regle d'appro. C'est ce qui fait qu'une
// ferme nue se fait recolter, et F1 en depend.
func accepte(m *Batiment, c Code) bool {
	for i := range m.Tuile.Appros {
		a := &m.Tuile.Appros[i]
		if a.Sens == "envoi" {
			continue
		}
		if a.Gere(c) {
			return true
		}
	}
	return false
}

// candidatsGeo : les cases que la GEOMETRIE seule rend atteignables — meme case
// exclue, `tileIds`, `Vise`, le rayon, distance > 0.
//
// ⚠️⚠️ LA LIMITE EST ICI, ET ELLE EST SUBTILE : le terrain ne bouge pas pendant
// une passe, mais `Vivant` SI — un chantier se termine en cours de route, une
// case se met en veille. On ne met donc en cache que la partie GEOMETRIQUE, et
// `Vivant(t)` se redemande a chaque usage. Mettre `aPorteeDe` entier en cache
// aurait fige les chantiers : un batiment jamais livre, et le banc ne l'aurait
// pas vu tout de suite.
func candidatsGeo(p *Plateau, b *Batiment, a *Appro) []*Batiment {
	if p.geo == nil {
		p.geo = map[*Appro]map[int][]*Batiment{}
	}
	parRegle := p.geo[a]
	if parRegle == nil {
		parRegle = map[int][]*Batiment{}
		p.geo[a] = parRegle
	}
	if l, ok := parRegle[b.clef]; ok {
		return l
	}
	l := []*Batiment{}
	for _, m := range p.Ordonnes() {
		if m == b {
			continue
		}
		if len(a.TileIds) > 0 {
			trouve := false
			for _, id := range a.TileIds {
				if id == m.Tuile.Tid {
					trouve = true
					break
				}
			}
			if !trouve {
				continue
			}
		}
		if !a.Vise(m.Tuile.Tid) {
			continue
		}
		if !APortee(a.Rayon, b.X, b.Z, m.X, m.Z) {
			continue
		}
		if Distance(b.X, b.Z, m.X, m.Z) <= 0 {
			continue
		}
		l = append(l, m)
	}
	parRegle[b.clef] = l
	return l
}

type cible struct {
	B     *Batiment
	Codes []Code
}

// ciblesDe : les cibles a portee qui valent le voyage, dans l'ordre `(z, x)`.
//
// ⚠️ `candidatsGeo` rend deja la liste dans l'ordre `(z, x)` (elle sort
// d'`Ordonnes`), donc il n'y a plus rien a trier ici — le JS retriait a chaque
// appel.
func ciblesDe(p *Plateau, b *Batiment, a *Appro, t int, directes Drapeaux) []cible {
	candidats := candidatsGeo(p, b, a)
	if len(candidats) == 0 {
		return nil
	}
	// ⚠️ Les deux tranches sont dimensionnees d'avance. Ce n'est pas de la
	// coquetterie : le profil du 12/09 mettait 31 % du temps dans `growslice`,
	// a reallouer ces deux listes des millions de fois par passe.
	l := make([]cible, 0, len(candidats))
	for _, m := range candidats {
		if !m.Vivant(t) {
			continue
		}
		// ⚠️ Cote destinataire aussi : ce qu'il mange en direct ne se livre pas.
		interdites := directes
		if a.Sens == "envoi" {
			interdites = directes.Ou(directesDe(p, m))
		}
		utiles := make([]Code, 0, len(a.Transportables))
		for _, c := range a.Transportables {
			if interdites.Mis(c) {
				continue
			}
			if a.Sens == "recolte" {
				// il faut qu'il y en ait la-bas ET de la place ici
				if Lire(p, m, c, t) > 0 && Place(p, b, c, t) > 0 {
					utiles = append(utiles, c)
				}
			} else {
				// il faut qu'il y en ait ici, de la place la-bas, ET que la-bas
				// ACCEPTE qu'on lui livre.
				if Lire(p, b, c, t) > 0 && Place(p, m, c, t) > 0 && accepte(m, c) {
					utiles = append(utiles, c)
				}
			}
		}
		// ⚠️ ON N'ENVOIE PAS UNE NAVETTE QUI ARRIVERAIT A VIDE.
		if len(utiles) == 0 {
			continue
		}
		l = append(l, cible{B: m, Codes: utiles})
	}
	return l
}

// Departs : pour chaque regle d'appro, on partage la flotte disponible a parts
// egales entre les cibles qui ont (recolte) ou qui peuvent prendre (envoi)
// quelque chose.
func Departs(p *Plateau, b *Batiment, t int) {
	if len(b.Tuile.Appros) == 0 {
		return
	}
	directes := directesDe(p, b)
	for i := range b.Tuile.Appros {
		a := &b.Tuile.Appros[i]

		// `illimite` : pas de navette du tout, le transfert est immediat.
		if a.Illimite {
			transfertImmediat(p, b, a, t, directes)
			continue
		}

		libres := a.Navettes - enVol(b, i)
		if libres <= 0 || a.Capacite <= 0 {
			continue
		}

		cibles := ciblesDe(p, b, a, t, directes)
		if len(cibles) == 0 {
			continue
		}

		// ⚠️ A PARTS EGALES. Le reste va aux premieres dans l'ordre `(z, x)`.
		base := libres / len(cibles)
		reste := libres - base*len(cibles)
		for _, c := range cibles {
			n := base
			if reste > 0 {
				n++
				reste--
			}
			if n <= 0 {
				continue
			}
			trajet := DureeTrajet(a, Distance(b.X, b.Z, c.B.X, c.B.Z))
			if trajet < 0 {
				continue
			}
			for k := 0; k < n; k++ {
				if !envoyerUne(p, b, a, i, c, t, trajet) {
					break
				}
			}
		}
	}
}

// envoyerUne : une navette. Rend `false` si elle n'avait rien a faire — on
// arrete la volee.
func envoyerUne(p *Plateau, b *Batiment, a *Appro, i int, c cible, t, trajet int) bool {
	if a.Sens == "recolte" {
		// Elle part A VIDE : on charge a l'arrivee, « on prend ce qu'il reste ».
		// Mais on ne part que s'il y a de quoi MAINTENANT (allocation au depart).
		ya := false
		for _, code := range c.Codes {
			if Lire(p, c.B, code, t) > 0 {
				ya = true
				break
			}
		}
		if !ya {
			return false
		}
		b.Navettes = append(b.Navettes, &Navette{
			Origine: [2]int{b.X, b.Z}, Destination: [2]int{c.B.X, c.B.Z},
			PartiA: t, ArriveA: t + trajet, Sens: "aller", Regle: i,
			Charge: NouveauSac(p.Genres.Reg)})
		return true
	}

	// ENVOI : on charge MAINTENANT, chez soi, et on ne part pas a vide.
	charge := NouveauSac(p.Genres.Reg)
	reste := a.Capacite
	for _, code := range c.Codes {
		if reste <= 0 {
			break
		}
		veut := reste
		if pl := Place(p, c.B, code, t); pl != INFINI && pl < veut {
			veut = pl
		}
		pris := Retirer(p, b, code, veut, t)
		if pris > 0 {
			charge.Ajouter(code, pris)
			reste -= pris
		}
	}
	if reste == a.Capacite {
		return false // rien charge : elle ne part pas
	}
	b.Navettes = append(b.Navettes, &Navette{
		Origine: [2]int{b.X, b.Z}, Destination: [2]int{c.B.X, c.B.Z},
		PartiA: t, ArriveA: t + trajet, Sens: "aller", Regle: i, Charge: charge})
	return true
}

// transfertImmediat : `illimite`, le transfert se fait sur place, sans objet et
// sans duree. Meme comptabilite que le reste — `Lire` / `Retirer` / `Ranger`,
// jamais un chemin parallele.
func transfertImmediat(p *Plateau, b *Batiment, a *Appro, t int, directes Drapeaux) {
	for _, c := range ciblesDe(p, b, a, t, directes) {
		for _, code := range c.Codes {
			src, dst := b, c.B
			if a.Sens == "recolte" {
				src, dst = c.B, b
			}
			veut := Place(p, dst, code, t)
			if veut == INFINI {
				veut = Lire(p, src, code, t)
			}
			pris := Retirer(p, src, code, veut, t)
			entre := Ranger(p, dst, code, pris, t)
			if entre < pris {
				// ⚠️ Ce qui ne tient pas dans le coffre d'en face revient chez
				// l'expediteur (§6).
				Ranger(p, src, code, pris-entre, t)
			}
		}
	}
}

// Arrivees : les livraisons et les recoltes qui atterrissent MAINTENANT sont
// encaissees — et c'est l'etape 1 du cycle, avant l'indice et avant la
// consommation. ⚠️ L'ordre 1 avant 3 n'est pas cosmetique : deux versions sont
// mortes au test pour l'avoir enfreint (§2).
func Arrivees(p *Plateau, b *Batiment, t int) {
	if len(b.Navettes) == 0 {
		return
	}
	// ⚠️ PAS DE RACCOURCI « la premiere n'est pas arrivee, donc aucune » : la
	// liste est triee a la FIN de cette fonction, mais `Departs` y pousse
	// ensuite des navettes dont l'arrivee peut tomber AVANT. Elle n'est donc
	// pas triee en entrant. Le raccourci a ete ecrit puis retire le 12/09.
	var encore []*Navette
	// Une copie : un aller qui se retourne pousse une navette neuve, qui ne
	// doit pas etre relue dans la meme passe.
	lot := b.Navettes
	b.Navettes = nil

	for _, n := range lot {
		if n.ArriveA > t {
			encore = append(encore, n)
			continue
		}
		var regle *Appro
		if n.Regle >= 0 && n.Regle < len(b.Tuile.Appros) {
			regle = &b.Tuile.Appros[n.Regle]
		}

		if n.Sens == "retour" {
			// Elle rentre a la maison : on vide la cale.
			for _, c := range n.Charge.NonNuls() {
				q := n.Charge.Get(c)
				if q <= 0 {
					continue
				}
				if entre := Ranger(p, b, c, q, t); entre < q {
					// ⚠️ CE QUI NE RENTRE PAS EST PERDU, et la spec ne le dit
					// nulle part — releve le 12/09, §11.3-D.
					p.perdre(c, q-entre)
				}
			}
			continue // la navette a fini son voyage : la place de flotte est libre
		}

		// C'est un ALLER qui touche sa cible.
		ciblee := p.BatimentEn(n.Destination[0], n.Destination[1])
		trajet := n.ArriveA - n.PartiA

		if regle != nil && regle.Sens == "envoi" {
			// On livre ce qu'on peut ; le reliquat repart avec elle.
			if ciblee != nil && ciblee.Vivant(t) {
				for _, c := range n.Charge.NonNuls() {
					q := n.Charge.Get(c)
					if q <= 0 {
						continue
					}
					n.Charge.Set(c, q-Ranger(p, ciblee, c, q, t))
				}
			}
		} else if regle != nil {
			// RECOLTE : « a l'arrivee, on prend ce qu'il reste ».
			// ⚠️ CE QU'ELLE A LE DROIT DE PRENDRE SE RELIT DANS SA REGLE, ICI, A
			// L'ARRIVEE (11/09) — rien n'est ecrit sur la navette.
			reste := regle.Capacite
			if reste > 0 && ciblee != nil && ciblee.Vivant(t) {
				directes := directesDe(p, b)
				for _, c := range regle.Transportables {
					if reste <= 0 {
						break
					}
					if directes.Mis(c) {
						continue
					}
					veut := reste
					if pl := Place(p, b, c, t); pl != INFINI && pl < veut {
						veut = pl
					}
					if pris := Retirer(p, ciblee, c, veut, t); pris > 0 {
						n.Charge.Ajouter(c, pris)
						reste -= pris
					}
				}
			}
		}

		// Elle fait demi-tour. Le retour dure autant que l'aller (§6).
		n.Origine, n.Destination = n.Destination, n.Origine
		n.PartiA, n.ArriveA = t, t+trajet
		n.Sens = "retour"
		b.Navettes = append(b.Navettes, n)
	}
	b.Navettes = append(b.Navettes, encore...)
	sort.SliceStable(b.Navettes, func(i, j int) bool {
		return b.Navettes[i].ArriveA < b.Navettes[j].ArriveA
	})
}
