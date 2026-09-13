package moteur

import "testing"

// ⚠️ CE QUE `lecture.go` DOIT AVALER : les formes que l'ADAPTATEUR lui livre.
// Ni plus, ni moins — voir l'en-tete de `lecture.go` et `pb/normaliser.go`.
func TestLireJsonAvaleLesFormesLivrees(t *testing.T) {
	// Un champ json de PocketBase arrive ici en octets NUS : `pb.Normaliser` a
	// deja retire le type nomme.
	l := ListeJson([]byte(`[{"niveau":1,"cycle_minutes":2}]`))
	if len(l) != 1 {
		t.Fatalf("des octets doivent se lire : %v", l)
	}
	if m, ok := l[0].(map[string]any); !ok || m["cycle_minutes"] != 2.0 {
		t.Errorf("contenu perdu : %v", l[0])
	}
	if o := ObjetJson(`{"or":250}`); o == nil || o["or"] != 250.0 {
		t.Error("une chaine doit passer")
	}
	if o := ObjetJson(map[string]any{"or": 250.0}); o == nil || o["or"] != 250.0 {
		t.Error("un objet deja decode doit passer")
	}
	// ⚠️ Un champ VIDE (jamais rempli) se lit nil, sans bruit — un plateau qui
	// refuse de se charger est pire qu'un plateau sans etats.
	if ObjetJson([]byte(``)) != nil || ObjetJson(nil) != nil || ObjetJson(" ") != nil {
		t.Error("un champ vide se lit nil")
	}
}

// ⚠️⚠️ LA FRONTIERE, MISE SOUS TEST — ce test dit « ce n'est PAS le travail
// d'ici », et il est la pour le rester.
//
// Un `types.JSONRaw` (type nomme sur []byte) n'est PAS lu ici, volontairement :
// c'est `pb.Normaliser` qui le ramene a des octets, du cote PocketBase de la
// frontiere. Si un jour quelqu'un « rajoute un filet au cas ou » dans
// `lecture.go`, ce test tombera — et c'est le but : deux endroits qui reparent
// la meme chose, c'est le jour ou l'un des deux ment. La panne du 12/09 se
// repare a UN endroit, pas a deux.
func TestUnTypeNommeNestPasLeTravailDuMoteur(t *testing.T) {
	type jsonRaw []byte
	if ListeJson(jsonRaw(`[1,2]`)) != nil {
		t.Error("⚠️ le moteur s'est mis a normaliser : la reparation doit rester " +
			"dans pb.Normaliser, et a un seul endroit")
	}
}

func TestTexteEtEntierSurLesFormesLivrees(t *testing.T) {
	if Texte("colonie") != "colonie" || Texte([]byte("colonie")) != "colonie" {
		t.Error("texte et octets se lisent tous les deux")
	}
	if Texte(nil) != "" || Texte(struct{}{}) != "" {
		t.Error("ce qui n'est pas du texte rend la chaine vide")
	}
	// PocketBase rend ses nombres en float64.
	if Entier(4.0, 0) != 4 || Entier(int64(4), 0) != 4 || Entier("4", 0) != 4 {
		t.Error("les trois formes de nombre se lisent")
	}
	if Entier(nil, 7) != 7 || Entier(struct{}{}, 7) != 7 || Entier("", 7) != 7 {
		t.Error("ce qui n'est pas un nombre rend le defaut")
	}
}
