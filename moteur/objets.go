// ============================================================
//  moteur/objets.go — LES OBJETS SONT MATERIALISES (§2bis)
//
//  « tu dois materialiser en code chaque batiment chaque navette, tout ce qui
//    transporte ou stock des ressources » — 11/09.
//
//  Deux choses NE SONT PAS RANGEES ici, et c'est le coeur du §2bis :
//
//    · UN SEUL HORODATAGE DE CYCLE. `cycle_debut`, `t_conso` et `t_prod`
//      valaient toujours le meme instant : la consommation et la production se
//      font dans le meme geste (§4) et le cycle suivant demarre la. Trois
//      champs pour une verite, c'etaient trois facons de diverger.
//
//    · LA POSITION D'UNE NAVETTE SE CALCULE. La ranger serait ecrire deux fois
//      la meme verite — meme faute que les trois horodatages.
//
//  La satisfaction non plus n'est pas un champ d'etat : c'est un rapport
//  recalcule (§5). `Servi` / `Demande` sont le RELEVE du dernier INDICE.
// ============================================================

package moteur

import "sort"

// INFINI : la meme valeur qu'en JS (`Number.MAX_SAFE_INTEGER`), pour que les
// deux moteurs comparent les memes bornes.
const INFINI = 9007199254740991

// ─── La navette ─────────────────────────────────────────────────────────────

// Navette. `Sens` vaut « aller » ou « retour », et il ne dit PAS la meme chose
// selon la regle qui l'a envoyee :
//
//	recolte  aller = vide, elle va chercher    retour = chargee, elle rentre
//	envoi    aller = chargee, elle va livrer   retour = vide, elle rentre
//
// Dans les deux cas le RETOUR se termine chez le proprietaire et libere une
// place de flotte. Un voyage est ENTIER (§6) : on ne fait pas 0,4 aller-retour.
type Navette struct {
	Origine     [2]int
	Destination [2]int
	PartiA      int
	ArriveA     int
	Sens        string
	// L'index de la regle d'appro qui l'a envoyee. ⚠️ ECRIT EN BASE (11/09) : la
	// flotte se compte PAR REGLE, et le deviner depuis la destination serait
	// ambigu des que deux regles visent la meme case.
	//
	// ⚠️ PAS DE LISTE DE RESSOURCES SUR LA NAVETTE. Ce qu'elle a le droit de
	// prendre se RELIT dans sa regle, a l'arrivee.
	Regle  int
	Charge Sac
}

// Position : OU EST-ELLE ? Calcule, jamais range (§2bis).
//
//	position(t) = origine + (destination - origine) x (t - parti_a)/(arrive_a - parti_a)
//
// ⚠️ Le seul endroit du moteur qui rend des flottants, et il ne rend rien
// d'autre : aucune decision ne se prend dessus. C'est du dessin.
func (n *Navette) Position(maintenant int) [2]float64 {
	duree := n.ArriveA - n.PartiA
	if duree <= 0 {
		return [2]float64{float64(n.Destination[0]), float64(n.Destination[1])}
	}
	f := float64(maintenant-n.PartiA) / float64(duree)
	if f < 0 {
		f = 0
	}
	if f > 1 {
		f = 1
	}
	return [2]float64{
		float64(n.Origine[0]) + float64(n.Destination[0]-n.Origine[0])*f,
		float64(n.Origine[1]) + float64(n.Destination[1]-n.Origine[1])*f,
	}
}

// ─── Le batiment ────────────────────────────────────────────────────────────

// Batiment :
//
//	TCycle    LA DERNIERE FRONTIERE DE CYCLE
//	EnMarche  false = a l'arret DEPUIS TCycle
//	Stock     son coffre, ressource par ressource
//
// La fin du cycle en cours vaut `TCycle + duree`, et rien d'autre n'est a
// ranger (§2bis).
type Batiment struct {
	X, Z     int
	Tuile    *Tuile
	Actif    bool
	Niveau   int
	Stock    Sac
	TCycle   int
	EnMarche bool
	// `nil` = pas de chantier. Un chantier en cours rend le batiment INERTE.
	ChantierFin *int
	// Le releve du dernier INDICE — recalcule, jamais sauvegarde (§5).
	Servi, Demande int
	Navettes       []*Navette

	clef int // z * 100000 + x, calcule une fois
	// Le facteur de proximite, en cache pour la duree d'une passe : il ne
	// depend que du terrain et de la tuile (voir proximite.go).
	prox     [2]int
	proxFait bool
	// Les ressources consommees « en direct » — ne depend que de la tuile.
	directes     Drapeaux
	directesFait bool
}

func CreerBatiment(g *Genres, d Brut, tuile *Tuile) *Batiment {
	b := &Batiment{
		X: nombreOu(d, "x", 0), Z: nombreOu(d, "z", 0),
		Tuile: tuile, Actif: true, Niveau: 1,
		Stock: NouveauSac(g.Reg),
	}
	b.clef = b.Z*100000 + b.X
	if v, ok := d["actif"].(bool); ok {
		b.Actif = v
	}
	if n := nombreOu(d, "niveau", 0); n != 0 {
		b.Niveau = n
	}
	if st, ok := d["stock"].(Brut); ok {
		for k, v := range st {
			f, _ := v.(float64)
			b.Stock.Set(g.Reg.Inscrire(k), int(f))
		}
	}
	b.TCycle = nombreOu(d, "t_cycle", 0)
	b.EnMarche, _ = d["en_marche"].(bool)
	if cf := nombreOu(d, "chantier_fin", 0); cf != 0 {
		v := cf
		b.ChantierFin = &v
	}
	for _, nv := range liste(d, "navettes") {
		nb, ok := nv.(Brut)
		if !ok {
			continue
		}
		n := &Navette{PartiA: nombreOu(nb, "parti_a", 0), ArriveA: nombreOu(nb, "arrive_a", 0),
			Sens: "aller", Regle: -1, Charge: NouveauSac(g.Reg)}
		if texte(nb, "sens") == "retour" {
			n.Sens = "retour"
		}
		if l := entiers(nb, "origine"); len(l) >= 2 {
			n.Origine = [2]int{l[0], l[1]}
		}
		if l := entiers(nb, "destination"); len(l) >= 2 {
			n.Destination = [2]int{l[0], l[1]}
		}
		if c, ok := nb["charge"].(Brut); ok {
			for k, v := range c {
				f, _ := v.(float64)
				n.Charge.Set(g.Reg.Inscrire(k), int(f))
			}
		}
		if aChamp(nb, "regle") {
			n.Regle = nombreOu(nb, "regle", -1)
		}
		b.Navettes = append(b.Navettes, n)
	}
	return b
}

// Clef — ⚠️ L'ORDRE DE DEPARTAGE EST `(z, x)`. Sans ca, deux parties identiques
// divergent.
func (b *Batiment) Clef() int { return b.clef }

func (b *Batiment) DureeCycle() int { return b.Tuile.DureeCycleS }

// FinDeCycle : l'instant ou le cycle en cours se termine, ou `false` s'il n'y
// en a pas.
func (b *Batiment) FinDeCycle() (int, bool) {
	if !b.EnMarche {
		return 0, false
	}
	d := b.Tuile.DureeCycleS
	if d <= 0 {
		return 0, false
	}
	return b.TCycle + d, true
}

func (b *Batiment) EnChantier(maintenant int) bool {
	return b.ChantierFin != nil && maintenant < *b.ChantierFin
}

// Vivant : un batiment INERTE ne produit pas, ne consomme pas, ne recolte pas,
// n'envoie pas, ne loge personne et ne publie aucun indicateur.
func (b *Batiment) Vivant(maintenant int) bool {
	return b.Actif && (b.ChantierFin == nil || maintenant >= *b.ChantierFin)
}

func (b *Batiment) Population() int { return b.Tuile.PlacesLogees() }

// ─── Le plateau ─────────────────────────────────────────────────────────────

type CaseSol struct{ X, Z, Tid int }

// Plateau — ⚠️ LE PLATEAU PORTE `T` : l'instant jusqu'ou il est a jour.
// Avancer, c'est aller de `T` a maintenant, puis ecrire le nouveau `T`.
// **On ne rejoue jamais la partie depuis le debut** (§1).
type Plateau struct {
	T         int
	Batiments []*Batiment
	// La reserve unique du plateau : c'est la que vivent les `FluxStock`.
	Reserve Sac
	Genres  *Genres
	// ⚠️ LE MONDE, pour les proximites (§4bis). A defaut on reconstruit depuis
	// les batiments PLUS le `Sol` — et oublier le `Sol` fait retomber dans le
	// piege du 31/08 : une foret n'a rien a retenir, donc aucun etat, donc
	// invisible.
	MondeReel Monde
	Sol       []CaseSol

	mondeReconstruit Monde
	// ⚠️ LES CACHES D'UNE PASSE. Rien de ce qu'ils retiennent ne change pendant
	// un `Avancer` ; un GESTE, lui, les invalide — `ViderCaches`.
	ordonnes []*Batiment
	geo      map[*Appro]map[int][]*Batiment
	// Ce qui n'a pas trouve de place. On le dit tout haut plutot que de l'avaler.
	Pertes Sac
}

func CreerPlateau(t int, batiments []*Batiment, reserve map[string]int, g *Genres, monde Monde, sol []CaseSol) *Plateau {
	p := &Plateau{T: t, Batiments: batiments, Reserve: NouveauSac(g.Reg),
		Genres: g, MondeReel: monde, Sol: append([]CaseSol(nil), sol...),
		Pertes: NouveauSac(g.Reg)}
	for k, v := range reserve {
		p.Reserve.Set(g.Reg.Inscrire(k), v)
	}
	return p
}

// Ordonnes : les batiments dans l'ordre `(z, x)` — LE seul ordre du moteur.
//
// ⚠️ MIS EN CACHE : le JS retriait la liste entiere a chaque instant de rupture,
// et `Ordonnes` est appele plusieurs fois par instant. La liste ne change qu'a
// une destruction ou a une pose — `ViderCaches` s'en charge.
func (p *Plateau) Ordonnes() []*Batiment {
	if p.ordonnes == nil {
		l := append([]*Batiment(nil), p.Batiments...)
		sort.SliceStable(l, func(i, j int) bool { return l[i].clef < l[j].clef })
		p.ordonnes = l
	}
	return p.ordonnes
}

func (p *Plateau) BatimentEn(x, z int) *Batiment {
	for _, b := range p.Batiments {
		if b.X == x && b.Z == z {
			return b
		}
	}
	return nil
}

// ViderCaches : a appeler des que la LISTE des batiments ou le terrain change
// (un geste pose ou detruit). Une passe, elle, ne change rien de ce qui est en
// cache.
func (p *Plateau) ViderCaches() {
	p.ordonnes = nil
	p.geo = nil
	p.mondeReconstruit = nil
	for _, b := range p.Batiments {
		b.proxFait = false
	}
}

func (p *Plateau) perdre(c Code, q int) {
	if q > 0 {
		p.Pertes.Ajouter(c, q)
	}
}
