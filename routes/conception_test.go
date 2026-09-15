package routes

import (
	"strings"
	"testing"

	"sysb/moteur"
)

func fiche(general bool, joueurs any, l, h, tu, re, te float64) moteur.Enregistrement {
	return record{"general": general, "joueurs": joueurs, "largeur_max": l, "hauteur_max": h,
		"tuiles_max": tu, "ressources_max": re, "technos_max": te}
}

func TestSansFicheOnNeCreeRien(t *testing.T) {
	l := LimitesDe("u1", nil)
	if l != (Limites{}) {
		t.Fatalf("attendu zero, recu %+v", l)
	}
	if RefusQuota("tuiles", 0, l) == "" {
		t.Error("sans limite, la creation doit etre refusee")
	}
}

func TestLaFicheGeneraleSAppliqueATous(t *testing.T) {
	l := LimitesDe("u9", []moteur.Enregistrement{fiche(true, []any{}, 80, 60, 10, 5, 3)})
	if l.TuilesMax != 10 || l.LargeurMax != 80 || l.Source != "general" {
		t.Fatalf("%+v", l)
	}
}

// ⚠️ La fiche personnelle REMPLACE la generale, meme plus petite : c'est ce qui
// permet de brider un joueur.
func TestLaFichePersonnelleRemplaceLaGenerale(t *testing.T) {
	fiches := []moteur.Enregistrement{
		fiche(true, nil, 80, 60, 10, 5, 3),
		fiche(false, []any{"u1"}, 20, 20, 2, 0, 0),
	}
	l := LimitesDe("u1", fiches)
	if l.TuilesMax != 2 || l.RessourcesMax != 0 || l.Source != "joueur" {
		t.Fatalf("u1 : %+v", l)
	}
	if LimitesDe("u2", fiches).TuilesMax != 10 {
		t.Fatal("u2 garde la generale")
	}
}

// Deux fiches personnelles : la plus large, champ par champ. Et une relation
// rendue en TEXTE (maxSelect 1) compte aussi.
func TestDeuxFichesLaPlusLarge(t *testing.T) {
	l := LimitesDe("u1", []moteur.Enregistrement{
		fiche(false, []any{"u1", "u2"}, 30, 10, 2, 9, 0),
		fiche(false, "u1", 10, 40, 5, 1, 4),
	})
	want := Limites{LargeurMax: 30, HauteurMax: 40, TuilesMax: 5, RessourcesMax: 9, TechnosMax: 4, Source: "joueur"}
	if l != want {
		t.Fatalf("%+v", l)
	}
	// Un nombre negatif ne vaut rien.
	if LimitesDe("u1", []moteur.Enregistrement{fiche(true, nil, -5, 1, 1, 1, 1)}).LargeurMax != 0 {
		t.Error("negatif = 0")
	}
}

func TestQuota(t *testing.T) {
	l := Limites{TuilesMax: 2}
	if RefusQuota("tuiles", 1, l) != "" {
		t.Error("1 sur 2 : permis")
	}
	if r := RefusQuota("tuiles", 2, l); !strings.Contains(r, "Limite atteinte") {
		t.Errorf("2 sur 2 : %q", r)
	}
	if RefusQuota("technologies", 0, l) == "" {
		t.Error("technos a 0 : refuse")
	}
}

// ⚠️ L'ESSAI QUI COMPTE : ecrire chez un autre, ou deplacer vers chez un autre.
func TestUnJoueurNEcritQueChezLui(t *testing.T) {
	cas := []struct {
		nom                                  string
		action, avant, pAvant, apres, pApres string
		permis                               bool
	}{
		{"cree chez lui", "create", "", "", "pSeb", "u1", true},
		{"cree sans planete", "create", "", "", "", "", false},
		{"cree sur la Terre", "create", "", "", "pTerre", "", false},
		{"cree chez Zoe", "create", "", "", "pZoe", "u2", false},
		{"cree sur une planete introuvable", "create", "", "", "pX", "?", false},
		{"modifie chez lui", "update", "pSeb", "u1", "pSeb", "u1", true},
		{"modifie une tuile du jeu", "update", "pTerre", "", "pTerre", "", false},
		{"demenage chez Zoe", "update", "pSeb", "u1", "pZoe", "u2", false},
		{"ramene chez lui", "update", "pZoe", "u2", "pSeb", "u1", false},
		{"supprime chez lui", "delete", "pSeb", "u1", "", "", true},
		{"supprime chez Zoe", "delete", "pZoe", "u2", "", "", false},
		{"action inconnue", "voler", "pSeb", "u1", "pSeb", "u1", false},
	}
	for _, c := range cas {
		r := RefusPlanete(c.action, "u1", c.avant, c.pAvant, c.apres, c.pApres)
		if (r == "") != c.permis {
			t.Errorf("%s : refus %q, attendu permis=%v", c.nom, r, c.permis)
		}
	}
}

func TestModeleDuJoueur(t *testing.T) {
	lim := Limites{LargeurMax: 10, HauteurMax: 10}
	permis := map[int]bool{300: true}
	grille := moteur.EcrireGrille([]int{0, 300, 0, 0})

	if r := RefusModele(2, 2, 2, 2, lim, grille, permis); r != "" {
		t.Errorf("sa tuile, taille inchangee : %q", r)
	}
	// Taille inchangee mais au-dessus de la limite : permis (modele fabrique en 60 x 60).
	grand := moteur.EcrireGrille(make([]int, 60*60))
	if r := RefusModele(60, 60, 60, 60, lim, grand, permis); r != "" {
		t.Errorf("taille d'origine : %q", r)
	}
	if r := RefusModele(60, 60, 61, 60, lim, moteur.EcrireGrille(make([]int, 61*60)), permis); !strings.Contains(r, "Taille refusee") {
		t.Errorf("agrandi au-dela : %q", r)
	}
	if r := RefusModele(60, 60, 10, 10, lim, moteur.EcrireGrille(make([]int, 100)), permis); r != "" {
		t.Errorf("reduit dans la limite : %q", r)
	}
	// ⚠️ Un cote a la fois : reduire la largeur d'un 60 x 60 avec une limite a 10.
	if r := RefusModele(60, 60, 10, 60, lim, moteur.EcrireGrille(make([]int, 600)), permis); r != "" {
		t.Errorf("reduire un seul cote : %q", r)
	}
	// Mais on ne regrandit pas au-dela de ce qu'on a.
	if r := RefusModele(10, 60, 11, 60, lim, moteur.EcrireGrille(make([]int, 660)), permis); r == "" {
		t.Error("regrandir au-dela de la limite : refuse")
	}
	if r := RefusModele(10, 60, 10, 61, lim, moteur.EcrireGrille(make([]int, 610)), permis); r == "" {
		t.Error("depasser la hauteur actuelle : refuse")
	}
	if r := RefusModele(2, 2, 0, 2, lim, "", permis); r == "" {
		t.Error("zero case : refuse")
	}
	// Une tuile du jeu peinte chez lui : refusee.
	if r := RefusModele(2, 2, 2, 2, lim, moteur.EcrireGrille([]int{1, 0, 0, 0}), permis); !strings.Contains(r, "pas une tuile de ta planete") {
		t.Errorf("tuile du jeu : %q", r)
	}
	// Une grille de travers : refusee.
	if r := RefusModele(2, 2, 2, 2, lim, moteur.OctetsVersBase64([]int{0, 0, 0}), permis); !strings.Contains(r, "ne correspond pas") {
		t.Errorf("grille fausse : %q", r)
	}
}

func TestProchainTileId(t *testing.T) {
	if n, r := ProchainTileId(255); n != 256 || r != "" {
		t.Errorf("255 -> %d %q", n, r)
	}
	if n, _ := ProchainTileId(-3); n != 1 {
		t.Errorf("base vide -> %d", n)
	}
	if _, r := ProchainTileId(moteur.TileIdMax); r == "" {
		t.Error("65535 atteint : refus")
	}
}

func TestLaVignetteDUnJoueur(t *testing.T) {
	ouverte := &IconeLue{Chemin: "Icones_Tuiles/ble", Usage: "tuile", Partage: Partageable{ToutesPlanetes: true}}
	fermee := &IconeLue{Chemin: "Icones_Tuiles/or", Usage: "tuile"}
	ressource := &IconeLue{Chemin: "Icones_Ressources/bois", Usage: "ressource", Partage: Partageable{JoueursAutorises: []string{"u1"}}}
	cas := []struct {
		nom        string
		collection string
		id         string
		icone      *IconeLue
		chemin     string
		permis     bool
	}{
		{"aucune vignette : chemin vide, quoi qu'il ait tape", "tuiles", "", nil, "", true},
		{"icone ouverte : son chemin est recopie", "tuiles", "i1", ouverte, "Icones_Tuiles/ble", true},
		{"relation cassee", "tuiles", "i9", nil, "", false},
		{"icone fermee", "tuiles", "i2", fermee, "", false},
		{"mauvaise categorie", "tuiles", "i3", ressource, "", false},
		{"ouverte au joueur par son nom", "ressources", "i3", ressource, "Icones_Ressources/bois", true},
	}
	for _, c := range cas {
		chemin, r := VignetteDeJoueur(c.collection, c.id, c.icone, "pSeb", "u1")
		if (r == "") != c.permis || chemin != c.chemin {
			t.Errorf("%s : chemin %q refus %q, attendu %q permis=%v", c.nom, chemin, r, c.chemin, c.permis)
		}
	}
}
