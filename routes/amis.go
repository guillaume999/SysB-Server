package routes

// ============================================================================
//  amis.go — LES AMIS (15/09).
//
//  Une collection, `amities` : demandeur, destinataire, les deux pseudos,
//  statut (`attente` | `acceptee`). Une ligne par COUPLE, quel que soit le sens.
//
//    · A demande        → ligne `attente` (A demandeur, B destinataire) ;
//    · B accepte        → la MEME ligne passe `acceptee` (seul B le peut) ;
//    · B refuse / A annule / l'un retire l'autre → la ligne est SUPPRIMEE ;
//    · l'un bloque l'autre → la ligne est supprimee par le serveur.
//
//  ⚠️⚠️ PAS PLUS DE `MaxAmis` AMIS. Compte a DEUX moments :
//    · a la DEMANDE : amis du demandeur + ses demandes envoyees en attente —
//      sinon on enverrait 200 demandes et la 31e acceptation deborderait ;
//    · a l'ACCEPTATION : les amis de CHACUN des deux, strictement sous la borne.
//  Les demandes RECUES ne comptent pas : un inconnu ne doit pas pouvoir
//  remplir ma liste a ma place.
//
//  ⚠️ Les messages prives exigent une amitie `acceptee` (`messages.go`).
// ============================================================================

const (
	MaxAmis           = 30
	StatutAttente     = "attente"
	StatutAcceptee    = "acceptee"
	VerdictAmiSoi     = "On ne devient pas ami avec soi-meme."
	VerdictAmiDeja    = "Vous etes deja amis, ou une demande est deja en cours."
	VerdictAmisPlein  = "Tu as deja 30 amis (demandes en attente comprises) : retires-en un d'abord."
	VerdictAutrePlein = "Ce joueur a deja 30 amis."
	VerdictAmiDroit   = "Seul le joueur qui a recu la demande peut l'accepter."
	VerdictAmiModif   = "Une amitie ne se modifie pas : on l'accepte, ou on la supprime."
)

// DemandeAmi : ce qu'il faut pour juger une demande d'ami.
type DemandeAmi struct {
	Demandeur          string
	Destinataire       string
	DestinataireExiste bool
	Bloque             bool // l'un des deux a bloque l'autre
	DejaLies           bool // une ligne existe deja entre eux, dans un sens ou l'autre
	// Amis acceptes du demandeur + demandes qu'il a envoyees, en attente.
	EngagesDemandeur int
}

// JugerDemandeAmi : le refus, ou "".
//
// ⚠️ Le blocage ne se dit pas : « ne peut pas recevoir » vaut dans les deux
// sens, comme pour les messages.
func JugerDemandeAmi(d DemandeAmi) string {
	switch {
	case d.Destinataire == "" || !d.DestinataireExiste:
		return VerdictDestinataire
	case d.Destinataire == d.Demandeur:
		return VerdictAmiSoi
	case d.Bloque:
		return VerdictBloque
	case d.DejaLies:
		return VerdictAmiDeja
	case d.EngagesDemandeur >= MaxAmis:
		return VerdictAmisPlein
	}
	return ""
}

// Acceptation : une mise a jour de `amities`.
type Acceptation struct {
	Auteur       string // qui fait la requete
	Destinataire string // le destinataire de la demande (valeur AVANT)
	StatutAvant  string
	StatutApres  string
	// Les champs modifies autres que `statut` (et `updated`).
	AutresChamps []string
	AmisAuteur   int // amities acceptees de l'auteur
	AmisAutre    int // amities acceptees du demandeur
}

// JugerAcceptation : la seule modification permise est `attente` → `acceptee`,
// par le destinataire, si aucun des deux n'est deja a `MaxAmis`.
func JugerAcceptation(a Acceptation) string {
	if a.Auteur == "" || a.Auteur != a.Destinataire {
		return VerdictAmiDroit
	}
	for _, c := range a.AutresChamps {
		if c != "updated" {
			return VerdictAmiModif
		}
	}
	if a.StatutAvant != StatutAttente || a.StatutApres != StatutAcceptee {
		return VerdictAmiModif
	}
	if a.AmisAuteur >= MaxAmis {
		return VerdictAmisPlein
	}
	if a.AmisAutre >= MaxAmis {
		return VerdictAutrePlein
	}
	return ""
}
