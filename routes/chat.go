package routes

// ============================================================================
//  chat.go — LE CHAT GENERAL DE L'ACCUEIL (15/09).
//
//  Une collection, `chat_general` (auteur, auteur_nom, contenu), lue par tout
//  compte connecte. Le jeu l'INTERROGE a intervalle regulier : il n'y a pas de
//  temps reel, et c'est voulu (pas de connexion a tenir ouverte sur mobile).
//
//  ⚠️⚠️ L'ANTI-SPAM EST ICI, PAS DANS LE JEU : UN MESSAGE PAR MINUTE, par
//  compte. Le compte a rebours affiche par Unity n'est qu'un confort — un client
//  modifie enverrait quand meme, et c'est ce fichier qui dit non.
//
//  ⚠️ Le compte a rebours se calcule sur le DERNIER message ACCEPTE du compte,
//  pas sur la derniere tentative : un envoi refuse ne relance pas la minute.
// ============================================================================

import "fmt"

const (
	// LongueurMaxChat : plus court qu'un message prive — c'est une salle commune.
	LongueurMaxChat = 300
	// DelaiChat : un message par compte toutes les N secondes.
	DelaiChat = 60
	// DureeVieChat : la purge quotidienne efface ce qui est plus vieux.
	DureeVieChat = 7 * 24 * 3600
)

const (
	VerdictChatTropLong      = "Le message est trop long (300 caracteres au plus)."
	VerdictSignalementSoiMem = "On ne signale pas son propre message."
)

// EnvoiChat : ce qu'il faut pour juger un message du chat.
type EnvoiChat struct {
	Contenu string
	// Instant (unix, s) du dernier message ACCEPTE de ce compte ; 0 = aucun.
	DernierEnvoi int
	Maintenant   int
}

// AttenteChat : les secondes a attendre avant le prochain message (0 = libre).
func AttenteChat(dernier, maintenant int) int {
	if dernier <= 0 {
		return 0
	}
	reste := dernier + DelaiChat - maintenant
	if reste < 0 {
		return 0
	}
	return reste
}

// VerdictChatAttente : la phrase du refus, avec les secondes restantes.
func VerdictChatAttente(secondes int) string {
	return fmt.Sprintf("Un message par minute : encore %d s.", secondes)
}

// JugerChat : le contenu a enregistrer, ou le refus. `attente` > 0 dit que le
// refus est la minute (le serveur repond alors 429, et le jeu lance son compte
// a rebours) ; 0 pour un refus de texte.
//
// ⚠️ Le texte d'abord, la minute ensuite : un message vide ne doit pas
// apprendre au joueur qu'il doit attendre.
func JugerChat(e EnvoiChat) (contenu string, attente int, refus string) {
	contenu = NettoyerTexte(e.Contenu)
	switch {
	case contenu == "":
		return "", 0, VerdictMessageVide
	case len([]rune(contenu)) > LongueurMaxChat:
		return "", 0, VerdictChatTropLong
	}
	if s := AttenteChat(e.DernierEnvoi, e.Maintenant); s > 0 {
		return "", s, VerdictChatAttente(s)
	}
	return contenu, 0, ""
}

// JugerSignalementChat : n'importe quel compte signale un message du chat —
// sauf le sien. Le motif suit la meme borne que pour un message prive.
func JugerSignalementChat(signaleur, auteur, motif string) (string, string) {
	motif = NettoyerTexte(motif)
	if signaleur == "" {
		return "", VerdictSignalementDroit
	}
	if signaleur == auteur {
		return "", VerdictSignalementSoiMem
	}
	if len([]rune(motif)) > LongueurMaxMotif {
		return "", VerdictMotifTropLong
	}
	return motif, ""
}
