// pose.go — les familles PLACEMENT et GESTE du banc.
//
// ⚠️ Meme decoupage qu'en JS (`rejeu.js`) : les trois familles ne se jugent pas
// pareil. `temps` monte un plateau et avance ; `placement` ne touche a aucune
// horloge et demande un verdict + un devis ; `geste` pose ou detruit UNE fois,
// a `maintenant`, et regarde ce que ca a fait au sol, aux etats et aux coffres.
package harnais

import (
	"strings"

	"sysb/moteur"
)

// CatalogueDe : les tuiles nommees par le bloc, indexees par tid.
func CatalogueDe(tuiles map[string]*moteur.Tuile, bloc moteur.Brut) map[int]*moteur.Tuile {
	cat := map[int]*moteur.Tuile{}
	for _, v := range Liste(bloc, "catalogue") {
		if nom, ok := v.(string); ok {
			if t := tuiles[nom]; t != nil {
				cat[t.Tid] = t
			}
		}
	}
	return cat
}

func tidDe(tuiles map[string]*moteur.Tuile, nom string) int {
	if t := tuiles[nom]; t != nil {
		return t.Tid
	}
	return 0
}

// VueDe : la vue d'un bloc (le plateau vise, ou un plateau de l'empire).
func VueDe(tuiles map[string]*moteur.Tuile, bloc moteur.Brut) moteur.Vue {
	var sol []moteur.CaseSol
	for _, v := range Liste(bloc, "sol") {
		sb, _ := v.(moteur.Brut)
		sol = append(sol, moteur.CaseSol{X: Entier(sb, "x"), Z: Entier(sb, "z"), Tid: Entier(sb, "tid")})
	}
	var cases []moteur.Brut
	for _, v := range Liste(bloc, "cases") {
		cb, _ := v.(moteur.Brut)
		nom, _ := cb["tuile"].(string)
		cases = append(cases, moteur.Brut{
			"x": float64(Entier(cb, "x")), "z": float64(Entier(cb, "z")),
			"tid":          float64(tidDe(tuiles, nom)),
			"chantier_fin": float64(Entier(cb, "chantier_fin")),
		})
	}
	typ, _ := bloc["type_de_plateau"].(string)
	return moteur.NouvelleVueDuSol(sol, cases, typ, Entier(bloc, "maintenant"))
}

func empireDe(tuiles map[string]*moteur.Tuile, bloc moteur.Brut) []moteur.Vue {
	var out []moteur.Vue
	for _, v := range Liste(bloc, "empire") {
		b, _ := v.(moteur.Brut)
		out = append(out, VueDe(tuiles, b))
	}
	return out
}

func technosAcquises(bloc moteur.Brut) map[string]int {
	out := map[string]int{}
	if m, ok := bloc["technos"].(moteur.Brut); ok {
		for k, v := range m {
			f, _ := v.(float64)
			out[k] = int(f)
		}
	}
	return out
}

func catalogueTechnos(d moteur.Brut, bloc moteur.Brut) []moteur.Techno {
	all, _ := d["technos"].(moteur.Brut)
	var out []moteur.Techno
	for _, v := range Liste(bloc, "technos_catalogue") {
		if nom, ok := v.(string); ok {
			if tb, y := all[nom].(moteur.Brut); y {
				out = append(out, moteur.ChargerTechno(tb))
			}
		}
	}
	return out
}

// ─── PLACEMENT ──────────────────────────────────────────────────────────────

type ResultatPlacement struct {
	Verdict moteur.Verdict
	Devis   moteur.Devis
}

func JouerPlacement(d moteur.Brut, tuiles map[string]*moteur.Tuile, s moteur.Brut) ResultatPlacement {
	bloc, _ := s["placement"].(moteur.Brut)
	cat := CatalogueDe(tuiles, bloc)
	vue := VueDe(tuiles, bloc)
	empire := empireDe(tuiles, bloc)
	pose, _ := bloc["pose"].(moteur.Brut)
	nom, _ := pose["tuile"].(string)
	tid := tidDe(tuiles, nom)
	niveau := Entier(pose, "niveau")
	if niveau < 1 {
		niveau = 1
	}
	technos := technosAcquises(bloc)
	return ResultatPlacement{
		Verdict: moteur.Verifier(vue, cat, Entier(pose, "x"), Entier(pose, "z"), tid, technos, empire),
		Devis:   moteur.Estimer(vue, cat, tid, niveau, empire, catalogueTechnos(d, bloc), technos),
	}
}

func MesurerPlacement(g *moteur.Genres, m moteur.Brut, r ResultatPlacement) (float64, bool) {
	typ, _ := m["type"].(string)
	switch typ {
	case "pose_permise":
		return bin(len(r.Verdict.Blocages) == 0), true
	case "blocages":
		return float64(len(r.Verdict.Blocages)), true
	case "avertissements":
		return float64(len(r.Verdict.Avertissements)), true
	case "devis_offert":
		return bin(r.Devis.Offert), true
	case "devis_interdit":
		return bin(r.Devis.Interdiction != ""), true
	case "interdiction_dit":
		motif, _ := m["motif"].(string)
		return bin(strings.Contains(r.Devis.Interdiction, motif)), true
	case "devis_paye", "devis_mobilise":
		mode := "paye"
		if typ == "devis_mobilise" {
			mode = "mobilise"
		}
		nom, _ := m["ressource"].(string)
		c := g.Reg.Id(nom)
		total := 0
		for _, l := range r.Devis.Lignes {
			if l.Mode == mode && l.Ressource == c {
				total += l.Quantite
			}
		}
		return float64(total), true
	}
	return 0, false
}

// ─── GESTE ──────────────────────────────────────────────────────────────────

type ResultatGeste struct {
	Terrain moteur.Terrain
	Plateau *moteur.Plateau
	Pose    moteur.ResultatPose
	Destr   moteur.ResultatDestruction
	EstPose bool
	T       int
}

func JouerGeste(d moteur.Brut, g *moteur.Genres, tuiles map[string]*moteur.Tuile, erreurs map[string]error, s moteur.Brut) (ResultatGeste, error) {
	bloc, _ := s["geste"].(moteur.Brut)
	cat := CatalogueDe(tuiles, bloc)
	t := Entier(bloc, "maintenant")
	if t == 0 {
		t = 1000
	}

	var bats []*moteur.Batiment
	for _, c := range Liste(bloc, "cases") {
		cb, _ := c.(moteur.Brut)
		nom, _ := cb["tuile"].(string)
		tu := tuiles[nom]
		if tu == nil {
			if e, y := erreurs[nom]; y {
				return ResultatGeste{}, e
			}
			return ResultatGeste{}, errTuile(nom)
		}
		bats = append(bats, moteur.CreerBatiment(g, cb, tu))
	}
	var sol []moteur.CaseSol
	for _, v := range Liste(bloc, "sol") {
		sb, _ := v.(moteur.Brut)
		sol = append(sol, moteur.CaseSol{X: Entier(sb, "x"), Z: Entier(sb, "z"), Tid: Entier(sb, "tid")})
	}
	reserve := map[string]int{}
	if m, ok := bloc["reserve"].(moteur.Brut); ok {
		for k, v := range m {
			f, _ := v.(float64)
			reserve[k] = int(f)
		}
	}
	p := moteur.CreerPlateau(t, bats, reserve, g, nil, sol)

	// ⚠️ LE SOL DU TERRAIN inclut les cases baties : c'est ce que fait le JS
	// (`terrainDesCases` complete la table depuis `plateau.batiments`).
	typ, _ := bloc["type_de_plateau"].(string)
	terrain := moteur.NouveauTerrainDesCases(sol, p, typ)

	opt := moteur.OptionsGeste{
		Technos:          technosAcquises(bloc),
		CatalogueTechnos: catalogueTechnos(d, bloc),
		Empire:           empireDe(tuiles, bloc),
	}
	a, _ := bloc["action"].(moteur.Brut)
	typA, _ := a["type"].(string)
	out := ResultatGeste{Terrain: terrain, Plateau: p, T: t}
	if typA == "poser" {
		nom, _ := a["tuile"].(string)
		out.EstPose = true
		out.Pose = moteur.Poser(terrain, cat, Entier(a, "x"), Entier(a, "z"), tidDe(tuiles, nom), t, opt)
	} else {
		out.Destr = moteur.DetruireCase(terrain, cat, Entier(a, "x"), Entier(a, "z"), t)
	}
	return out, nil
}

func MesurerGeste(g *moteur.Genres, m moteur.Brut, j ResultatGeste) (float64, bool) {
	typ, _ := m["type"].(string)
	nom, _ := m["ressource"].(string)
	c := g.Reg.Id(nom)
	switch typ {
	case "geste_ok":
		return bin(j.Pose.Ok || j.Destr.Ok), true
	case "geste_refuse":
		return bin(j.refus() != ""), true
	case "refus_dit":
		motif, _ := m["motif"].(string)
		return bin(strings.Contains(j.refus(), motif)), true
	case "geste_paye":
		return float64(j.Pose.Paye.Get(c)), true
	case "geste_mobilise":
		return float64(j.Pose.Mobilise.Get(c)), true
	case "geste_perdu":
		if j.EstPose {
			return float64(j.Pose.Perdu.Get(c)), true
		}
		return float64(j.Destr.Perdu.Get(c)), true
	case "geste_chantier":
		return float64(j.Pose.Chantier), true
	case "geste_offert":
		return bin(j.Pose.Offert), true
	case "avertissements":
		if j.EstPose {
			return float64(len(j.Pose.Avertissements)), true
		}
		return float64(len(j.Destr.Avertissements)), true
	case "sol_apres":
		return float64(j.Terrain.Tid(Entier(m, "x"), Entier(m, "z"))), true
	case "nb_etats":
		return float64(len(j.Plateau.Batiments)), true
	case "a_un_etat":
		return bin(j.Terrain.BatimentAt(Entier(m, "x"), Entier(m, "z")) != nil), true
	}
	return 0, false
}

func (j ResultatGeste) refus() string {
	if j.EstPose {
		return j.Pose.Refus
	}
	return j.Destr.Refus
}

func bin(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

type errTuileType string

func (e errTuileType) Error() string { return "tuile inconnue « " + string(e) + " »" }
func errTuile(n string) error        { return errTuileType(n) }
