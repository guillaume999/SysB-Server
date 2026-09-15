package routes

import (
	"strings"
	"testing"
)

func TestChatUnMessageParMinute(t *testing.T) {
	e := EnvoiChat{Contenu: "bonjour", DernierEnvoi: 1000, Maintenant: 1059}
	if _, a, r := JugerChat(e); r != VerdictChatAttente(1) || a != 1 {
		t.Fatalf("59 s apres : attendu le refus « encore 1 s », recu %q", r)
	}
	e.Maintenant = 1060
	if c, a, r := JugerChat(e); r != "" || a != 0 || c != "bonjour" {
		t.Fatalf("60 s apres : attendu accepte, recu %q / %q", c, r)
	}
	e.DernierEnvoi = 0
	e.Maintenant = 5
	if _, _, r := JugerChat(e); r != "" {
		t.Fatalf("premier message refuse : %q", r)
	}
}

func TestChatTexteAvantMinute(t *testing.T) {
	e := EnvoiChat{Contenu: "  \n ", DernierEnvoi: 1000, Maintenant: 1001}
	if _, a, r := JugerChat(e); r != VerdictMessageVide || a != 0 {
		t.Fatalf("attendu « vide », recu %q", r)
	}
	e.Contenu = strings.Repeat("é", LongueurMaxChat+1)
	if _, _, r := JugerChat(e); r != VerdictChatTropLong {
		t.Fatalf("attendu « trop long », recu %q", r)
	}
	e.Contenu = strings.Repeat("é", LongueurMaxChat)
	e.DernierEnvoi = 0
	if _, _, r := JugerChat(e); r != "" {
		t.Fatalf("300 caracteres accentues refuses : %q", r)
	}
}

func TestAttenteChat(t *testing.T) {
	cas := [][3]int{{0, 10, 0}, {100, 100, 60}, {100, 130, 30}, {100, 200, 0}}
	for _, c := range cas {
		if got := AttenteChat(c[0], c[1]); got != c[2] {
			t.Errorf("AttenteChat(%d,%d) = %d, attendu %d", c[0], c[1], got, c[2])
		}
	}
}

func TestSignalementChat(t *testing.T) {
	if _, r := JugerSignalementChat("a", "a", ""); r != VerdictSignalementSoiMem {
		t.Errorf("son propre message signale : %q", r)
	}
	if _, r := JugerSignalementChat("", "a", ""); r == "" {
		t.Error("signalement anonyme accepte")
	}
	if m, r := JugerSignalementChat("b", "a", " spam "); r != "" || m != "spam" {
		t.Errorf("signalement normal refuse : %q %q", m, r)
	}
}
