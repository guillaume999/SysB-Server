package routes

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// fauxDepot : une base de guildes en memoire.
type fauxDepot struct {
	t        int
	reglages ReglagesGuildes
	joueurs  map[string]string // id -> pseudo
	ages     map[string]int
	guildes  []Guilde
	membres  []Membre
	demandes []DemandeGuilde
	messages []MessageGuilde
	dernier  map[string]int // guilde|uid -> instant
	seq      int
	panne    bool
}

func nouveauDepot() *fauxDepot {
	return &fauxDepot{
		t:        1000,
		reglages: ReglagesGuildes{MembresMax: 3, OfficiersMax: 1, DelaiSalon: 10, AgeMinCreation: 2},
		joueurs:  map[string]string{"a": "Alice", "b": "Bob", "c": "Cid", "d": "Dan", "e": "Eve"},
		ages:     map[string]int{"a": 2, "b": 1, "c": 3},
		dernier:  map[string]int{},
	}
}

func (f *fauxDepot) id() string { f.seq++; return fmt.Sprintf("x%d", f.seq) }

func (f *fauxDepot) Maintenant() int { return f.t }
func (f *fauxDepot) Reglages() (ReglagesGuildes, error) {
	if f.panne {
		return ReglagesGuildes{}, errors.New("disque")
	}
	return f.reglages, nil
}
func (f *fauxDepot) NomJoueur(uid string) (string, bool, error) {
	n, ok := f.joueurs[uid]
	return n, ok, nil
}
func (f *fauxDepot) AgeDe(uid string) (int, error) { return f.ages[uid], nil }
func (f *fauxDepot) Guildes() ([]Guilde, error)    { return append([]Guilde(nil), f.guildes...), nil }
func (f *fauxDepot) Membres() ([]Membre, error)    { return append([]Membre(nil), f.membres...), nil }
func (f *fauxDepot) Demandes() ([]DemandeGuilde, error) {
	return append([]DemandeGuilde(nil), f.demandes...), nil
}
func (f *fauxDepot) GuildeParId(id string) (*Guilde, error) {
	for _, g := range f.guildes {
		if g.Id == id {
			return &g, nil
		}
	}
	return nil, nil
}
func (f *fauxDepot) InstantDernierMessage(g, uid string) (int, error) {
	return f.dernier[g+"|"+uid], nil
}
func (f *fauxDepot) MessagesSalon(g, apres string, limite int) ([]MessageGuilde, error) {
	var out []MessageGuilde
	for _, m := range f.messages {
		if m.Guilde == g && m.Cree > apres {
			out = append(out, m)
		}
	}
	return out, nil
}
func (f *fauxDepot) CreerGuilde(g *Guilde) error {
	g.Id = f.id()
	f.guildes = append(f.guildes, *g)
	return nil
}
func (f *fauxDepot) SauverGuilde(g Guilde) error {
	for i := range f.guildes {
		if f.guildes[i].Id == g.Id {
			f.guildes[i] = g
		}
	}
	return nil
}
func (f *fauxDepot) SupprimerGuilde(id string) error {
	var g []Guilde
	for _, x := range f.guildes {
		if x.Id != id {
			g = append(g, x)
		}
	}
	f.guildes = g
	var m []Membre
	for _, x := range f.membres {
		if x.Guilde != id {
			m = append(m, x)
		}
	}
	f.membres = m
	var d []DemandeGuilde
	for _, x := range f.demandes {
		if x.Guilde != id {
			d = append(d, x)
		}
	}
	f.demandes = d
	return nil
}
func (f *fauxDepot) AjouterMembre(m *Membre) error {
	m.Id = f.id()
	f.membres = append(f.membres, *m)
	return nil
}
func (f *fauxDepot) SauverMembre(m Membre) error {
	for i := range f.membres {
		if f.membres[i].Id == m.Id {
			f.membres[i] = m
		}
	}
	return nil
}
func (f *fauxDepot) RetirerMembre(id string) error {
	var m []Membre
	for _, x := range f.membres {
		if x.Id != id {
			m = append(m, x)
		}
	}
	f.membres = m
	return nil
}
func (f *fauxDepot) CreerDemande(d *DemandeGuilde) error {
	d.Id = f.id()
	f.demandes = append(f.demandes, *d)
	return nil
}
func (f *fauxDepot) SupprimerDemande(id string) error {
	var d []DemandeGuilde
	for _, x := range f.demandes {
		if x.Id != id {
			d = append(d, x)
		}
	}
	f.demandes = d
	return nil
}
func (f *fauxDepot) CreerMessage(m *MessageGuilde) error {
	m.Id = f.id()
	m.Cree = fmt.Sprintf("t%06d", f.t)
	f.messages = append(f.messages, *m)
	f.dernier[m.Guilde+"|"+m.Auteur] = f.t
	return nil
}

func (f *fauxDepot) role(uid string) string {
	for _, m := range f.membres {
		if m.Joueur == uid {
			return m.Role
		}
	}
	return ""
}

func attendu(t *testing.T, r Reponse, code int, verdict string) {
	t.Helper()
	if r.Code != code {
		t.Fatalf("code %d attendu, recu %d (%v)", code, r.Code, r.Corps["verdict"])
	}
	if verdict != "" && !strings.Contains(fmt.Sprint(r.Corps["verdict"]), verdict) {
		t.Fatalf("verdict %q attendu, recu %q", verdict, r.Corps["verdict"])
	}
}

// fonder : Alice (age 2) fonde « Les Ours ».
func fonder(t *testing.T) (*fauxDepot, string) {
	t.Helper()
	f := nouveauDepot()
	r := GuildeCreer(f, "a", "  Les   Ours ", "Grr")
	attendu(t, r, 200, "")
	return f, f.guildes[0].Id
}

func TestCreerGuilde(t *testing.T) {
	f, _ := fonder(t)
	if f.guildes[0].Nom != "Les Ours" || f.role("a") != RoleChef {
		t.Fatalf("guilde %+v, role %q", f.guildes[0], f.role("a"))
	}
	attendu(t, GuildeCreer(f, "b", "Les Loups", ""), 403, "l'age 2") // Bob est a l'age 1
	attendu(t, GuildeCreer(f, "c", "les ours", ""), 400, VerdictGuildeNomPris)
	attendu(t, GuildeCreer(f, "c", "ab", ""), 400, VerdictGuildeNom)
	attendu(t, GuildeCreer(f, "a", "Autre", ""), 400, VerdictGuildeDejaMembre)
	attendu(t, GuildeCreer(f, "", "Autre", ""), 401, "")
	f.reglages.AgeMinCreation = 0
	attendu(t, GuildeCreer(f, "b", "Les Loups", ""), 200, "")
}

func TestDemandeEtAcceptation(t *testing.T) {
	f, g := fonder(t)
	attendu(t, GuildeDemander(f, "b", g), 200, "")
	attendu(t, GuildeDemander(f, "b", g), 400, VerdictGuildeDejaDemande)
	d := f.demandes[0].Id
	// Bob ne s'accepte pas lui-meme.
	attendu(t, GuildeRepondre(f, "b", d, true), 403, "")
	// Un autre joueur non plus.
	attendu(t, GuildeRepondre(f, "c", d, true), 403, "")
	attendu(t, GuildeRepondre(f, "a", d, true), 200, "")
	if f.role("b") != RoleMembre || len(f.demandes) != 0 {
		t.Fatalf("bob %q, demandes %v", f.role("b"), f.demandes)
	}
	attendu(t, GuildeDemander(f, "b", g), 400, VerdictGuildeDejaMembre)
}

func TestInvitation(t *testing.T) {
	f, g := fonder(t)
	attendu(t, GuildeInviter(f, "a", "a"), 400, VerdictGuildeSoi)
	attendu(t, GuildeInviter(f, "a", "zz"), 404, "")
	attendu(t, GuildeInviter(f, "a", "c"), 200, "")
	// Cid avait aussi demande ailleurs : rejoindre efface tout.
	f.guildes = append(f.guildes, Guilde{Id: "g2", Nom: "Autre"})
	f.demandes = append(f.demandes, DemandeGuilde{Id: "dx", Guilde: "g2", Joueur: "c", Sens: SensDemande})
	var inv string
	for _, x := range f.demandes {
		if x.Sens == SensInvitation {
			inv = x.Id
		}
	}
	// Le chef n'accepte pas l'invitation a la place de Cid.
	attendu(t, GuildeRepondre(f, "a", inv, true), 403, "")
	attendu(t, GuildeRepondre(f, "c", inv, true), 200, "")
	if f.role("c") != RoleMembre || len(f.demandes) != 0 {
		t.Fatalf("cid %q, demandes %v", f.role("c"), f.demandes)
	}
	_ = g
	// Un simple membre n'invite pas.
	attendu(t, GuildeInviter(f, "c", "d"), 403, "")
}

func TestRefusEtAnnulation(t *testing.T) {
	f, g := fonder(t)
	attendu(t, GuildeDemander(f, "b", g), 200, "")
	attendu(t, GuildeRepondre(f, "b", f.demandes[0].Id, false), 200, "") // Bob annule
	attendu(t, GuildeInviter(f, "a", "b"), 200, "")
	attendu(t, GuildeRepondre(f, "a", f.demandes[0].Id, false), 200, "") // le chef annule
	attendu(t, GuildeRepondre(f, "a", "nope", false), 404, "")
}

func TestGuildePleine(t *testing.T) {
	f, g := fonder(t) // 1 membre, max 3
	for _, j := range []string{"b", "c"} {
		attendu(t, GuildeInviter(f, "a", j), 200, "")
		attendu(t, GuildeRepondre(f, j, f.demandes[len(f.demandes)-1].Id, true), 200, "")
	}
	attendu(t, GuildeDemander(f, "d", g), 400, VerdictGuildePleine)
	attendu(t, GuildeInviter(f, "a", "d"), 400, VerdictGuildePleine)
	// Une demande deja posee ne passe pas non plus une fois la guilde pleine.
	f.demandes = append(f.demandes, DemandeGuilde{Id: "old", Guilde: g, Joueur: "e", JoueurNom: "Eve", Sens: SensDemande})
	attendu(t, GuildeRepondre(f, "a", "old", true), 400, VerdictGuildePleine)
}

func membres(t *testing.T) (*fauxDepot, string) {
	f, g := fonder(t)
	for _, j := range []string{"b", "c"} {
		attendu(t, GuildeInviter(f, "a", j), 200, "")
		attendu(t, GuildeRepondre(f, j, f.demandes[len(f.demandes)-1].Id, true), 200, "")
	}
	return f, g
}

func TestRolesEtExclusion(t *testing.T) {
	f, _ := membres(t) // a chef, b et c membres ; 1 officier max
	attendu(t, GuildeRole(f, "b", "c", RoleOfficier), 403, "")
	attendu(t, GuildeRole(f, "a", "b", RoleOfficier), 200, "")
	attendu(t, GuildeRole(f, "a", "c", RoleOfficier), 400, VerdictGuildeOfficiers)
	attendu(t, GuildeRole(f, "a", "c", "roi"), 400, VerdictGuildeRole)
	// L'officier exclut un membre, pas le chef.
	attendu(t, GuildeExclure(f, "b", "a"), 403, "")
	attendu(t, GuildeExclure(f, "b", "c"), 200, "")
	if f.role("c") != "" {
		t.Fatal("cid toujours membre")
	}
	// Le chef ne quitte pas ; l'officier si.
	attendu(t, GuildeQuitter(f, "a"), 400, VerdictGuildeChefQuitte)
	attendu(t, GuildeExclure(f, "a", "c"), 404, "")
}

func TestTransmission(t *testing.T) {
	f, g := membres(t)
	attendu(t, GuildeRole(f, "a", "b", RoleOfficier), 200, "")
	// b (officier) devient chef : son poste se libere, a devient officier.
	attendu(t, GuildeRole(f, "a", "b", RoleChef), 200, "")
	if f.role("b") != RoleChef || f.role("a") != RoleOfficier {
		t.Fatalf("b %q, a %q", f.role("b"), f.role("a"))
	}
	gg, _ := f.GuildeParId(g)
	if gg.Chef != "b" {
		t.Fatalf("chef de la guilde : %q", gg.Chef)
	}
	// b transmet a c (membre) : plus de place d'officier (a l'occupe) → b membre.
	attendu(t, GuildeRole(f, "b", "c", RoleChef), 200, "")
	if f.role("c") != RoleChef || f.role("b") != RoleMembre {
		t.Fatalf("c %q, b %q", f.role("c"), f.role("b"))
	}
	n := 0
	for _, m := range f.membres {
		if m.Role == RoleChef {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("%d chefs", n)
	}
}

func TestDissoudre(t *testing.T) {
	f, _ := membres(t)
	attendu(t, GuildeDissoudre(f, "b"), 403, "")
	attendu(t, GuildeDissoudre(f, "a"), 200, "")
	if len(f.guildes) != 0 || len(f.membres) != 0 {
		t.Fatalf("reste %v %v", f.guildes, f.membres)
	}
}

func TestSalonPriveEtAntiSpam(t *testing.T) {
	f, _ := membres(t)
	attendu(t, SalonLire(f, "d", ""), 403, "")
	attendu(t, SalonEcrire(f, "d", "coucou"), 403, "")
	attendu(t, SalonEcrire(f, "b", "  "), 400, VerdictGuildeVide)
	attendu(t, SalonEcrire(f, "b", strings.Repeat("é", MessageGuildeMax+1)), 400, VerdictGuildeLong)
	attendu(t, SalonEcrire(f, "b", "salut"), 200, "")
	f.t += 9
	r := SalonEcrire(f, "b", "encore")
	attendu(t, r, 429, "encore 1 s")
	if r.Corps["attente"] != 1 {
		t.Fatalf("attente %v", r.Corps["attente"])
	}
	// Un autre membre n'est pas freine par Bob.
	attendu(t, SalonEcrire(f, "c", "moi aussi"), 200, "")
	f.t++
	attendu(t, SalonEcrire(f, "b", "encore"), 200, "")
	r = SalonLire(f, "a", "")
	attendu(t, r, 200, "")
	if n := len(r.Corps["messages"].([]MessageGuilde)); n != 3 {
		t.Fatalf("%d messages", n)
	}
}

func TestEtat(t *testing.T) {
	f, g := membres(t)
	f.reglages.MembresMax = 10
	attendu(t, GuildeDemander(f, "d", g), 200, "")
	r := GuildesEtat(f, "a")
	attendu(t, r, 200, "")
	ma := r.Corps["ma_guilde"].(map[string]any)
	if ma["role"] != RoleChef || len(ma["demandes"].([]DemandeGuilde)) != 1 {
		t.Fatalf("ma guilde %v", ma)
	}
	ms := ma["membres"].([]Membre)
	if ms[0].Role != RoleChef {
		t.Fatalf("le chef d'abord : %v", ms)
	}
	// Un membre ne voit pas les demandes.
	r = GuildesEtat(f, "b")
	if _, ok := r.Corps["ma_guilde"].(map[string]any)["demandes"]; ok {
		t.Fatal("un membre voit les demandes")
	}
	// Dan voit sa demande, et qu'il ne peut pas fonder (age 0 < 2).
	r = GuildesEtat(f, "d")
	if r.Corps["peut_creer"] != false || len(r.Corps["mes_demandes"].([]DemandeGuilde)) != 1 {
		t.Fatalf("etat de Dan %v", r.Corps)
	}
	if l := r.Corps["guildes"].([]ResumeGuilde); len(l) != 1 || l[0].Membres != 3 || l[0].ChefNom != "Alice" {
		t.Fatalf("liste %v", l)
	}
	f.panne = true
	attendu(t, GuildesEtat(f, "a"), 500, "")
}

func TestAgeAtteint(t *testing.T) {
	ages := []AgeRequis{{Numero: 3, Batiments: []int{30}}, {Numero: 1}, {Numero: 2, Batiments: []int{20, 21}}}
	cas := []struct {
		poss map[int]bool
		age  int
	}{
		{map[int]bool{}, 1},
		{map[int]bool{20: true}, 1},
		{map[int]bool{20: true, 21: true}, 2},
		{map[int]bool{20: true, 21: true, 30: true}, 3},
		{map[int]bool{30: true}, 1}, // l'age 3 sans l'age 2 ne compte pas
	}
	for _, c := range cas {
		if got := AgeAtteint(ages, c.poss); got != c.age {
			t.Errorf("%v : age %d, attendu %d", c.poss, got, c.age)
		}
	}
	if AgeAtteint(nil, nil) != 0 {
		t.Error("sans age : 0")
	}
}

func TestReglagesNormaliser(t *testing.T) {
	r := ReglagesGuildes{MembresMax: 0, OfficiersMax: -1, DelaiSalon: -5, AgeMinCreation: -2}.Normaliser()
	if r.MembresMax != ReglagesParDefaut.MembresMax || r.OfficiersMax != ReglagesParDefaut.OfficiersMax ||
		r.DelaiSalon != ReglagesParDefaut.DelaiSalon || r.AgeMinCreation != 0 {
		t.Fatalf("%+v", r)
	}
	r = ReglagesGuildes{MembresMax: 5, OfficiersMax: 0, DelaiSalon: 0, AgeMinCreation: 4}.Normaliser()
	if r.MembresMax != 5 || r.OfficiersMax != 0 || r.DelaiSalon != 0 || r.AgeMinCreation != 4 {
		t.Fatalf("zero officier / zero delai doivent rester possibles : %+v", r)
	}
}
