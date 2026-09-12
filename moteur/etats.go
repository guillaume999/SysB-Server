// ============================================================
//  moteur/etats.go — LE FORMAT DE `plateaux.etats` (§10)
//
//  Une entree par case qui a quelque chose a retenir. Ni `satisfaction` ni
//  `position` : les deux se RECALCULENT.
//
//  ⚠️ PAS DE `tid` : il est deja dans `tilesBase64`, un octet par case.
//  ⚠️ LES NAVETTES VIVENT DANS LA CASE QUI LES POSSEDE : detruire un batiment
//     doit emporter ses navettes, et imbriquees c'est gratuit.
//  ⚠️ L'ORDRE DES CLES d'un stock est ALPHABETIQUE — c'est ce que faisait
//     `Object.keys(stock).sort()`, et l'etat ecrit se compare texte a texte.
// ============================================================

package moteur

import (
	"encoding/json"
	"strings"
)

type NavetteEcrite struct {
	Origine     [2]int         `json:"origine"`
	Destination [2]int         `json:"destination"`
	PartiA      int            `json:"parti_a"`
	ArriveA     int            `json:"arrive_a"`
	Sens        string         `json:"sens"`
	Regle       int            `json:"regle"`
	Charge      map[string]int `json:"charge"`
}

type EtatEcrit struct {
	X           int             `json:"x"`
	Z           int             `json:"z"`
	Niveau      int             `json:"niveau"`
	Actif       bool            `json:"actif"`
	Stock       map[string]int  `json:"stock"`
	TCycle      int             `json:"t_cycle"`
	EnMarche    bool            `json:"en_marche"`
	ChantierFin *int            `json:"chantier_fin,omitempty"`
	Navettes    []NavetteEcrite `json:"navettes"`
}

func versNoms(reg *Registre, s *Sac) map[string]int {
	out := map[string]int{}
	for _, c := range s.NonNuls() {
		if q := s.Get(c); q > 0 {
			out[reg.Nom(c)] = q
		}
	}
	return out
}

// EcrireEtats : le plateau, tel qu'il retourne en base (§10).
//
// ⚠️ `chantier_fin` n'est ecrit que tant qu'il est A VENIR. Passe, il ne dit
// plus rien : le garder serait un champ mort qui traine dans chaque sauvegarde.
func EcrireEtats(p *Plateau) []EtatEcrit {
	reg := p.Genres.Reg
	sortie := []EtatEcrit{}
	for _, b := range p.Ordonnes() {
		e := EtatEcrit{X: b.X, Z: b.Z, Niveau: b.Niveau, Actif: b.Actif,
			Stock: versNoms(reg, &b.Stock), TCycle: b.TCycle, EnMarche: b.EnMarche,
			Navettes: []NavetteEcrite{}}
		if b.ChantierFin != nil && *b.ChantierFin > p.T {
			v := *b.ChantierFin
			e.ChantierFin = &v
		}
		for _, n := range b.Navettes {
			e.Navettes = append(e.Navettes, NavetteEcrite{
				Origine: n.Origine, Destination: n.Destination,
				PartiA: n.PartiA, ArriveA: n.ArriveA, Sens: n.Sens, Regle: n.Regle,
				Charge: versNoms(reg, &n.Charge)})
		}
		sortie = append(sortie, e)
	}
	return sortie
}

// Photo : l'etat ecrit + la reserve, en une chaine comparable. C'est ce qui sert
// a juger l'INVARIANCE AUX CADENCES — le meme intervalle, joue en morceaux, doit
// rendre exactement la meme chaine.
func Photo(p *Plateau) string {
	var sb strings.Builder
	a, _ := json.Marshal(EcrireEtats(p))
	sb.Write(a)
	sb.WriteString("|")
	b, _ := json.Marshal(versNoms(p.Genres.Reg, &p.Reserve))
	sb.Write(b)
	return sb.String()
}

// ─── CE QUE LE CLIENT DEMANDE AU MOMENT OU IL DESSINE ───────────────────────
//
//  La position de chaque navette, et la satisfaction de chaque batiment EN POUR
//  CENT. Ni l'une ni l'autre n'est rangee — les deux se calculent (§2bis, §5).

type NavetteVue struct {
	Position    [2]float64     `json:"position"`
	Origine     [2]int         `json:"origine"`
	Destination [2]int         `json:"destination"`
	PartiA      int            `json:"parti_a"`
	ArriveA     int            `json:"arrive_a"`
	Sens        string         `json:"sens"`
	Charge      map[string]int `json:"charge"`
}

type CaseVue struct {
	X          int  `json:"x"`
	Z          int  `json:"z"`
	Tid        int  `json:"tid"`
	EnMarche   bool `json:"en_marche"`
	TCycle     int  `json:"t_cycle"`
	FinDeCycle *int `json:"fin_de_cycle"`
	EnChantier bool `json:"en_chantier"`
	// ⚠️ EN POUR CENT (0-100). Plus de pour mille nulle part dans ce qui est
	// publie, affiche ou compare a un seuil (§3).
	//
	// ⚠️ RELUE A L'INSTANT : « ce qu'il a / ce qu'il attend ». Le releve
	// `Servi`/`Demande` du dernier cycle n'est pas sauvegarde, donc il vaudrait
	// 100 % apres chaque relecture — un ecran qui mentirait a chaque
	// rafraichissement. ⚠️ C'est l'ECRAN seulement : l'escalier, lui, reste le
	// trou §8bis A de la spec.
	Satisfaction int            `json:"satisfaction"`
	Stock        map[string]int `json:"stock"`
	Navettes     []NavetteVue   `json:"navettes"`
}

type PlateauVue struct {
	T       int            `json:"t"`
	Reserve map[string]int `json:"reserve"`
	Cases   []CaseVue      `json:"cases"`
}

// VueDuClient — ⚠️ le nom differe du JS (`vue`) parce que `Vue` est deja, ici,
// l'interface que lit le PLACEMENT. Deux choses qui n'ont rien a voir ne
// peuvent pas porter le meme nom dans un langage type.
func VueDuClient(p *Plateau, maintenant int) PlateauVue {
	reg := p.Genres.Reg
	out := PlateauVue{T: maintenant, Reserve: versNoms(reg, &p.Reserve)}
	for _, b := range p.Ordonnes() {
		servi, demande := MesurerIndice(p, b, maintenant, nil)
		c := CaseVue{X: b.X, Z: b.Z, Tid: b.Tuile.Tid,
			EnMarche: b.EnMarche, TCycle: b.TCycle,
			EnChantier:   b.EnChantier(maintenant),
			Satisfaction: PourCent(servi, demande),
			Stock:        versNoms(reg, &b.Stock)}
		if fin, ok := b.FinDeCycle(); ok {
			v := fin
			c.FinDeCycle = &v
		}
		for _, n := range b.Navettes {
			c.Navettes = append(c.Navettes, NavetteVue{
				Position: n.Position(maintenant), Origine: n.Origine,
				Destination: n.Destination, PartiA: n.PartiA, ArriveA: n.ArriveA,
				Sens: n.Sens, Charge: versNoms(reg, &n.Charge)})
		}
		out.Cases = append(out.Cases, c)
	}
	return out
}

// RegleVue : la flotte telle que la FICHE l'affiche (rayon + N, decision du
// 08/09). `Cibles` = les cases A PORTEE — geometrie et type, JAMAIS le stock :
// c'est entre elles que la flotte se partage quand elles ont quelque chose. Le
// client LIT ce chiffre, il ne refait pas le calcul.
//
// ⚠️ Le `Sens` rendu est celui du CATALOGUE (« entrant » / « envoi ») : c'est le
// mot du site et du jeu. « recolte » est le mot interne du moteur.
type RegleVue struct {
	Sens     string `json:"sens"`
	Rayon    int    `json:"rayon"`
	Illimite bool   `json:"illimite"`
	Navettes int    `json:"navettes"`
	EnVol    int    `json:"en_vol"`
	Cibles   int    `json:"cibles"`
}

func FlotteVue(p *Plateau, t int) map[string][]RegleVue {
	sortie := map[string][]RegleVue{}
	for _, b := range p.Ordonnes() {
		if len(b.Tuile.Appros) == 0 || !b.Vivant(t) {
			continue
		}
		var l []RegleVue
		for i := range b.Tuile.Appros {
			a := &b.Tuile.Appros[i]
			cibles := 0
			for _, m := range candidatsGeo(p, b, a) {
				if m.Vivant(t) {
					cibles++
				}
			}
			sens := "entrant"
			if a.Sens == "envoi" {
				sens = "envoi"
			}
			l = append(l, RegleVue{Sens: sens, Rayon: a.Rayon, Illimite: a.Illimite,
				Navettes: a.Navettes, EnVol: enVol(b, i), Cibles: cibles})
		}
		sortie[itoa(b.X)+":"+itoa(b.Z)] = l
	}
	return sortie
}
