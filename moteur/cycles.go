// ============================================================
//  moteur/cycles.go — LE CYCLE, ET SON ORDRE (§2)
//
//    1. ARRIVEE      les livraisons et recoltes qui atterrissent MAINTENANT
//    2. INDICE       les indicateurs sont relus, a partir des satisfactions a jour
//    3. CONSO/PROD   le batiment mange et livre — DANS LE MEME GESTE
//    4. DEPART       les navettes qui repartent sont allouees et envoyees
//       puis on recommence.
//
//  ⚠️ LES INDICES SONT RELUS A CHAQUE CYCLE, pas une fois pour toutes.
//
//  ⚠️ L'ORDRE 1 AVANT 3 N'EST PAS COSMETIQUE. Deux versions sont mortes au test
//  pour l'avoir enfreint : les entrepots servis avant les consommateurs directs
//  donnaient une boulangerie a 60 pains en jouant et ZERO apres une heure.
//
//  ⚠️ L'ORDRE 2 AVANT 3 NON PLUS. La satisfaction evaluee apres la production
//  fait tourner a plein regime un batiment qui produit plus vite qu'il ne
//  mange — 150 unites d'ecart sur une heure, mesure le 25/08.
//
//  ⚠️ INDICE ET CONSO/PROD S'ENCHAINENT BATIMENT PAR BATIMENT (11/09), et non
//  deux passes globales.
//
//  ⚠️⚠️ QUAND CA DEBLOQUE, ON NE RATTRAPE PAS EN UN COUP. Un batiment arrete
//  une heure qui recoit enfin sa cargaison redemarre UN cycle, pas soixante.
// ============================================================

package moteur

// ─── LES GROUPES « EN DIRECT » (§4) ─────────────────────────────────────────
//
//  ⚠️⚠️ TOUS LES PRENEURS EN DIRECT D'UN MEME TYPE NE FONT QU'UN : ils
//  consomment et produisent COMME UN SEUL BATIMENT. Il n'y a donc AUCUN ordre a
//  departager — il n'y a qu'un seul preneur, et sa satisfaction est celle du
//  groupe entier.
//
//  ⚠️ « les consommateurs directs d'abord, les entrepots ensuite » (11/09) : le
//  groupe se sert AVANT la passe batiment-par-batiment.

type membreGroupe struct {
	B      *Batiment
	Besoin int
}

type Groupe struct {
	Tid            int
	Code           Code
	Demande, Servi int
	Membres        []membreGroupe
	Parts          map[int]int
}

// Groupes — ⚠️ UNE TRANCHE, PAS UNE MAP. Le JS parcourait `Object.keys(groupes)`
// dans l'ordre de CREATION, et cet ordre compte : chaque groupe PRELEVE sur le
// plateau, donc celui qui passe en premier se sert en premier. Une map Go se
// parcourt au hasard — ici ca aurait rendu deux resultats pour la meme colonie.
type Groupes struct {
	ordre []*Groupe
	index map[int64]*Groupe
}

func cleGroupe(tid int, c Code) int64 { return int64(tid)<<32 | int64(uint32(c)) }

func (g *Groupes) Get(tid int, c Code) *Groupe {
	if g == nil || g.index == nil {
		return nil
	}
	return g.index[cleGroupe(tid, c)]
}

func ConstruireGroupes(p *Plateau, lot []*Batiment, t int) *Groupes {
	var gs *Groupes
	for _, b := range lot {
		if !b.Tuile.aDirect {
			continue
		}
		prox := Facteur(p, b)
		for i := range b.Tuile.Utilisation {
			l := &b.Tuile.Utilisation[i]
			if !l.Direct {
				continue
			}
			besoin := QuantiteLigne(l, prox)
			if besoin <= 0 {
				continue
			}
			if gs == nil {
				gs = &Groupes{index: map[int64]*Groupe{}}
			}
			k := cleGroupe(b.Tuile.Tid, l.Ressource)
			g := gs.index[k]
			if g == nil {
				g = &Groupe{Tid: b.Tuile.Tid, Code: l.Ressource, Parts: map[int]int{}}
				gs.index[k] = g
				gs.ordre = append(gs.ordre, g)
			}
			g.Demande += besoin
			g.Membres = append(g.Membres, membreGroupe{B: b, Besoin: besoin})
		}
	}
	if gs == nil {
		return nil
	}
	for _, g := range gs.ordre {
		// (les membres arrivent deja dans l'ordre `(z, x)` : `lot` est trie)
		// Le groupe se sert MAINTENANT, sur tout le plateau, sans navette et
		// sans limite de distance.
		g.Servi = PreleverPlateau(p, g.Code, g.Demande, t, g.Tid)
		// La MATIERE se repartit, elle : au prorata, le reliquat aux premiers
		// dans l'ordre `(z, x)`. La SATISFACTION, elle, est celle du groupe
		// pour tout le monde — c'est le rapport qui compte, pas les miettes.
		distribue := 0
		for _, m := range g.Membres {
			part := 0
			if g.Demande > 0 {
				part = g.Servi * m.Besoin / g.Demande
			}
			g.Parts[m.B.clef] = part
			distribue += part
		}
		reste := g.Servi - distribue
		for _, m := range g.Membres {
			if reste <= 0 {
				break
			}
			g.Parts[m.B.clef]++
			reste--
		}
	}
	return gs
}

// ─── 2. INDICE ──────────────────────────────────────────────────────────────

// MesurerIndice : ce que le batiment PEUT consommer maintenant, rapporte a ce
// qu'il demande au maximum. `Servi` / `Demande` sont un RELEVE, pas une donnee
// du jeu (§5) : ils ne sont pas sauvegardes et se refont au cycle suivant.
func MesurerIndice(p *Plateau, b *Batiment, t int, groupes *Groupes) (servi, demande int) {
	acc := CreerRapport()
	prox := Facteur(p, b)
	for i := range b.Tuile.Utilisation {
		l := &b.Tuile.Utilisation[i]
		q := QuantiteLigne(l, prox) // §4bis : le palier est deja plafonne
		if q <= 0 {
			continue
		}
		// Le rapport de la ligne : `servi / demande`. Pour une ligne « en
		// direct » c'est celui du GROUPE (§4), sinon celui du coffre.
		servi, demande := 0, q
		if l.Direct {
			// ⚠️ Hors cycle (la LECTURE d'un etat, pour l'afficher), il n'y a
			// pas de groupe constitue : on lit ce que le plateau lui offrirait.
			if g := groupes.Get(b.Tuile.Tid, l.Ressource); g != nil {
				servi, demande = g.Servi, g.Demande
			} else {
				servi = SurLePlateau(p, l.Ressource, t, b.Tuile.Tid)
				if q < servi {
					servi = q
				}
			}
		} else {
			servi = Lire(p, b, l.Ressource, t)
			if q < servi {
				servi = q
			}
		}
		// ⚠️⚠️ §5.5 — UNE LIGNE BONUS N'ENTRE PAS DANS LA DEMANDE. Elle ajoute
		// son pourcentage au prorata, et c'est la seule facon de depasser 100.
		if l.Bonus > 0 {
			acc.AjouterBonus(servi, demande, l.Bonus)
		} else {
			acc.Ajouter(servi, demande, q)
		}
	}
	return acc.Servi(), acc.Demande()
}

// ─── 3. CONSO / PROD ────────────────────────────────────────────────────────

// ConsoProd — ⚠️ LE PRELEVEMENT SE FAIT EN MEME TEMPS QUE LA LIVRAISON (§4).
// C'est ce qui justifie l'horodatage unique : `TCycle` vaut pour les deux.
//
// Une ligne de production qui SUIT un indicateur est cadencee par l'escalier
// (§5) ; une ligne ordinaire l'est par la satisfaction PROPRE du batiment — il
// a 80 % de ce qu'il attend, il livre 80 %. Jamais les deux : ce serait compter
// la penurie deux fois.
//
// ⚠️ Les lignes BONUS se consomment comme les autres (elles coutent vraiment) ;
// elles ne changent que la satisfaction, et par elle l'escalier.
func ConsoProd(p *Plateau, b *Batiment, t int) {
	prox := Facteur(p, b) // §4bis — il plafonne TOUT le palier

	// Consommer. La part « en direct » a deja quitte le plateau (elle est dans
	// les groupes) : il n'y a rien a reprelever pour elle.
	for i := range b.Tuile.Utilisation {
		l := &b.Tuile.Utilisation[i]
		if l.Direct {
			continue
		}
		besoin := QuantiteLigne(l, prox)
		if besoin <= 0 {
			continue
		}
		veut := Lire(p, b, l.Ressource, t)
		if besoin < veut {
			veut = besoin
		}
		if veut > 0 {
			Retirer(p, b, l.Ressource, veut, t)
		}
	}

	// Produire, dans le meme geste.
	for i := range b.Tuile.Production {
		l := &b.Tuile.Production[i]
		plein := QuantiteLigne(l, prox)
		if plein <= 0 {
			continue
		}
		var q int
		switch {
		case l.Indicateur != "":
			q = plein * RendementEscalier(p, l, t) / 100
		case b.Demande > 0:
			// ⚠️ ARRONDI VERS LE BAS (11/09).
			// ⚠️⚠️ ET PLAFONNE A 100 % (13/09) : une ligne ordinaire ne livre
			// JAMAIS plus que sa quantite declaree, meme a 120 % de
			// satisfaction. Le surplus ne paie que par l'escalier, la ou une
			// tranche au-dessus de 100 le dit explicitement — sinon tout le
			// catalogue se mettrait a sur-produire d'un coup, et le debit
			// affiche sur la fiche cesserait d'etre un maximum.
			servi := b.Servi
			if servi > b.Demande {
				servi = b.Demande
			}
			q = plein * servi / b.Demande
		default:
			q = plein
		}
		if q > 0 {
			Ranger(p, b, l.Ressource, q, t)
		}
	}

	// Le cycle est consomme. Le suivant demarrera — ou pas — a la REPRISE.
	b.TCycle = t
	b.EnMarche = false
}

// ─── 5. REPRISE ─────────────────────────────────────────────────────────────

// DeQuoiTourner : un cycle DEMARRE quand les ressources necessaires sont
// presentes. Sinon IL ATTEND, et le batiment est inerte.
//
// ⚠️⚠️ « PRESENTES » VEUT DIRE TOUTES (§2, tranche le 11/09).
//
// ☑️ SAUF si la tuile coche « demarre avec ce qu'il y a » (`DemarrePartiel`).
//
// ⚠️⚠️ PORTE TEL QUEL, Y COMPRIS SA ZONE D'OMBRE (releve le 12/09, spec §11.3-A) :
// le mode partiel compare le TOTAL servi au total demande, pas ressource par
// ressource. Un four coche qui demande 20 ble + 5 bois, avec 100 ble et ZERO
// bois, DEMARRE quand meme. La spec dit « une ressource totalement absente
// arrete toujours le cycle » — le JS et le miroir Python disent le contraire
// tous les deux. Tant que ce n'est pas tranche, Go dit la meme chose qu'eux :
// c'est la seule facon que les 126 vecteurs restent comparables.
func DeQuoiTourner(p *Plateau, b *Batiment, t int) bool {
	prox := Facteur(p, b)
	demande, dispo := 0, 0
	for i := range b.Tuile.Utilisation {
		l := &b.Tuile.Utilisation[i]
		// ⚠️⚠️ §5.5 — UNE LIGNE BONUS NE BLOQUE JAMAIS UN CYCLE. Sans ce
		// `continue`, une habitation sans gibier cesserait d'entamer le
		// moindre cycle : le « en plus » deviendrait un « obligatoire », et
		// une colonie mourrait de faim faute de viande de luxe.
		if l.Bonus > 0 {
			continue
		}
		q := QuantiteLigne(l, prox)
		if q <= 0 {
			continue
		}
		demande += q
		var d int
		if l.Direct {
			d = SurLePlateau(p, l.Ressource, t, b.Tuile.Tid)
		} else {
			d = Lire(p, b, l.Ressource, t)
		}
		if q < d {
			d = q
		}
		dispo += d
	}
	if demande > 0 {
		if b.Tuile.DemarrePartiel {
			if dispo <= 0 {
				return false
			}
		} else if dispo < demande {
			return false
		}
	}

	produit, placeUtile := 0, 0
	for i := range b.Tuile.Production {
		l := &b.Tuile.Production[i]
		if QuantiteLigne(l, prox) <= 0 {
			continue
		}
		// ⚠️⚠️ UNE LIGNE QUI NE SE RANGE NULLE PART N'EST PAS UNE LIGNE PLEINE.
		// Un `indicateur` se CALCULE et un `mobilise` se compte sur les places
		// declarees : ni l'un ni l'autre n'occupe un coffre. Les compter ici
		// arretait le batiment POUR TOUJOURS — releve le 11/09 en rejouant S7.
		cl := p.Genres.de(l.Ressource)
		if cl != classeStock && cl != classeFluxStock {
			continue
		}
		produit++
		if Place(p, b, l.Ressource, t) > 0 {
			placeUtile++
		}
	}
	// ⚠️ COFFRE PLEIN EN FIN DE CYCLE : LE BATIMENT S'ARRETE (§7).
	return produit == 0 || placeUtile > 0
}

func Reprendre(p *Plateau, b *Batiment, t int) {
	if b.EnMarche || b.Tuile.DureeCycleS <= 0 || !b.Vivant(t) {
		return
	}
	if !DeQuoiTourner(p, b, t) {
		return
	}
	b.TCycle = t
	b.EnMarche = true
}

// ─── L'INSTANT ──────────────────────────────────────────────────────────────

func TraiterInstant(p *Plateau, t int) {
	tous := p.Ordonnes()

	// 1. ARRIVEE — pour TOUT LE MONDE avant que quiconque ne consomme.
	for _, b := range tous {
		if len(b.Navettes) > 0 && b.Vivant(t) {
			Arrivees(p, b, t)
		}
	}

	// Qui est a une frontiere de cycle MAINTENANT ?
	var lot []*Batiment
	for _, b := range tous {
		if !b.EnMarche || !b.Vivant(t) {
			continue
		}
		if fin, ok := b.FinDeCycle(); ok && fin == t {
			lot = append(lot, b)
		}
	}

	// Les preneurs en direct se servent d'abord, en groupe (§4).
	groupes := ConstruireGroupes(p, lot, t)

	// 2 et 3 — enchaines batiment par batiment, dans l'ordre `(z, x)`.
	for _, b := range lot {
		b.Servi, b.Demande = MesurerIndice(p, b, t, groupes)
		ConsoProd(p, b, t)
	}

	// 4. DEPART — juste avant le cycle suivant (§6).
	for _, b := range tous {
		if b.Vivant(t) {
			Departs(p, b, t)
		}
	}

	// 5. Et on recommence : qui peut repartir, repart.
	for _, b := range tous {
		Reprendre(p, b, t)
	}
}

// ProchaineRupture — ⚠️⚠️ LA PIECE CENTRALE DU MOTEUR (§2). On ne saute a
// travers le temps QU'A TRAVERS UN REGIME STABLE : des qu'un changement de
// regime tombe — une source qui se vide, un coffre qui se remplit, une navette
// qui arrive — on s'arrete a cet instant et on reprend cycle par cycle.
//
// Ici chaque frontiere de cycle EST une rupture, donc le saut analytique n'a
// pas lieu d'etre : on va d'evenement en evenement, et rien n'est approxime.
//
// ⚠️ C'EST CE CHOIX QUI COUTE CHER (spec §11.1) : le cout monte lineairement
// avec la duree d'absence. Le portage ne le change pas — il fait exactement ce
// que faisait le JS, pour que les vecteurs restent comparables. Le saut
// analytique, si on l'ecrit un jour, s'ecrit ICI.
func ProchaineRupture(p *Plateau, t int) int {
	prochaine := INFINI
	for _, b := range p.Batiments {
		if b.ChantierFin != nil && *b.ChantierFin > t && *b.ChantierFin < prochaine {
			prochaine = *b.ChantierFin
		}
		if b.EnMarche && b.Tuile.DureeCycleS > 0 {
			if fin := b.TCycle + b.Tuile.DureeCycleS; fin > t && fin < prochaine {
				prochaine = fin
			}
		}
		for _, n := range b.Navettes {
			if n.ArriveA > t && n.ArriveA < prochaine {
				prochaine = n.ArriveA
			}
		}
	}
	return prochaine
}

// LimiteEvenements : le plafond par defaut d'un `Avancer`. Ce n'est PAS une
// regle de jeu, c'est un budget de calcul.
const LimiteEvenements = 2000000

// Progres : jusqu'ou `Avancer` est alle, et s'il reste du chemin.
//
// ⚠️⚠️ C'EST LA REPONSE AU TROU DE LA SPEC §11.1, ET ELLE TIENT EN UN CHAMP.
// L'ancien code LEVAIT quand il depassait son plafond : la transaction etait
// annulee, donc `plateau.t` ne bougeait pas, donc l'appel suivant refaisait
// exactement le meme chemin jusqu'a la meme exception. Un plateau qui franchit
// ce seuil ne se rattrape plus JAMAIS — il repond 500 a chaque rafraichissement,
// pour toujours.
//
// Une garde doit DEGRADER, pas condamner. On s'arrete donc sur le budget, on
// rend le `T` REELLEMENT ATTEINT, et l'appelant l'ecrit. Le §8ter a deja
// tranche que « `passe` ecrit des que le temps a avance » : un rattrapage par
// tranches est donc deja compatible avec tout le reste du modele, il ne
// manquait que ce champ.
type Progres struct {
	Fini       bool // false = il reste du chemin, rappelle `Avancer`
	T          int  // l'instant reellement atteint — c'est LUI qu'on ecrit
	Evenements int
}

// AvancerBudget : de `p.T` a `jusqua`, sans depasser `maxEvenements` ruptures.
// `maxEvenements <= 0` = pas de budget.
//
// ⚠️⚠️ ON NE REJOUE JAMAIS LA PARTIE DEPUIS LE DEBUT (§1).
//
// ⚠️ LA PASSE D'ENTREE (depart + reprise a `p.T`) N'EST PAS UNE ENTORSE A
// L'INVARIANCE AUX CADENCES : ce qu'un batiment peut faire ne change QU'AUX
// RUPTURES. Comme depart et reprise tournent deja a chaque rupture, la passe
// d'entree est un NO-OP partout — sauf au tout premier appel sur un plateau
// neuf, ou elle met la colonie en marche.
//
// ⚠️ S'ARRETER SUR LE BUDGET EST SANS DANGER, et c'est le modele qui le
// garantit : rien ne se passe ENTRE deux ruptures. Un plateau arrete a la
// derniere rupture traitee est un etat parfaitement legal — le meme qu'une
// passe normale aurait laisse si le joueur s'etait connecte a cet instant-la.
func AvancerBudget(p *Plateau, jusqua, maxEvenements int) Progres {
	if jusqua < p.T {
		return Progres{Fini: true, T: p.T} // on ne recule pas
	}
	t := p.T
	tous := p.Ordonnes()
	for _, b := range tous {
		if b.Vivant(t) {
			Departs(p, b, t)
		}
	}
	for _, b := range tous {
		Reprendre(p, b, t)
	}

	evenements := 0
	for {
		prochaine := ProchaineRupture(p, t)
		if prochaine == INFINI || prochaine > jusqua {
			break
		}
		if maxEvenements > 0 && evenements >= maxEvenements {
			// ⚠️ ON ECRIT LE `T` ATTEINT, et on le dit. Pas d'exception : le
			// travail deja fait est bon, il doit etre garde.
			p.T = t
			return Progres{Fini: false, T: t, Evenements: evenements}
		}
		TraiterInstant(p, prochaine)
		t = prochaine
		evenements++
	}
	p.T = jusqua
	return Progres{Fini: true, T: jusqua, Evenements: evenements}
}

// Avancer : le cas courant — tout le chemin, sans budget.
//
// ⚠️ Il ne peut pas boucler sans fin : `ProchaineRupture` rend STRICTEMENT plus
// grand que `t` (une fin de cycle exige `DureeCycleS > 0`, une arrivee de
// navette exige `ArriveA > t`), donc `t` monte a chaque tour et finit par
// depasser `jusqua`. Le budget est un garde-fou de COUT, pas de terminaison.
func Avancer(p *Plateau, jusqua int) error {
	AvancerBudget(p, jusqua, 0)
	return nil
}
