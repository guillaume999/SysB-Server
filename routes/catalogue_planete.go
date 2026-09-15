package routes

// ============================================================================
//  catalogue_planete.go — UN CATALOGUE PAR PLANETE (15/09).
//
//  Jusqu'ici le serveur chargeait TOUTES les tuiles, ressources et technos pour
//  juger n'importe quel plateau. Depuis que les joueurs concoivent les leurs,
//  ca ne tient plus : la tuile d'un joueur se poserait chez tout le monde, et
//  deux planetes peuvent porter chacune leur « bois ».
//
//  ⚠️⚠️ LA REGLE (decision de Guillaume, 15/09 — « il n'a que les siennes ») :
//
//	planete de JOUEUR → ce qui est range sur CETTE planete, et rien d'autre ;
//	planete GAME      → ce qui est range sur cette planete + sur « Game »
//	                    (le contenu commun) + ce qui n'est range nulle part
//	                    (un record d'avant le patch des planetes).
//
//  ⚠️ LA TERRE NE VOIT PAS LES TUILES DE JUPITER (decision du 13/09 : « le
//  magasin filtre par monde »). Une case d'une autre planete n'est pas effacee :
//  `ChargerPartie` la met de cote (`Figes`) et la reecrit telle quelle.
//
//  ⚠️ CE QUI NE CHANGE PAS : un `tileId` reste UNIQUE GLOBALEMENT. Le catalogue
//  filtre ce qui JOUE ; il ne rend pas deux tuiles au meme numero.
//
//  ⚠️ SANS PLANETES EN BASE (patch pas passe), on rend TOUT : l'ancien
//  comportement. Une nouveaute de schema ne doit pas rendre le serveur muet.
// ============================================================================

import (
	"sysb/moteur"
)

// NomGame : le porte-contenu commun (`patch-planetes-2026-09-14.js`). Meme
// valeur que `NOM_GAME` cote site.
const NomGame = "Game"

// PlanetesDuCatalogue — les valeurs de `planete` dont les records entrent dans
// le catalogue de `planeteId`. `tout = true` : pas de filtre du tout.
func PlanetesDuCatalogue(planetes []moteur.Enregistrement, planeteId string) (garde map[string]bool, tout bool) {
	if len(planetes) == 0 {
		return nil, true
	}
	proprietaire := ""
	trouvee := false
	idGame := ""
	for _, p := range planetes {
		id := moteur.Texte(moteur.Champ(p, "id"))
		prop := moteur.Texte(moteur.Champ(p, "proprietaire"))
		if prop == "" && moteur.Texte(moteur.Champ(p, "nom")) == NomGame {
			idGame = id
		}
		if planeteId != "" && id == planeteId {
			trouvee = true
			proprietaire = prop
		}
	}
	if trouvee && proprietaire != "" {
		// ⚠️ CHEZ UN JOUEUR : sa planete, et SEULEMENT elle.
		return map[string]bool{planeteId: true}, false
	}
	// Planete game, ou plateau d'avant les planetes (planete vide ou inconnue) :
	// le jeu de l'administrateur.
	garde = map[string]bool{"": true}
	if idGame != "" {
		garde[idGame] = true
	}
	if trouvee {
		garde[planeteId] = true
		return garde, false
	}
	// ⚠️ UN PLATEAU SANS PLANETE CONNUE lit TOUTES les planetes game — le
	// comportement d'avant, borne au jeu : il ne voit jamais un contenu de
	// joueur.
	for _, p := range planetes {
		if moteur.Texte(moteur.Champ(p, "proprietaire")) == "" {
			garde[moteur.Texte(moteur.Champ(p, "id"))] = true
		}
	}
	return garde, false
}

// SourceFiltree : une source de records qui ne rend que ceux dont `planete` est
// gardee. `tuiles`, `ressources` et `technologies` sont filtrees ; toute autre
// collection passe telle quelle.
type SourceFiltree struct {
	Src   moteur.SourceRecords
	Garde map[string]bool
}

var collectionsParPlanete = map[string]bool{"tuiles": true, "ressources": true, "technologies": true}

func (s SourceFiltree) Tous(collection string) []moteur.Enregistrement {
	tous := s.Src.Tous(collection)
	if s.Garde == nil || !collectionsParPlanete[collection] {
		return tous
	}
	out := make([]moteur.Enregistrement, 0, len(tous))
	for _, r := range tous {
		if s.Garde[moteur.Texte(moteur.Champ(r, "planete"))] {
			out = append(out, r)
		}
	}
	return out
}

// SourceMemoire : lit chaque collection UNE fois, puis la rend de memoire. Le
// catalogue global et ceux des planetes partagent ainsi la meme lecture.
type SourceMemoire struct {
	Src  moteur.SourceRecords
	deja map[string][]moteur.Enregistrement
}

func NouvelleSourceMemoire(src moteur.SourceRecords) *SourceMemoire {
	return &SourceMemoire{Src: src, deja: map[string][]moteur.Enregistrement{}}
}

func (s *SourceMemoire) Tous(collection string) []moteur.Enregistrement {
	if l, ok := s.deja[collection]; ok {
		return l
	}
	l := s.Src.Tous(collection)
	s.deja[collection] = l
	return l
}

// Catalogues : le catalogue global et ceux des planetes, faits a la demande et
// gardes le temps d'une requete.
//
// ⚠️ LE GLOBAL SERT AU GARDE-FOU (`gardeCatalogue`) : une planete de joueur
// neuve a un catalogue VIDE, et c'est normal — ce n'est pas une base illisible.
type Catalogues struct {
	src        *SourceMemoire
	planetes   func() ([]moteur.Enregistrement, error)
	global     *moteur.CatalogueCharge
	parGarde   map[string]*moteur.CatalogueCharge
	lues       []moteur.Enregistrement
	luesFaites bool
}

func NouveauxCatalogues(src moteur.SourceRecords, planetes func() ([]moteur.Enregistrement, error)) *Catalogues {
	return &Catalogues{src: NouvelleSourceMemoire(src), planetes: planetes,
		parGarde: map[string]*moteur.CatalogueCharge{}}
}

func (c *Catalogues) Global() *moteur.CatalogueCharge {
	if c.global == nil {
		c.global = moteur.ChargerCatalogue(c.src)
	}
	return c.global
}

func (c *Catalogues) De(planeteId string) *moteur.CatalogueCharge {
	if !c.luesFaites {
		c.luesFaites = true
		if c.planetes != nil {
			if l, err := c.planetes(); err == nil {
				c.lues = l
			}
		}
	}
	garde, tout := PlanetesDuCatalogue(c.lues, planeteId)
	if tout {
		return c.Global()
	}
	cle := cleDeGarde(garde)
	if cat, ok := c.parGarde[cle]; ok {
		return cat
	}
	cat := moteur.ChargerCatalogue(SourceFiltree{Src: c.src, Garde: garde})
	c.parGarde[cle] = cat
	return cat
}

func cleDeGarde(g map[string]bool) string {
	ids := make([]string, 0, len(g))
	for id := range g {
		ids = append(ids, id)
	}
	for i := 1; i < len(ids); i++ {
		for j := i; j > 0 && ids[j] < ids[j-1]; j-- {
			ids[j], ids[j-1] = ids[j-1], ids[j]
		}
	}
	cle := ""
	for _, id := range ids {
		cle += id + "|"
	}
	return cle
}
