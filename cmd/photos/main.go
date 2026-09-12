// cmd/photos — l'etat COMPLET de chaque scenario de temps, pour diffuser Go / JS.
//
// ⚠️ C'EST LE CONTROLE QUI COMPTE. Les 45 mesures du banc sont une passoire :
// elles regardent une poignee de coffres. Ici on compare `ecrireEtats` + la
// reserve, c'est-a-dire TOUT ce que le moteur sait — chaque coffre, chaque
// `t_cycle`, chaque `en_marche`, chaque navette en vol et sa cale.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"sysb/harnais"
	"sysb/moteur"
)

func main() {
	d := harnais.LireJson(os.Args[1])
	genres, tuiles, erreurs := harnais.ChargerCatalogue(d)
	sortie := map[string]any{}

	for _, sv := range harnais.Liste(d, "scenarios") {
		s, _ := sv.(moteur.Brut)
		nom, _ := s["nom"].(string)
		// PLACEMENT : le verdict et le devis, mot pour mot.
		if _, y := s["placement"]; y {
			r := harnais.JouerPlacement(d, tuiles, s)
			var lignes []any
			for _, l := range r.Devis.Lignes {
				lignes = append(lignes, []any{genres.Reg.Nom(l.Ressource), l.Quantite, l.Mode})
			}
			sortie[nom] = map[string]any{
				"blocages": r.Verdict.Blocages, "avert": r.Verdict.Avertissements,
				"offert": r.Devis.Offert, "offerts": r.Devis.Offerts, "deja": r.Devis.Deja,
				"interdiction": r.Devis.Interdiction, "lignes": lignes,
			}
			continue
		}
		// GESTE : le compte rendu ET l'etat qu'il laisse.
		if _, y := s["geste"]; y {
			j, err := harnais.JouerGeste(d, genres, tuiles, erreurs, s)
			if err != nil {
				sortie[nom] = "CASSE: " + err.Error()
				continue
			}
			bloc := map[string]any{"etats": moteur.EcrireEtats(j.Plateau),
				"reserve": sacNoms(genres, &j.Plateau.Reserve)}
			if j.EstPose {
				bloc["ok"] = j.Pose.Ok
				bloc["refus"] = j.Pose.Refus
				bloc["blocages"] = j.Pose.Blocages
				bloc["avert"] = j.Pose.Avertissements
				bloc["paye"] = sacNoms(genres, &j.Pose.Paye)
				bloc["mobilise"] = sacNoms(genres, &j.Pose.Mobilise)
				bloc["perdu"] = sacNoms(genres, &j.Pose.Perdu)
				bloc["offert"] = j.Pose.Offert
				bloc["chantier"] = j.Pose.Chantier
			} else {
				bloc["ok"] = j.Destr.Ok
				bloc["refus"] = j.Destr.Refus
				bloc["avert"] = j.Destr.Avertissements
				bloc["perdu"] = sacNoms(genres, &j.Destr.Perdu)
				bloc["rendu"] = sacNoms(genres, &j.Destr.Rendu)
				bloc["ancien"] = j.Destr.Ancien
				bloc["apres"] = j.Destr.Apres
			}
			sortie[nom] = bloc
			continue
		}
		duree := 3600
		if v, ok := s["duree"].(float64); ok {
			duree = int(v)
		}
		p, err := harnais.Monter(genres, tuiles, erreurs, s)
		if err != nil {
			sortie[nom] = "CASSE: " + err.Error()
			continue
		}
		if err := moteur.Avancer(p, duree); err != nil {
			sortie[nom] = "CASSE: " + err.Error()
			continue
		}
		res := map[string]int{}
		for _, c := range p.Reserve.NonNuls() {
			if q := p.Reserve.Get(c); q != 0 {
				res[genres.Reg.Nom(c)] = q
			}
		}
		sortie[nom] = map[string]any{"etats": moteur.EcrireEtats(p), "reserve": res, "t": p.T}
	}
	b, _ := json.Marshal(sortie)
	fmt.Println(string(b))
}

func sacNoms(g *moteur.Genres, s *moteur.Sac) map[string]int {
	out := map[string]int{}
	for _, c := range s.NonNuls() {
		if q := s.Get(c); q != 0 {
			out[g.Reg.Nom(c)] = q
		}
	}
	return out
}
