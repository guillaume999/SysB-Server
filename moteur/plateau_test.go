package moteur

import (
	"encoding/json"
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

// ─── Un record en memoire ───────────────────────────────────────────────────

// recordFactice — ⚠️⚠️ IL A MENTI, ET SON MENSONGE A COUTE UN MOIS. Il avait un
// jeu `avale` : certains champs y refusaient le `Set`, « exactement comme
// PocketBase avale un champ absent de la collection ». **PocketBase ne fait pas
// ca** : `Record.Set` sur un champ inconnu retombe sur `SetRaw`, garde la valeur
// et la rend a `Get` (source v0.39.2) ; elle n'est perdue qu'au SAVE.
//
// L'essai qui s'appuyait dessus (`TestEcrireRefuseSiLeChampTNestPasRetenu`)
// etait donc vert sur une forme qui n'existe pas, pendant que la vraie
// collection `plateaux` n'avait pas de champ `t` et que plus rien ne produisait.
// Le garde-fou vit maintenant dans `routes.gardeSchema`, qui demande les champs
// au lieu de les deviner. ⚠️ NE PAS REMETTRE `avale` : un faux ne doit imiter
// que ce que son voisin immediat fait vraiment.
type recordFactice struct {
	champs map[string]any
}

func (r *recordFactice) Get(nom string) any { return r.champs[nom] }
func (r *recordFactice) Set(nom string, v any) {
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
	}}

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

// ⚠️ UN PLATEAU SANS AUCUN BATIMENT REND `[]`, JAMAIS `nil`. Une tranche nulle
// part en base en `null` : le moteur la relit, mais un lecteur qui fait
// `etats.map()` casse dessus. Vu en vrai le 13/09 sur le premier plateau
// fabrique par `assurer` (modele sans etats).
func TestUnPlateauVideEcritUneListeVide(t *testing.T) {
	g := CreerGenres(map[string]string{"ble": "stock"})
	p := CreerPlateau(1000, nil, nil, g,
		&MondeDuPlateau{Largeur: 1, Hauteur: 1, Tiles: []int{0}}, nil)
	partie := &Partie{Plateau: p}

	etats := partie.VersEtats()
	if etats == nil {
		t.Fatal("VersEtats rend nil — ca s'ecrit `null` en base, pas `[]`")
	}
	if len(etats) != 0 {
		t.Errorf("0 etat attendu, %d rendus", len(etats))
	}

	rec := &recordFactice{champs: map[string]any{}}
	partie.Ecrire(rec)
	if b, err := json.Marshal(rec.Get("etats")); err != nil || string(b) != "[]" {
		t.Errorf("ce qui part en base : %s (attendu [])", b)
	}
}

// ─── La grille sur 1 ou 2 octets (15/09) ────────────────────────────────────

// Une grille d'avant (1 octet par case) se relit TELLE QUELLE : c'est ce qui
// evite de vider les plateaux des joueurs.
func TestGrilleUnOctetSeRelitTelleQuelle(t *testing.T) {
	src := []int{0, 1, 255, 7, 0, 12}
	cases, ok := LireGrille(OctetsVersBase64(src), 3, 2)
	if !ok || len(cases) != 6 {
		t.Fatalf("attendu 6 cases lisibles, recu %v (ok=%v)", cases, ok)
	}
	for i := range src {
		if cases[i] != src[i] {
			t.Fatalf("case %d : %d != %d", i, cases[i], src[i])
		}
	}
}

// Deux octets, POIDS FORT D'ABORD — le format que le site et Unity suivent.
func TestGrilleDeuxOctetsPoidsFortDabord(t *testing.T) {
	// 300 = 0x012C, 65535 = 0xFFFF, 1 = 0x0001
	texte := OctetsVersBase64([]int{0x01, 0x2C, 0xFF, 0xFF, 0x00, 0x01, 0, 0})
	cases, ok := LireGrille(texte, 2, 2)
	if !ok {
		t.Fatal("grille de 2 octets refusee")
	}
	want := []int{300, 65535, 1, 0}
	for i := range want {
		if cases[i] != want[i] {
			t.Fatalf("case %d : %d != %d", i, cases[i], want[i])
		}
	}
}

// On ecrit en 1 octet tant que c'est possible, en 2 des qu'un id depasse 255,
// et ce qu'on ecrit se relit.
func TestEcrireGrilleChoisitLeFormatEtFaitLeTour(t *testing.T) {
	petit := []int{0, 1, 255, 3}
	if n := len(Base64VersOctets(EcrireGrille(petit))); n != 4 {
		t.Errorf("ids <= 255 : attendu 4 octets, recu %d", n)
	}
	r := rand.New(rand.NewSource(20260915))
	for essai := 0; essai < 200; essai++ {
		l, h := 1+r.Intn(20), 1+r.Intn(20)
		src := make([]int, l*h)
		for i := range src {
			src[i] = r.Intn(TileIdMax + 1)
		}
		src[r.Intn(len(src))] = 256 + r.Intn(TileIdMax-256)
		texte := EcrireGrille(src)
		if n := len(Base64VersOctets(texte)); n != 2*l*h {
			t.Fatalf("id > 255 : attendu %d octets, recu %d", 2*l*h, n)
		}
		relu, ok := LireGrille(texte, l, h)
		if !ok {
			t.Fatal("grille ecrite illisible")
		}
		for i := range src {
			if relu[i] != src[i] {
				t.Fatalf("case %d : %d != %d", i, relu[i], src[i])
			}
		}
	}
}

// Une longueur qui ne tombe sur aucun format n'est pas « lisible », mais elle ne
// fait pas planter : les octets reviennent tels quels.
func TestGrilleDeTravers(t *testing.T) {
	cases, ok := LireGrille(OctetsVersBase64([]int{1, 2, 3}), 2, 2)
	if ok {
		t.Error("3 octets pour 4 cases : ne doit pas etre lisible")
	}
	if len(cases) != 3 {
		t.Errorf("les octets doivent revenir tels quels, recu %v", cases)
	}
	if FormatGrille(0, 0, 0) != 0 {
		t.Error("une grille sans case n'a pas de format")
	}
	// Un id hors bornes s'ecrit case vide, jamais tronque.
	relu, _ := LireGrille(EcrireGrille([]int{70000, 300}), 2, 1)
	if relu[0] != 0 || relu[1] != 300 {
		t.Errorf("attendu [0 300], recu %v", relu)
	}
}

// Un plateau en 2 octets se charge : la tuile 300 joue sur sa case.
func TestChargerPartieLitDeuxOctets(t *testing.T) {
	g := CreerGenres(map[string]string{"ble": "stock"})
	tuile, err := ChargerTuile(g, Brut{"tid": 300.0, "nom": "grande ferme", "cycle_minutes": 1.0,
		"production": []any{Brut{"ressource": "ble", "quantite": 1.0}},
		"stockage":   Brut{"*": 10.0}})
	if err != nil {
		t.Fatal(err)
	}
	Refiger(g, []*Tuile{tuile})
	cat := &catalogueUnique{g: g, t: tuile}
	rec := &recordFactice{champs: map[string]any{
		"largeur": 2.0, "hauteur": 1.0,
		"tilesBase64": EcrireGrille([]int{300, 0}),
		"etats":       `[{"x":0,"z":0}]`, "t": 1000.0,
	}}
	p := ChargerPartie(rec, cat, 1000)
	if len(p.Plateau.Batiments) != 1 || len(p.Figes) != 0 {
		t.Fatalf("attendu 1 batiment joue, 0 fige ; recu %d / %d", len(p.Plateau.Batiments), len(p.Figes))
	}
	if p.Tiles[0] != 300 {
		t.Errorf("case 0 : attendu 300, recu %d", p.Tiles[0])
	}
}

type catalogueUnique struct {
	g *Genres
	t *Tuile
}

func (c *catalogueUnique) Genres() *Genres { return c.g }
func (c *catalogueUnique) TuilePour(tid, _ int) *Tuile {
	if tid == 300 {
		return c.t
	}
	return nil
}
