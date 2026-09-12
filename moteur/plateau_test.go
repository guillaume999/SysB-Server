package moteur

import (
	"math/rand"
	"testing"
)

// Le base64 fait le tour : ce qu'on ecrit, on le relit.
func TestBase64FaitLeTour(t *testing.T) {
	r := rand.New(rand.NewSource(20260912))
	for n := 0; n < 300; n++ {
		taille := r.Intn(200)
		src := make([]int, taille)
		for i := range src {
			src[i] = r.Intn(256)
		}
		relu := Base64VersOctets(OctetsVersBase64(src))
		if len(relu) != len(src) {
			t.Fatalf("taille %d -> %d", len(src), len(relu))
		}
		for i := range src {
			if src[i] != relu[i] {
				t.Fatalf("octet %d : %d != %d", i, src[i], relu[i])
			}
		}
	}
}

// ⚠️ Un caractere impossible fait RENONCER, il ne fait pas deviner : un plateau
// a moitie decode serait pire qu'un plateau vide.
func TestBase64IllisibleRendVide(t *testing.T) {
	if got := Base64VersOctets("AAAA§§AAAA"); got != nil {
		t.Errorf("attendu nil sur un caractere impossible, recu %v", got)
	}
	if got := Base64VersOctets(""); got != nil {
		t.Errorf("attendu nil sur une chaine vide, recu %v", got)
	}
	// Les blancs, eux, sont tolerees : PocketBase peut replier une longue ligne.
	a := Base64VersOctets("AAEC")
	b := Base64VersOctets("AA\nEC")
	if len(a) != 3 || len(b) != 3 || a[1] != b[1] {
		t.Errorf("un retour a la ligne ne devrait rien changer : %v vs %v", a, b)
	}
}

// ─── Le garde-fou du champ `t` ──────────────────────────────────────────────

// recordFactice : un record qui AVALE certains champs, exactement comme
// PocketBase avale en silence un champ absent de la collection.
type recordFactice struct {
	champs map[string]any
	avale  map[string]bool
}

func (r *recordFactice) Get(nom string) any { return r.champs[nom] }
func (r *recordFactice) Set(nom string, v any) {
	if r.avale[nom] {
		return // ⚠️ c'est LE comportement qu'on veut attraper
	}
	r.champs[nom] = v
}

func partiePourTest(t *testing.T) *Partie {
	g := CreerGenres(map[string]string{"ble": "stock"})
	ferme, err := ChargerTuile(g, Brut{"tid": 1.0, "nom": "ferme", "cycle_minutes": 1.0,
		"production": []any{Brut{"ressource": "ble", "quantite": 10.0}},
		"stockage":   Brut{"*": 1000.0}})
	if err != nil {
		t.Fatal(err)
	}
	Refiger(g, []*Tuile{ferme})
	b := CreerBatiment(g, Brut{"x": 0.0, "z": 0.0}, ferme)
	p := CreerPlateau(1000, []*Batiment{b}, nil, g, nil, nil)
	return &Partie{Plateau: p, TAvant: 1000}
}

func TestEcrireRefuseSiLeChampTNestPasRetenu(t *testing.T) {
	partie := partiePourTest(t)
	partie.Plateau.T = 4242

	bon := &recordFactice{champs: map[string]any{}, avale: map[string]bool{}}
	if err := partie.Ecrire(bon); err != nil {
		t.Fatalf("une collection complete ne doit pas lever : %v", err)
	}
	if Entier(bon.Get("t"), -1) != 4242 {
		t.Errorf("t mal ecrit : %v", bon.Get("t"))
	}

	// La collection n'a pas de champ `t` : PocketBase avale l'ecriture.
	muet := &recordFactice{champs: map[string]any{}, avale: map[string]bool{"t": true}}
	err := partie.Ecrire(muet)
	if err == nil {
		t.Fatal("un champ `t` avale DOIT lever — sans ca chaque passe repart du " +
			"meme instant, et la route repond 200 comme si tout allait bien")
	}
	if _, ok := err.(*ChampAbsent); !ok {
		t.Errorf("mauvais type d'erreur : %T", err)
	}
}

// ⚠️ UNE CASE DONT LA TUILE EST REFUSEE N'EST PAS EFFACEE : elle est mise de
// cote et reecrite telle quelle. Une faute de saisie sur le site ne doit pas
// vider les cases des joueurs au premier rafraichissement.
func TestUneTuileRefuseeGardeSaCase(t *testing.T) {
	g := CreerGenres(map[string]string{"ble": "stock"})
	ferme, err := ChargerTuile(g, Brut{"tid": 1.0, "nom": "ferme", "cycle_minutes": 1.0,
		"production": []any{Brut{"ressource": "ble", "quantite": 10.0}},
		"stockage":   Brut{"*": 1000.0}})
	if err != nil {
		t.Fatal(err)
	}
	Refiger(g, []*Tuile{ferme})

	cat := catalogueFactice{g: g, tuiles: map[int]*Tuile{1: ferme}} // le tid 2 est REFUSE
	// Un plateau 2x1 : la case 0 porte le tid 1, la case 1 le tid 2.
	tiles := OctetsVersBase64([]int{1, 2})
	r := &recordFactice{champs: map[string]any{
		"largeur": 2.0, "hauteur": 1.0, "tilesBase64": tiles, "t": 1000.0,
		"etats": `[{"x":0,"z":0,"stock":{"ble":5}},{"x":1,"z":0,"stock":{"ble":7}}]`,
	}, avale: map[string]bool{}}

	partie := ChargerPartie(r, cat, 1000)
	if len(partie.Plateau.Batiments) != 1 {
		t.Fatalf("un seul batiment doit jouer, %d ont ete montes", len(partie.Plateau.Batiments))
	}
	if len(partie.Figes) != 1 {
		t.Fatalf("la case refusee doit etre FIGEE, pas effacee (%d figes)", len(partie.Figes))
	}
	etats := partie.VersEtats()
	if len(etats) != 2 {
		t.Fatalf("les deux cases doivent repartir en base, %d sont ecrites", len(etats))
	}
}

type catalogueFactice struct {
	g      *Genres
	tuiles map[int]*Tuile
}

func (c catalogueFactice) TuilePour(tid, niveau int) *Tuile { return c.tuiles[tid] }
func (c catalogueFactice) Genres() *Genres                  { return c.g }
