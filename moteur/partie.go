// ============================================================
//  moteur/partie.go — CE QUE LES QUATRE ROUTES FONT PAREIL
//
//  `etat`, `passe`, `geste` et `recherche` repetaient toutes la meme chose :
//  charger le catalogue, lire le record, rattraper jusqu'a l'heure du serveur,
//  verifier avant d'ecrire, et rendre LE MEME bloc au client. Quatre copies,
//  c'etaient quatre facons de diverger — et elles avaient deja diverge (le bloc
//  `ressources` du 08/09 n'etait dans que trois d'entre elles).
//
//  ⚠️ LE BLOC RENDU EST LE CONTRAT AVEC UNITY. Une seule fonction l'ecrit :
//  `Bloc()`. Si un champ doit changer, il change ici, et les quatre routes le
//  disent de la meme facon le meme jour.
//
//  ⚠️ CE FICHIER NE DECIDE RIEN. Les regles du jeu sont dans le moteur, les
//  regles de pose dans `placement.go`, le geste dans `geste.go`. Ici il n'y a
//  que de la plomberie — et les GARDE-FOUS d'avant ecriture, qui ne sont pas
//  des regles mais des refus de se tromper en silence.
// ============================================================

package moteur

import (
	"fmt"
	"time"
)

// Maintenant : L'HEURE DU SERVEUR, en secondes entieres.
//
// ⚠️ C'est la SEULE horloge du jeu (§1) : le client predit entre deux reponses
// et se recale sur l'en-tete HTTP `Date` de chacune. Avancer la date du
// telephone ne doit rien donner.
func Maintenant() int { return int(time.Now().Unix()) }

// TechnosDe : les technos du joueur, `{ code: niveau }`.
//
// ⚠️ UN CHAMP json VIDE N'ARRIVE PAS EN `null` cote goja : il revenait en OBJET
// GO, et `Object.keys` rendait alors les NOMS DE METHODES du wrapper — pris
// pour cinq technos a niveau 0 le 06/09. En Go le probleme ne se pose pas, mais
// la regle qui en decoule reste : un niveau est un NOMBRE, et ce qui n'en est
// pas un n'est pas une techno.
func TechnosDe(r Enregistrement) map[string]int {
	out := map[string]int{}
	for code, v := range ObjetJson(Champ(r, "technos")) {
		if f, ok := v.(float64); ok {
			out[code] = int(f)
		}
	}
	return out
}

// Avancer la partie. Le rattrapage se fait EN MEMOIRE : c'est l'appelant qui
// decide d'ecrire (`passe`, `geste`, `recherche`) ou pas (`etat`).
//
// ⚠️ ON NE REJOUE JAMAIS DEPUIS LE DEBUT (§1) : on va du `T` du plateau a
// maintenant, et c'est tout.
//
// `budget` = 0 : tout le chemin. Sinon, on s'arrete et `Progres.Fini` est faux
// — voir `AvancerBudget` et le §11.1 de la spec.
func (partie *Partie) Avancer(t, budget int) Progres {
	if t <= partie.Plateau.T {
		return Progres{Fini: true, T: partie.Plateau.T}
	}
	return AvancerBudget(partie.Plateau, t, budget)
}

// Verifier : LES GARDE-FOUS D'AVANT ECRITURE.
//
// ⚠️ Ils ne jugent pas le jeu : ils refusent d'ecrire un etat qui ne peut pas
// etre vrai. Les pannes les plus cheres de ce projet ont toutes REPONDU 200.
//
// `casesAttendues` : -1 = « autant qu'avant » (une passe), sinon le nombre
// attendu apres un geste.
func (partie *Partie) Verifier(casesAttendues int) error {
	etats := partie.VersEtats()
	avant := len(partie.EtatsAvant) + len(partie.Figes)
	vise := casesAttendues
	if vise < 0 {
		vise = avant
	}
	if len(etats) != vise {
		return fmt.Errorf("%s : %d cases lues, %d a ecrire (%d attendues)",
			partie.Id, avant, len(etats), vise)
	}
	for _, b := range partie.Plateau.Batiments {
		for _, c := range b.Stock.NonNuls() {
			if b.Stock.Get(c) < 0 {
				return fmt.Errorf("%s : stock negatif (%s = %d) en %d,%d",
					partie.Id, partie.Plateau.Genres.Reg.Nom(c), b.Stock.Get(c), b.X, b.Z)
			}
		}
		if b.TCycle < 0 {
			return fmt.Errorf("%s : t_cycle invalide en %d,%d", partie.Id, b.X, b.Z)
		}
	}
	for _, c := range partie.Plateau.Reserve.NonNuls() {
		if partie.Plateau.Reserve.Get(c) < 0 {
			return fmt.Errorf("%s : reserve negative (%s = %d)", partie.Id,
				partie.Plateau.Genres.Reg.Nom(c), partie.Plateau.Reserve.Get(c))
		}
	}
	if partie.Plateau.T < partie.TAvant {
		return fmt.Errorf("%s : le temps recule (%d -> %d)", partie.Id,
			partie.TAvant, partie.Plateau.T)
	}
	return nil
}

// Changement : ce qui a REELLEMENT bouge, pour pouvoir le montrer avant de
// l'ecrire.
type Changement struct {
	X        int            `json:"x"`
	Z        int            `json:"z"`
	Stock    map[string]int `json:"stock"`
	TCycle   int            `json:"t_cycle"`
	EnMarche bool           `json:"en_marche"`
	Navettes int            `json:"navettes"`
	Nouvelle bool           `json:"nouvelle,omitempty"`
}

// Changements — ⚠️ L'APPARIEMENT SE FAIT SUR (x, z), PAS SUR L'INDEX. Une passe
// ne touche pas a l'ordre de la liste, mais un GESTE si : detruire retire un
// element, et tous les suivants glissent d'un cran. Compare index par index, ce
// diff annoncerait alors des mouvements de stock qui n'ont jamais eu lieu —
// dans une reponse dont le seul role est de dire la verite sur ce qui va etre
// ecrit.
func (partie *Partie) Changements() []Changement {
	avant := map[[2]int]EtatEcrit{}
	for _, a := range partie.EtatsAvant {
		avant[[2]int{a.X, a.Z}] = a
	}
	var out []Changement
	for _, b := range EcrireEtats(partie.Plateau) {
		a, connue := avant[[2]int{b.X, b.Z}]
		if !connue {
			out = append(out, Changement{X: b.X, Z: b.Z, Stock: map[string]int{}, Nouvelle: true})
			continue
		}
		bouge := map[string]int{}
		codes := map[string]bool{}
		for c := range a.Stock {
			codes[c] = true
		}
		for c := range b.Stock {
			codes[c] = true
		}
		for c := range codes {
			if d := b.Stock[c] - a.Stock[c]; d != 0 {
				bouge[c] = d
			}
		}
		cycle := a.TCycle != b.TCycle || a.EnMarche != b.EnMarche
		navettes := len(a.Navettes) != len(b.Navettes)
		if len(bouge) > 0 || cycle || navettes {
			out = append(out, Changement{X: b.X, Z: b.Z, Stock: bouge,
				TCycle: b.TCycle, EnMarche: b.EnMarche, Navettes: len(b.Navettes)})
		}
	}
	return out
}

// ─── LE BLOC QUE LIT UNITY ──────────────────────────────────────────────────

type BlocPlateau struct {
	Id            string `json:"id"`
	Nom           string `json:"nom"`
	TypeOfPlateau string `json:"typeOfPlateau"`
	Largeur       int    `json:"largeur"`
	Hauteur       int    `json:"hauteur"`
	Version       int    `json:"version"`
	// ⚠️ LE SOL TEL QU'EN BASE : le rattrapage ne touche JAMAIS au terrain, seul
	// un geste change le sol.
	TilesBase64 string                `json:"tilesBase64"`
	T           int                   `json:"t"`
	Etats       []any                 `json:"etats"`
	Reserve     map[string]int        `json:"reserve"`
	Flotte      map[string][]RegleVue `json:"flotte"`
}

type BlocRessources struct {
	Depensable map[string]int `json:"depensable"`
	EnAttente  map[string]int `json:"en_attente"`
	Mobilise   map[string]int `json:"mobilise"`
	Libre      map[string]int `json:"libre"`
}

type Bloc struct {
	Plateau    BlocPlateau    `json:"plateau"`
	Cases      []CaseVue      `json:"cases"`
	Ressources BlocRessources `json:"ressources"`
	// ⚠️ EN POUR CENT, par code d'indicateur. Un code absent = personne ne le
	// publie, ce qui n'est PAS « 0 % ».
	IndicateursPourCent map[string]int `json:"indicateurs_pour_cent"`
	TechnosAcquises     map[string]int `json:"technos_acquises"`
}

// FaireBloc — ⚠️⚠️ LE CONTRAT AVEC UNITY, ecrit ICI et nulle part ailleurs.
//
//	Plateau.T            l'instant jusqu'ou cet etat est a jour (secondes
//	                     serveur). Le client PREDIT a partir de la, et se recale
//	                     sur l'en-tete HTTP `Date` (§1).
//	Plateau.Etats        le format du §10 : `t_cycle`, `en_marche`, `navettes`.
//	Cases[].Satisfaction EN POUR CENT (0-100), relue a l'instant.
//	Ressources           en ENTIERS D'UNITES REELLES — le 1/3600 a disparu.
func FaireBloc(r Enregistrement, partie *Partie, technos map[string]int, indicateurs map[string]int) Bloc {
	p := partie.Plateau
	t := p.T
	vue := VueDuClient(p, t)
	depensable := Disponible(p, t)
	occupe := Mobilise(p, t)
	libre := map[string]int{}
	for _, c := range occupe.NonNuls() {
		libre[p.Genres.Reg.Nom(c)] = Libre(depensable, occupe, c)
	}
	return Bloc{
		Plateau: BlocPlateau{
			Id: partie.Id, Nom: partie.Nom, TypeOfPlateau: partie.TypeOfPlateau,
			Largeur: partie.Largeur, Hauteur: partie.Hauteur,
			Version:     Entier(Champ(r, "version"), 0),
			TilesBase64: Texte(Champ(r, "tilesBase64")),
			T:           t,
			Etats:       partie.VersEtats(),
			Reserve:     VersReserve(p),
			// Par case qui transporte, cle "x:z" : une entree par regle d'appro.
			Flotte: FlotteVue(p, t),
		},
		// Ce que le client DESSINE : satisfaction en %, position des navettes,
		// fin de cycle. Rien de tout ca n'est range en base (§2bis, §5).
		Cases: vue.Cases,
		Ressources: BlocRessources{
			Depensable: versNoms(p.Genres.Reg, &depensable),
			EnAttente:  versNoms(p.Genres.Reg, ptr(EnAttente(p, t))),
			Mobilise:   versNoms(p.Genres.Reg, &occupe),
			Libre:      libre,
		},
		IndicateursPourCent: indicateurs,
		TechnosAcquises:     technos,
	}
}

func ptr(s Sac) *Sac { return &s }

// Indicateurs : LA SATISFACTION PUBLIEE, par code d'indicateur — la moyenne des
// satisfactions PONDEREE PAR LA POPULATION, en pour cent (§5, lecture B).
//
// ⚠️⚠️ C'EST UN CHIFFRE D'ECRAN, ET IL NE DIT PAS LA MEME CHOSE QUE L'ESCALIER.
// Ici on fait la moyenne des SATISFACTIONS ; l'escalier, lui, fait la moyenne
// des TRANCHES (`Σ pop × tranche(sat) / Σ pop`, voir `RendementEscalier`). Deux
// nombres differents sous le meme nom, et seul le second agit sur le jeu.
// Releve le 12/09, spec §11.3-H — a trancher.
//
// ⚠️ ET TOUS LES CODES D'INDICATEUR RECOIVENT LA MEME VALEUR : le jour ou il y
// en aura deux (satisfaction et foi, par exemple), l'ecran affichera le meme
// chiffre pour les deux. Meme trou, meme ligne de la spec.
func Indicateurs(p *Plateau, genresBruts map[string]string, t int) map[string]int {
	pop, cumul := 0, 0
	for _, b := range p.Ordonnes() {
		if !b.Vivant(t) {
			continue
		}
		servi, demande := MesurerIndice(p, b, t, nil)
		if demande <= 0 {
			continue
		}
		n := b.Tuile.PlacesLogees()
		if n <= 0 {
			continue
		}
		pop += n
		cumul += n * 100 * servi / demande
	}
	valeur := 100
	if pop > 0 {
		valeur = cumul / pop
	}
	sortie := map[string]int{}
	for code, genre := range genresBruts {
		if genre == "indicateur" {
			sortie[code] = valeur
		}
	}
	return sortie
}
