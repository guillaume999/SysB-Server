package routes

// ============================================================================
//  partage.go — QUI A LE DROIT D'UTILISER QUOI, SUR QUELLE PLANETE.
//
//  L'administrateur ouvre un modele 3D ou une icone a des planetes. Un joueur
//  ne peut citer, dans une tuile, que ce qui est ouvert A SA PLANETE.
//
//  ⚠️⚠️ CE FICHIER EST LE REFUS, PAS LE FILTRE. Le site cache deja ce qui n'est
//  pas ouvert dans ses listes deroulantes — mais un filtre n'est pas une regle
//  (26/08). Tant que seul l'ecran refusait, une requete a la main posait
//  n'importe quoi.
//
//  ⚠️ POURQUOI PAS UNE REGLE D'API POCKETBASE : il faudrait comparer DEUX champs
//  du corps l'un a l'autre — `@request.body.modele.planetes_autorisees` contre
//  `@request.body.planete`. Les regles savent traverser une relation du corps,
//  pas arbitrer deux relations du corps entre elles. D'ou un crochet.
// ============================================================================

// Partageable : ce qu'un modele 3D ou une icone dit de son partage.
type Partageable struct {
	// Ouvert a TOUTES les planetes, y compris celles creees demain.
	ToutesPlanetes bool
	// Les planetes ouvertes une par une.
	PlanetesAutorisees []string
}

// AutoriseeSur — LA REGLE, et elle vit ici seulement.
//
//	autorise = ToutesPlanetes
//	        || la planete est dans PlanetesAutorisees
//	        || la planete n'a PAS de proprietaire (planete game)
//
// ⚠️⚠️ UNE LISTE VIDE VEUT DIRE « A PERSONNE », PAS « A TOUT LE MONDE ». C'est
// pour ca qu'il faut les DEUX champs : avec « vide = toutes », on perdrait la
// seule facon de dire « rien pour l'instant », et une entree oubliee s'ouvrirait
// a tous en silence.
//
// ⚠️ La troisieme branche est ce qui rend le patch indolore : les 28 prefabs et
// les icones en service restent utilisables sur la Terre et Jupiter SANS qu'on
// ait rien coche. Une planete de joueur, elle, n'a rien tant que l'admin n'a pas
// ouvert.
func AutoriseeSur(p Partageable, planeteId string, planeteGame bool) bool {
	if planeteId == "" {
		// ⚠️ PAS DE PLANETE, PAS D'AUTORISATION. Laisser passer ici reviendrait a
		// ouvrir tout a une tuile mal rattachee — exactement le trou qu'on ferme.
		return false
	}
	if p.ToutesPlanetes {
		return true
	}
	for _, id := range p.PlanetesAutorisees {
		if id == planeteId {
			return true
		}
	}
	return planeteGame
}

// RefusDeTuile — la phrase a rendre quand une tuile cite ce qu'elle n'a pas le
// droit de citer, ou la chaine vide quand tout va bien.
//
// ⚠️ ELLE NOMME LAQUELLE DES DEUX LISTES A REFUSE. « non autorise » tout court
// enverrait chercher dans la mauvaise : une tuile a besoin d'un modele 3D ET
// d'une icone, et il n'y a aucune raison que les deux soient ouvertes ensemble.
func RefusDeTuile(modele, icone Partageable, aUnModele, uneIcone bool,
	planeteId, nomPlanete string, planeteGame bool) string {
	if planeteId == "" {
		return "Cette tuile n'est rattachee a aucune planete : impossible de dire ce qu'elle a " +
			"le droit d'utiliser. Choisis d'abord son modele de plateau."
	}
	if aUnModele && !AutoriseeSur(modele, planeteId, planeteGame) {
		return "Ce modele 3D n'est pas ouvert a la planete « " + nomPlanete + " ». " +
			"C'est l'administrateur qui l'ouvre, dans l'onglet Planetes."
	}
	if uneIcone && !AutoriseeSur(icone, planeteId, planeteGame) {
		return "Cette icone n'est pas ouverte a la planete « " + nomPlanete + " ». " +
			"C'est l'administrateur qui l'ouvre, dans l'onglet Planetes."
	}
	return ""
}
