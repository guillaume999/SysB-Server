package routes

// ============================================================================
//  messages.go — LES MESSAGES PRIVES ENTRE JOUEURS (15/09).
//
//  Trois collections (patch `patch-messages-prives-2026-09-15.js`) :
//    · messages_prives  expediteur, destinataire, les deux pseudos, contenu, lu
//    · blocages         bloqueur, bloque
//    · signalements     signaleur, message, contenu recopie, auteur, motif
//
//  ⚠️⚠️ CE FICHIER EST LE REFUS. Les regles d'API disent QUI a le droit de lire
//  et d'ecrire ; ce qu'elles ne savent pas dire vit ici, et `pb/crochet_messages.go`
//  ne fait que lire la base et poser la question :
//    · « l'un des deux a-t-il bloque l'autre ? » — il faudrait qu'une regle
//      prouve l'ABSENCE d'une ligne dans une autre collection ;
//    · « ecrit-il trop vite ? » — une regle ne compte pas ;
//    · les deux pseudos — un joueur ne peut pas lire `users`, donc ni ecrire
//      le pseudo de l'autre sans mentir, ni le lire ensuite. C'est le serveur
//      qui les recopie, pas le client.
//
//  ⚠️⚠️ ON N'ECRIT QU'A SES AMIS (15/09, `amis.go`). Une conversation avec un
//  ancien ami reste LISIBLE ; on ne peut simplement plus y ecrire.
//
//  ⚠️ PAS DE LECTURE ADMIN DES CONVERSATIONS. Un message prive se lit a deux.
//  La moderation passe par le SIGNALEMENT, qui recopie le message signale :
//  l'admin lit ce qu'on lui a montre, et rien d'autre.
// ============================================================================

import (
	"sort"
	"strings"
	"unicode"
)

const (
	// LongueurMaxMessage : en caracteres (runes), pas en octets — un accent
	// ne doit pas couter double. Meme borne que le champ du patch.
	LongueurMaxMessage = 1000
	// DelaiMinMessages : deux messages du meme expediteur, au moins N s d'ecart.
	DelaiMinMessages = 2
	// FenetreMessages / MaxMessagesFenetre : au plus N messages par fenetre.
	// ⚠️ Les deux bornes, pas une : la premiere arrete le doigt qui tremble,
	// la seconde le script qui envoie un message toutes les 2,1 s.
	FenetreMessages    = 600
	MaxMessagesFenetre = 30
	// LongueurMaxMotif : le motif d'un signalement.
	LongueurMaxMotif = 500
	// Recherche de joueurs : bornes du texte tape, et du nombre de reponses.
	RechercheMin    = 2
	RechercheMax    = 30
	RechercheLimite = 20
)

// Les phrases de refus, les memes partout (Unity les affiche telles quelles).
const (
	VerdictMessageVide      = "Le message est vide."
	VerdictMessageTropLong  = "Le message est trop long (1000 caracteres au plus)."
	VerdictMessageSoiMeme   = "On ne s'ecrit pas a soi-meme."
	VerdictDestinataire     = "Ce joueur n'existe pas."
	VerdictBloque           = "Ce joueur ne peut pas recevoir tes messages."
	VerdictTropVite         = "Doucement : attends un instant avant le message suivant."
	VerdictTropDeMessages   = "Trop de messages en peu de temps : reessaie dans quelques minutes."
	VerdictSignalementDroit = "Tu ne peux signaler qu'un message qu'on t'a envoye."
	VerdictMotifTropLong    = "Le motif est trop long (500 caracteres au plus)."
	VerdictBlocageSoiMeme   = "On ne se bloque pas soi-meme."
	VerdictModifMessage     = "Un message envoye ne se modifie pas : seul le destinataire le marque lu."
	VerdictPasAmis          = "On n'ecrit qu'a ses amis : envoie d'abord une demande d'ami."
)

// EnvoiMessage : tout ce qu'il faut savoir pour juger un envoi.
type EnvoiMessage struct {
	Expediteur         string
	Destinataire       string
	Contenu            string
	DestinataireExiste bool
	// Vrai si l'UN DES DEUX a bloque l'autre, dans un sens ou dans l'autre.
	Bloque bool
	// Vrai si les deux sont AMIS (amitie acceptee) — voir `amis.go`.
	Amis bool
	// Les instants (unix, s) des envois recents de l'expediteur, dans la
	// fenetre — l'ordre n'importe pas.
	EnvoisRecents []int
	Maintenant    int
}

// NettoyerTexte : espaces de bord retires, caracteres de controle supprimes
// (sauf le retour a la ligne), retours \r normalises, et jamais plus de deux
// lignes vides d'affilee — un message de 900 retours a la ligne n'est pas un
// message.
func NettoyerTexte(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	var b strings.Builder
	sautes := 0
	for _, r := range s {
		if r == '\n' {
			sautes++
			if sautes > 2 {
				continue
			}
			b.WriteRune(r)
			continue
		}
		if r == '\t' {
			r = ' '
		}
		if unicode.IsControl(r) {
			continue
		}
		sautes = 0
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

// JugerMessage : le contenu a enregistrer, ou la raison du refus.
//
// ⚠️ L'ORDRE DES REFUS EST VOULU : ce qui ne depend que du texte d'abord (le
// joueur corrige sans attendre), puis le blocage, puis l'amitie, et le debit en
// dernier — un joueur bloque n'a pas a apprendre qu'il ecrit trop vite, ni
// qu'il n'est « pas ami » (ce qui lui ferait tenter une demande).
// ⚠️ Le blocage ne dit PAS qui a bloque qui : « ne peut pas recevoir » vaut
// dans les deux sens, et ne renseigne pas l'autre.
func JugerMessage(e EnvoiMessage) (contenu string, refus string) {
	contenu = NettoyerTexte(e.Contenu)
	switch {
	case contenu == "":
		return "", VerdictMessageVide
	case len([]rune(contenu)) > LongueurMaxMessage:
		return "", VerdictMessageTropLong
	case e.Destinataire == "" || !e.DestinataireExiste:
		return "", VerdictDestinataire
	case e.Destinataire == e.Expediteur:
		return "", VerdictMessageSoiMeme
	case e.Bloque:
		return "", VerdictBloque
	case !e.Amis:
		return "", VerdictPasAmis
	}
	dansFenetre := 0
	for _, t := range e.EnvoisRecents {
		if e.Maintenant-t < DelaiMinMessages {
			return "", VerdictTropVite
		}
		if e.Maintenant-t < FenetreMessages {
			dansFenetre++
		}
	}
	if dansFenetre >= MaxMessagesFenetre {
		return "", VerdictTropDeMessages
	}
	return contenu, ""
}

// ModifAutorisee : une mise a jour de message ne touche QUE `lu`, et seulement
// par le destinataire. Les regles d'API le disent deja ; ceci le redit cote
// serveur, exprès — une regle retouchee a la main ne doit pas suffire a ouvrir
// la reecriture d'un message deja lu par l'autre.
func ModifAutorisee(auteurRequete, destinataire string, champsChanges []string) string {
	if auteurRequete != destinataire {
		return VerdictModifMessage
	}
	for _, c := range champsChanges {
		if c != "lu" && c != "updated" {
			return VerdictModifMessage
		}
	}
	return ""
}

// JugerSignalement : seul le DESTINATAIRE d'un message le signale — c'est le
// seul, avec l'auteur, a l'avoir lu, et l'auteur n'a aucune raison de le faire.
func JugerSignalement(signaleur, destinataireDuMessage, motif string) (string, string) {
	motif = NettoyerTexte(motif)
	if signaleur == "" || signaleur != destinataireDuMessage {
		return "", VerdictSignalementDroit
	}
	if len([]rune(motif)) > LongueurMaxMotif {
		return "", VerdictMotifTropLong
	}
	return motif, ""
}

// JugerBlocage : on ne se bloque pas soi-meme, et on bloque quelqu'un qui existe.
func JugerBlocage(bloqueur, bloque string, bloqueExiste bool) string {
	if bloque == "" || !bloqueExiste {
		return VerdictDestinataire
	}
	if bloque == bloqueur {
		return VerdictBlocageSoiMeme
	}
	return ""
}

// NormaliserRecherche : le texte tape pour chercher un joueur par son pseudo.
// Faux quand il est trop court — on ne liste pas tous les comptes du jeu.
//
// ⚠️ `%` est retire : c'est le joker de LIKE, et « % » tout seul listerait
// tout le monde malgre la borne de longueur.
func NormaliserRecherche(q string) (string, bool) {
	q = strings.TrimSpace(strings.ReplaceAll(q, "%", ""))
	r := []rune(q)
	if len(r) < RechercheMin {
		return "", false
	}
	if len(r) > RechercheMax {
		r = r[:RechercheMax]
	}
	return string(r), true
}

// Joueur : ce que la recherche rend d'un compte — l'id et le pseudo, rien d'autre.
type Joueur struct {
	Id     string `json:"id"`
	Pseudo string `json:"pseudo"`
}

// RangerJoueurs : soi-meme et les comptes sans pseudo retires ; ceux dont le
// pseudo COMMENCE par la recherche d'abord, puis l'ordre alphabetique ; au plus
// `RechercheLimite`.
func RangerJoueurs(liste []Joueur, moi, q string) []Joueur {
	ql := strings.ToLower(q)
	out := make([]Joueur, 0, len(liste))
	for _, j := range liste {
		if j.Id == "" || j.Id == moi || strings.TrimSpace(j.Pseudo) == "" {
			continue
		}
		out = append(out, j)
	}
	sort.SliceStable(out, func(a, b int) bool {
		pa := strings.HasPrefix(strings.ToLower(out[a].Pseudo), ql)
		pb := strings.HasPrefix(strings.ToLower(out[b].Pseudo), ql)
		if pa != pb {
			return pa
		}
		return strings.ToLower(out[a].Pseudo) < strings.ToLower(out[b].Pseudo)
	})
	if len(out) > RechercheLimite {
		out = out[:RechercheLimite]
	}
	return out
}
