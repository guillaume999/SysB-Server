// ============================================================
//  moteur/sac.go — LE REGISTRE DES CODES, ET LE SAC
//
//  ⚠️⚠️ CE FICHIER N'A PAS D'EQUIVALENT EN JS, ET C'EST TOUT L'INTERET.
//
//  Le premier portage (12/09, matin) transcrivait le JS : chaque coffre etait
//  une `map[string]int`. Mesure au profileur : **41 % du temps passe a hacher
//  des chaines** (`aeshashbody`, `mapaccess1_faststr`). V8 range un petit objet
//  a cles fixes dans une classe cachee, ce qu'une map Go ne sait pas faire —
//  le portage etait donc a peine plus rapide que node, alors que Go devrait
//  l'ecraser.
//
//  La reponse n'est pas « optimiser la map », c'est **ne plus en avoir** : une
//  ressource devient un ENTIER des le chargement, et un coffre devient un
//  tableau indexe par cet entier. Plus une seule chaine n'est hachee pendant
//  une passe.
//
//  ⚠️ UNE SEULE REGLE A TENIR, ET ELLE EST SUBTILE : le JS trie ses sorties PAR
//  NOM (`Object.keys(stock).sort()`). Si les identifiants etaient distribues
//  dans l'ordre de rencontre, parcourir « par identifiant croissant » ne serait
//  PAS « par nom croissant », et l'etat ecrit changerait d'ordre — donc les 28
//  photos ne colleraient plus. `Alpha()` est la pour ca, et c'est le seul
//  endroit ou l'ordre des noms est reconstruit.
// ============================================================

package moteur

import "sort"

// Code : l'identite interne d'une ressource. `CodeInconnu` vaut pour une
// ressource qu'aucun catalogue n'a jamais nommee.
type Code int

const CodeInconnu Code = -1

// Registre : nom <-> identifiant. C'est le SEUL endroit du moteur ou une chaine
// de ressource est encore hachee, et il ne sert qu'au chargement.
type Registre struct {
	noms  []string
	ids   map[string]Code
	alpha []Code // cache : les codes dans l'ordre ALPHABETIQUE de leur nom
}

func NouveauRegistre() *Registre {
	return &Registre{ids: map[string]Code{}}
}

// Inscrire : rend l'identifiant du nom, en le creant au besoin.
func (r *Registre) Inscrire(nom string) Code {
	if nom == "" {
		return CodeInconnu
	}
	if c, ok := r.ids[nom]; ok {
		return c
	}
	c := Code(len(r.noms))
	r.noms = append(r.noms, nom)
	r.ids[nom] = c
	r.alpha = nil // l'ordre alphabetique est a refaire
	return c
}

// Id : l'identifiant d'un nom deja connu, `CodeInconnu` sinon.
func (r *Registre) Id(nom string) Code {
	if c, ok := r.ids[nom]; ok {
		return c
	}
	return CodeInconnu
}

func (r *Registre) Nom(c Code) string {
	if c < 0 || int(c) >= len(r.noms) {
		return ""
	}
	return r.noms[c]
}

func (r *Registre) Nb() int { return len(r.noms) }

// Alpha : tous les codes, dans l'ordre ALPHABETIQUE de leur nom.
//
// ⚠️ C'est le remplacant exact de `Object.keys(...).sort()` du JS. Partout ou le
// moteur doit produire un ordre stable et le MEME que le JS — l'etat ecrit, la
// liste des ressources transportables, le parcours d'un sac — c'est par ici que
// ca passe, et nulle part ailleurs.
func (r *Registre) Alpha() []Code {
	if r.alpha == nil {
		l := make([]Code, len(r.noms))
		for i := range l {
			l[i] = Code(i)
		}
		sort.Slice(l, func(i, j int) bool { return r.noms[l[i]] < r.noms[l[j]] })
		r.alpha = l
	}
	return r.alpha
}

// ─── LE SAC ─────────────────────────────────────────────────────────────────

// Sac : un coffre, une cale de navette, une reserve. Un tableau indexe par
// `Code`, alloue seulement quand on y met quelque chose — la plupart des
// navettes partent a vide et n'allouent donc rien.
//
// ⚠️ IL N'Y A PAS DE « CLE ABSENTE » : une ressource jamais rangee vaut zero,
// exactement comme `stock[code] || 0` en JS. C'est ce qui permet de supprimer
// tous les tests d'existence du portage precedent.
type Sac struct {
	q   []int
	reg *Registre
}

func NouveauSac(reg *Registre) Sac { return Sac{reg: reg} }

func (s *Sac) Get(c Code) int {
	if c < 0 || int(c) >= len(s.q) {
		return 0
	}
	return s.q[c]
}

func (s *Sac) Set(c Code, v int) {
	if c < 0 {
		return
	}
	if int(c) >= len(s.q) {
		if v == 0 {
			return // rien a ranger : inutile d'allouer
		}
		n := s.reg.Nb()
		if int(c) >= n {
			n = int(c) + 1
		}
		q := make([]int, n)
		copy(q, s.q)
		s.q = q
	}
	s.q[c] = v
}

func (s *Sac) Ajouter(c Code, v int) { s.Set(c, s.Get(c)+v) }

// NonNuls : les codes que ce sac porte vraiment, dans l'ordre ALPHABETIQUE de
// leur nom — le meme que `Object.keys(stock).sort()` en JS.
func (s *Sac) NonNuls() []Code {
	if len(s.q) == 0 {
		return nil
	}
	var l []Code
	for _, c := range s.reg.Alpha() {
		if int(c) < len(s.q) && s.q[c] != 0 {
			l = append(l, c)
		}
	}
	return l
}

// Vide : rien dedans ?
func (s *Sac) Vide() bool {
	for _, v := range s.q {
		if v != 0 {
			return false
		}
	}
	return true
}

func (s *Sac) Copie() Sac {
	c := Sac{reg: s.reg}
	if len(s.q) > 0 {
		c.q = append([]int(nil), s.q...)
	}
	return c
}

func (s *Sac) Effacer() { s.q = nil }

// ─── Un jeu de drapeaux par code (« quelles ressources sont en direct ? ») ───
//
//  ⚠️ Remplace les `map[string]bool` du portage precedent, qui etaient alloues
//  et remplis a chaque appel de `ciblesDe` — des millions de fois par passe.

type Drapeaux struct{ v []bool }

func NouveauxDrapeaux(n int) Drapeaux { return Drapeaux{v: make([]bool, n)} }

func (d *Drapeaux) Mis(c Code) bool {
	return c >= 0 && int(c) < len(d.v) && d.v[c]
}

func (d *Drapeaux) Mettre(c Code) {
	if c >= 0 && int(c) < len(d.v) {
		d.v[c] = true
	}
}

func (d *Drapeaux) Aucun() bool {
	for _, b := range d.v {
		if b {
			return false
		}
	}
	return true
}

// Ou : l'union de deux jeux, sans allouer quand le second est vide.
func (d Drapeaux) Ou(autre Drapeaux) Drapeaux {
	if autre.Aucun() {
		return d
	}
	if d.Aucun() {
		return autre
	}
	out := NouveauxDrapeaux(len(d.v))
	for i := range d.v {
		out.v[i] = d.v[i] || (i < len(autre.v) && autre.v[i])
	}
	return out
}
