package routes

import (
	"strings"
	"testing"
)

func envoi() EnvoiMessage {
	return EnvoiMessage{Expediteur: "a", Destinataire: "b", Contenu: "  salut  ",
		DestinataireExiste: true, Amis: true, Maintenant: 10_000}
}

func TestMessageAccepteEtNettoye(t *testing.T) {
	c, refus := JugerMessage(envoi())
	if refus != "" || c != "salut" {
		t.Fatalf("attendu « salut » accepte, recu %q / %q", c, refus)
	}
}

func TestMessageRefus(t *testing.T) {
	cas := map[string]func(*EnvoiMessage){
		VerdictMessageVide:     func(e *EnvoiMessage) { e.Contenu = " \n\t\x00 " },
		VerdictMessageTropLong: func(e *EnvoiMessage) { e.Contenu = strings.Repeat("é", LongueurMaxMessage+1) },
		VerdictDestinataire:    func(e *EnvoiMessage) { e.DestinataireExiste = false },
		VerdictMessageSoiMeme:  func(e *EnvoiMessage) { e.Destinataire = "a" },
		VerdictBloque:          func(e *EnvoiMessage) { e.Bloque = true },
		VerdictPasAmis:         func(e *EnvoiMessage) { e.Amis = false },
		VerdictTropVite:        func(e *EnvoiMessage) { e.EnvoisRecents = []int{9_999} },
	}
	for attendu, modif := range cas {
		e := envoi()
		modif(&e)
		if _, refus := JugerMessage(e); refus != attendu {
			t.Errorf("attendu %q, recu %q", attendu, refus)
		}
	}
}

// La borne est en caracteres, pas en octets : 1000 « é » passent.
func TestMessageLongueurEnRunes(t *testing.T) {
	e := envoi()
	e.Contenu = strings.Repeat("é", LongueurMaxMessage)
	if _, refus := JugerMessage(e); refus != "" {
		t.Fatalf("1000 caracteres accentues refuses : %q", refus)
	}
}

func TestMessageDebitFenetre(t *testing.T) {
	e := envoi()
	for i := 0; i < MaxMessagesFenetre-1; i++ {
		e.EnvoisRecents = append(e.EnvoisRecents, e.Maintenant-10-i)
	}
	if _, refus := JugerMessage(e); refus != "" {
		t.Fatalf("%d messages dans la fenetre devraient passer : %q", MaxMessagesFenetre-1, refus)
	}
	e.EnvoisRecents = append(e.EnvoisRecents, e.Maintenant-100)
	if _, refus := JugerMessage(e); refus != VerdictTropDeMessages {
		t.Fatalf("attendu le refus de debit, recu %q", refus)
	}
	// Hors fenetre, ils ne comptent plus.
	for i := range e.EnvoisRecents {
		e.EnvoisRecents[i] = e.Maintenant - FenetreMessages
	}
	if _, refus := JugerMessage(e); refus != "" {
		t.Fatalf("des envois hors fenetre bloquent encore : %q", refus)
	}
}

// Un joueur bloque ne doit pas apprendre qu'il ecrit trop vite.
func TestBlocageAvantDebit(t *testing.T) {
	e := envoi()
	e.Bloque = true
	e.EnvoisRecents = []int{e.Maintenant}
	if _, refus := JugerMessage(e); refus != VerdictBloque {
		t.Fatalf("attendu le blocage, recu %q", refus)
	}
}

func TestNettoyerTexte(t *testing.T) {
	got := NettoyerTexte("a\r\n\n\n\n\nb\tc\x07")
	if got != "a\n\nb c" {
		t.Fatalf("recu %q", got)
	}
}

func TestModifAutorisee(t *testing.T) {
	if ModifAutorisee("b", "b", []string{"lu"}) != "" {
		t.Error("le destinataire doit pouvoir marquer lu")
	}
	if ModifAutorisee("a", "b", []string{"lu"}) == "" {
		t.Error("l'expediteur ne marque pas lu a la place de l'autre")
	}
	if ModifAutorisee("b", "b", []string{"lu", "contenu"}) == "" {
		t.Error("le contenu ne se reecrit pas")
	}
}

func TestSignalementEtBlocage(t *testing.T) {
	if _, r := JugerSignalement("a", "b", ""); r != VerdictSignalementDroit {
		t.Errorf("l'auteur ne signale pas son propre message : %q", r)
	}
	if m, r := JugerSignalement("b", "b", "  insultes "); r != "" || m != "insultes" {
		t.Errorf("signalement du destinataire refuse : %q %q", m, r)
	}
	if _, r := JugerSignalement("b", "b", strings.Repeat("x", LongueurMaxMotif+1)); r != VerdictMotifTropLong {
		t.Errorf("motif trop long accepte : %q", r)
	}
	if JugerBlocage("a", "a", true) != VerdictBlocageSoiMeme {
		t.Error("se bloquer soi-meme accepte")
	}
	if JugerBlocage("a", "b", false) != VerdictDestinataire {
		t.Error("bloquer un compte inexistant accepte")
	}
	if JugerBlocage("a", "b", true) != "" {
		t.Error("blocage normal refuse")
	}
}

func TestRecherche(t *testing.T) {
	if _, ok := NormaliserRecherche(" %% a "); ok {
		t.Error("une recherche d'une lettre (jokers retires) doit etre refusee")
	}
	if q, ok := NormaliserRecherche("  Sam "); !ok || q != "Sam" {
		t.Errorf("recu %q %v", q, ok)
	}
	liste := []Joueur{{"1", "Zsam"}, {"2", "samp"}, {"moi", "Sammy"}, {"3", ""}, {"4", "Samba"}}
	got := RangerJoueurs(liste, "moi", "sam")
	var noms []string
	for _, j := range got {
		noms = append(noms, j.Pseudo)
	}
	if strings.Join(noms, ",") != "Samba,samp,Zsam" {
		t.Fatalf("ordre recu %v", noms)
	}
}

// Bloque ET pas amis : c'est le blocage qui parle.
func TestBlocageAvantAmitie(t *testing.T) {
	e := envoi()
	e.Bloque, e.Amis = true, false
	if _, refus := JugerMessage(e); refus != VerdictBloque {
		t.Fatalf("attendu le blocage, recu %q", refus)
	}
}
