// ============================================================
//  moteur/ressources.go — OU VIT UNE RESSOURCE, ET LE SEUL PASSAGE
//
//  ⚠️⚠️ AUCUN AUTRE FICHIER NE TOUCHE `b.Stock` NI `p.Reserve`. Quatre
//  fonctions, un seul point de passage — c'est ce qui garantit qu'un cout, une
//  production, une navette et un entretien empruntent tous le meme chemin. Le
//  §6bis le dit pour les couts : « il n'y a pas de chemin parallele ».
//
//  Trois aiguillages, dans cet ordre :
//
//    1. FluxStock       -> `p.Reserve`. UNE SEULE RESERVE PAR PLATEAU, sans
//                          coffre de case, sans plafond, sans navette.
//    2. stock commun    -> la BOURSE du type : la somme des coffres des
//                          membres, avec la somme de leurs plafonds.
//    3. sinon           -> le coffre de la case.
//
//  ⚠️ `indicateur` et `mobilise` ne vivent dans AUCUN coffre. Les lire rend 0,
//  les ranger n'absorbe rien. Ce n'est pas un oubli, c'est la regle.
//
//  ⚠️ `b` PEUT ETRE nil : la tresorerie appelle `Retirer(p, nil, ...)` pour un
//  `FluxStock`, qui ne vit dans aucune case.
// ============================================================

package moteur

import "sort"

// Membres : les membres vivants de la bourse commune d'un type, dans l'ordre
// `(z, x)`.
//
// ⚠️ NON MIS EN CACHE, volontairement : `Commun()` ne concerne qu'une poignee
// de types, et un cache indexe par (tid, instant) se serait perime a chaque
// destruction. Le balayage passe par `Ordonnes()`, lui-meme en cache.
func Membres(p *Plateau, b *Batiment, maintenant int) []*Batiment {
	tid := b.Tuile.Tid
	var l []*Batiment
	for _, m := range p.Ordonnes() {
		if m.Tuile.Tid == tid && m.Vivant(maintenant) {
			l = append(l, m)
		}
	}
	return l
}

// Lire : COMBIEN Y EN A-T-IL, vu par ce batiment ?
func Lire(p *Plateau, b *Batiment, c Code, maintenant int) int {
	switch p.Genres.de(c) {
	case classeFluxStock:
		return p.Reserve.Get(c)
	case classeIndicateur, classeMobilise:
		return 0
	}
	if b == nil {
		return 0
	}
	if b.Tuile.Commun() {
		s := 0
		for _, m := range Membres(p, b, maintenant) {
			s += m.Stock.Get(c)
		}
		return s
	}
	return b.Stock.Get(c)
}

// Plafond : COMBIEN PEUT-IL EN TENIR au total ?
func Plafond(p *Plateau, b *Batiment, c Code, maintenant int) int {
	switch p.Genres.de(c) {
	case classeFluxStock:
		// ⚠️ UN FLUXSTOCK N'A PAS DE PLAFOND. La monnaie ne sature pas.
		return INFINI
	case classeIndicateur, classeMobilise:
		return 0
	}
	if b == nil {
		return 0
	}
	if b.Tuile.Commun() {
		s := 0
		for _, m := range Membres(p, b, maintenant) {
			s += m.Tuile.MaxStocke(c)
		}
		return s
	}
	return b.Tuile.MaxStocke(c)
}

// Place : la place qui reste.
func Place(p *Plateau, b *Batiment, c Code, maintenant int) int {
	pl := Plafond(p, b, c, maintenant)
	if pl == INFINI {
		return INFINI
	}
	if n := pl - Lire(p, b, c, maintenant); n > 0 {
		return n
	}
	return 0
}

// Ranger. Rend CE QUI EST REELLEMENT PASSE — on range d'abord, on compte
// ensuite. Un appelant qui suppose que tout est entre se trompera le jour ou le
// coffre sera plein, et ce jour-la il fabriquera de la matiere.
func Ranger(p *Plateau, b *Batiment, c Code, q, maintenant int) int {
	if q <= 0 {
		return 0
	}
	switch p.Genres.de(c) {
	case classeFluxStock:
		p.Reserve.Ajouter(c, q)
		return q
	case classeIndicateur, classeMobilise:
		return 0
	}
	if b == nil {
		return 0
	}

	if b.Tuile.Commun() {
		// On remplit les membres dans l'ordre `(z, x)`, chacun jusqu'a SON
		// plafond. La bourse est une somme, mais la marchandise est bien
		// quelque part.
		reste := q
		for _, m := range Membres(p, b, maintenant) {
			if reste <= 0 {
				break
			}
			libre := m.Tuile.MaxStocke(c) - m.Stock.Get(c)
			if libre > reste {
				libre = reste
			}
			if libre > 0 {
				m.Stock.Ajouter(c, libre)
				reste -= libre
			}
		}
		return q - reste
	}

	libre := b.Tuile.MaxStocke(c) - b.Stock.Get(c)
	if libre > q {
		libre = q
	}
	if libre > 0 {
		b.Stock.Ajouter(c, libre)
		return libre
	}
	return 0
}

// Retirer. Rend CE QUI A REELLEMENT ETE PRIS.
func Retirer(p *Plateau, b *Batiment, c Code, q, maintenant int) int {
	if q <= 0 {
		return 0
	}
	switch p.Genres.de(c) {
	case classeFluxStock:
		n := p.Reserve.Get(c)
		if q < n {
			n = q
		}
		if n > 0 {
			p.Reserve.Ajouter(c, -n)
		}
		return n
	case classeIndicateur, classeMobilise:
		return 0
	}
	if b == nil {
		return 0
	}

	if b.Tuile.Commun() {
		reste := q
		for _, m := range Membres(p, b, maintenant) {
			if reste <= 0 {
				break
			}
			n := m.Stock.Get(c)
			if reste < n {
				n = reste
			}
			if n > 0 {
				m.Stock.Ajouter(c, -n)
				reste -= n
			}
		}
		return q - reste
	}

	n := b.Stock.Get(c)
	if q < n {
		n = q
	}
	if n > 0 {
		b.Stock.Ajouter(c, -n)
	}
	return n
}

// Detruire (§7). Le contenu est PERDU — sauf pour un stock commun : la
// marchandise revient aux membres restants, leurs limites sont recalculees et
// le surplus est ECRETE. Les navettes de la case partent avec elle : c'est
// gratuit parce qu'elles sont imbriquees (§10), et on ne peut pas l'oublier.
func Detruire(p *Plateau, b *Batiment, maintenant int) (rendu, ecrete Sac) {
	partageable := b.Tuile.Commun()
	contenu := b.Stock.Copie()
	b.Stock.Effacer()
	b.Navettes = nil
	for i, m := range p.Batiments {
		if m == b {
			p.Batiments = append(p.Batiments[:i], p.Batiments[i+1:]...)
			break
		}
	}
	p.ViderCaches()
	rendu, ecrete = NouveauSac(p.Genres.Reg), NouveauSac(p.Genres.Reg)
	if !partageable {
		return rendu, contenu
	}

	var restants []*Batiment
	for _, m := range p.Ordonnes() {
		if m.Tuile.Tid == b.Tuile.Tid && m.Vivant(maintenant) {
			restants = append(restants, m)
		}
	}
	sort.SliceStable(restants, func(i, j int) bool { return restants[i].clef < restants[j].clef })
	for _, c := range contenu.NonNuls() {
		reste := contenu.Get(c)
		for _, m := range restants {
			if reste <= 0 {
				break
			}
			libre := m.Tuile.MaxStocke(c) - m.Stock.Get(c)
			if libre > reste {
				libre = reste
			}
			if libre > 0 {
				m.Stock.Ajouter(c, libre)
				reste -= libre
			}
		}
		rendu.Set(c, contenu.Get(c)-reste)
		if reste > 0 {
			ecrete.Set(c, reste)
		}
	}
	return rendu, ecrete
}

// ─── CHEZ QUI LE GROUPE NE SE SERT PAS ──────────────────────────────────────
//
//  ⚠️ On se sert partout sur le plateau, SAUF chez un batiment qui consomme lui
//  aussi cette ressource — les membres du groupe exceptes, dont le coffre
//  appartient au groupe (§4). Sans cette reserve, un type de consommateur
//  viderait le garde-manger d'un autre type, et la « famine entre types » que
//  le §4 veut garder visible se transformerait en course au premier servi.

func mange(b *Batiment, c Code) bool {
	for i := range b.Tuile.Utilisation {
		l := &b.Tuile.Utilisation[i]
		if l.Ressource == c && l.Quantite > 0 {
			return true
		}
	}
	return false
}

func interdit(b *Batiment, c Code, tidGroupe int) bool {
	return b.Tuile.Tid != tidGroupe && mange(b, c)
}

// PreleverPlateau : PRELEVER SUR TOUT LE PLATEAU, sans limite de distance —
// c'est le §4, la consommation « en direct » : la ressource est prise SANS
// NAVETTE et tout le plateau est a portee.
//
// On balaie dans l'ordre `(z, x)`, seul ordre du moteur.
func PreleverPlateau(p *Plateau, c Code, q, maintenant, tidGroupe int) int {
	if q <= 0 {
		return 0
	}
	switch p.Genres.de(c) {
	case classeFluxStock:
		n := p.Reserve.Get(c)
		if q < n {
			n = q
		}
		if n > 0 {
			p.Reserve.Ajouter(c, -n)
		}
		return n
	case classeIndicateur, classeMobilise:
		return 0
	}
	reste := q
	for _, m := range p.Ordonnes() {
		if reste <= 0 {
			break
		}
		if !m.Vivant(maintenant) || interdit(m, c, tidGroupe) {
			continue
		}
		n := m.Stock.Get(c)
		if reste < n {
			n = reste
		}
		if n > 0 {
			m.Stock.Ajouter(c, -n)
			reste -= n
		}
	}
	return q - reste
}

// SurLePlateau : COMBIEN LE GROUPE PEUT-IL EN PRENDRE ? Le pendant en lecture.
func SurLePlateau(p *Plateau, c Code, maintenant, tidGroupe int) int {
	switch p.Genres.de(c) {
	case classeFluxStock:
		return p.Reserve.Get(c)
	case classeIndicateur, classeMobilise:
		return 0
	}
	s := 0
	for _, m := range p.Batiments {
		if !m.Vivant(maintenant) || interdit(m, c, tidGroupe) {
			continue
		}
		s += m.Stock.Get(c)
	}
	return s
}
