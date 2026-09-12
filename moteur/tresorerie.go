// ============================================================
//  moteur/tresorerie.go — CE QUE LE JOUEUR PEUT DEPENSER
//
//    disponible   ce qui se PAIE : le contenu des coffres des batiments qui
//                 ENVOIENT (l'entrepot se DEDUIT, il ne se declare pas), plus
//                 les places logees (la population), plus la reserve du plateau
//                 pour les `FluxStock`.
//    en attente   le contenu des coffres des batiments qui ne font que
//                 recolter : ils gardent pour eux, ca ne se depense pas.
//    mobilise     ce que les couts `mobilise` des batiments en marche
//                 immobilisent. Jamais preleve : c'est lu.
//    libre        disponible - mobilise. C'est a LUI que se compare un cout
//                 `mobilise`, jamais au disponible.
//
//  ⚠️ PRODUIRE N'EST PAS DEPENSER. Une ferme pleine dont personne ne vient
//  chercher le ble n'enrichit pas le joueur : c'est cette couche qui explique
//  « ma monnaie n'a pas augmente » quand la logistique ne suit pas.
//
//  ⚠️ LECTURE SEULE sur les coffres, a une exception pres : `Debiter`, qui ne
//  touche a rien lui-meme et passe par `Retirer` — le seul passage (§6bis :
//  « il n'y a pas de chemin parallele »).
// ============================================================

package moteur

import (
	"fmt"
	"sort"
)

// Places : la population se LIT sur les places declarees des batiments vivants
// (26/08). Une maison en chantier ou en veille ne loge personne.
func Places(p *Plateau, t int) Sac {
	total := NouveauSac(p.Genres.Reg)
	for _, b := range p.Batiments {
		if !b.Vivant(t) {
			continue
		}
		for _, c := range b.Tuile.StockageCodes() {
			if p.Genres.EstMobilise(c) {
				total.Ajouter(c, b.Tuile.MaxStocke(c))
			}
		}
	}
	return total
}

// additionner — ⚠️ SEULS LES BATIMENTS VIVANTS COMPTENT, des deux cotes. Un
// entrepot en chantier ne sert personne ; un entrepot en veille non plus. Le
// meme predicat que `Lire` sur une bourse commune — sinon le magasin dirait
// « tu as de quoi » et le prelevement echouerait.
func additionner(p *Plateau, t int, versLeDepensable bool) Sac {
	total := NouveauSac(p.Genres.Reg)
	for _, b := range p.Batiments {
		if !b.Vivant(t) || b.Tuile.Envoie() != versLeDepensable {
			continue
		}
		for _, c := range b.Stock.NonNuls() {
			if p.Genres.EnCoffre(c) {
				total.Ajouter(c, b.Stock.Get(c))
			}
		}
	}
	if versLeDepensable {
		places := Places(p, t)
		for _, c := range places.NonNuls() {
			total.Ajouter(c, places.Get(c))
		}
		for _, c := range p.Reserve.NonNuls() {
			if p.Genres.EstFluxStock(c) {
				total.Ajouter(c, p.Reserve.Get(c))
			}
		}
	}
	return total
}

func Disponible(p *Plateau, t int) Sac { return additionner(p, t, true) }
func EnAttente(p *Plateau, t int) Sac  { return additionner(p, t, false) }

// Mobilise : ce que les couts `mobilise` des batiments vivants immobilisent.
func Mobilise(p *Plateau, t int) Sac {
	total := NouveauSac(p.Genres.Reg)
	for _, b := range p.Batiments {
		if !b.Vivant(t) {
			continue
		}
		for _, c := range b.Tuile.Cout {
			if c.Mode == "mobilise" && c.Quantite > 0 {
				total.Ajouter(c.Ressource, c.Quantite)
			}
		}
	}
	return total
}

// Capacite : combien les entrepots (les tuiles qui ENVOIENT et qui STOCKENT)
// peuvent tenir au total, par ressource.
//
// ⚠️ `*` est le fourre-tout : il vaut pour toute ressource QUI VIT DANS UN
// COFFRE. L'appliquer a tout le catalogue afficherait « Satisfaction 0/500 »
// dans la bande du haut — la faute du 09/09, et celle du 07/09 avec la monnaie.
func Capacite(p *Plateau, t int, codesConnus []Code) Sac {
	total := NouveauSac(p.Genres.Reg)
	tous := codesConnus
	if len(tous) == 0 {
		tous = p.Genres.Codes()
	}
	for _, b := range p.Batiments {
		if !b.Vivant(t) || !b.Tuile.Envoie() || !b.Tuile.Stocke() {
			continue
		}
		for _, c := range b.Tuile.StockageCodes() {
			max := b.Tuile.MaxStocke(c)
			if max > 0 && p.Genres.EnCoffre(c) {
				total.Ajouter(c, max)
			}
		}
		partage := b.Tuile.Etoile()
		if partage <= 0 {
			continue
		}
		for _, c := range tous {
			if p.Genres.EnCoffre(c) && b.Tuile.MaxStocke(c) == partage {
				total.Ajouter(c, partage)
			}
		}
	}
	return total
}

func Libre(dispo, mob Sac, c Code) int { return dispo.Get(c) - mob.Get(c) }

// Debiter : PRELEVER `montants` — TOUT OU RIEN : on verifie tout avant de
// prelever quoi que ce soit, un paiement a moitie fait laisserait une colonie
// amputee sans batiment en face.
//
// Un `FluxStock` sort de la reserve ; le reste, des entrepots, LE PLUS PLEIN
// D'ABORD, a egalite dans l'ordre `(z, x)`.
func Debiter(p *Plateau, montants Sac, t int) (bool, error) {
	dispo := Disponible(p, t)
	codes := montants.NonNuls()
	for _, c := range codes {
		if p.Genres.EstMobilise(c) {
			return false, nil // un habitant ne se paie pas
		}
		if montants.Get(c) > dispo.Get(c) {
			return false, nil
		}
	}
	var entrepots []*Batiment
	for _, b := range p.Ordonnes() {
		if b.Vivant(t) && b.Tuile.Envoie() {
			entrepots = append(entrepots, b)
		}
	}
	for _, c := range codes {
		reste := montants.Get(c)
		if reste <= 0 {
			continue
		}
		if p.Genres.EstFluxStock(c) {
			reste -= Retirer(p, nil, c, reste, t) // la reserve : aucune case
		} else {
			ordre := append([]*Batiment(nil), entrepots...)
			sort.SliceStable(ordre, func(i, j int) bool {
				return ordre[i].Stock.Get(c) > ordre[j].Stock.Get(c)
			})
			for _, b := range ordre {
				if reste <= 0 {
					break
				}
				reste -= Retirer(p, b, c, reste, t)
			}
		}
		if reste > 0 {
			// Impossible si `Disponible` et `Retirer` lisent la meme chose.
			// Si ca arrive un jour, c'est qu'ils ont diverge : on le dit.
			return false, fmt.Errorf("[tresorerie] %s : %d n'a pas pu etre preleve "+
				"alors que le disponible le promettait", p.Genres.Reg.Nom(c), reste)
		}
	}
	return true, nil
}

// Crediter (une dotation de depart, un remboursement). Rend CE QUI N'A PAS
// TROUVE DE PLACE, par ressource — on ne l'avale pas en silence.
//
// ⚠️ DANS LES ENTREPOTS, PAS N'IMPORTE OU. Verser dans le coffre d'une ferme
// mettrait la dotation hors de portee du joueur : `Disponible` ne compte que ce
// qui peut SORTIR.
func Crediter(p *Plateau, montants Sac, t int) Sac {
	perdu := NouveauSac(p.Genres.Reg)
	for _, c := range montants.NonNuls() {
		reste := montants.Get(c)
		if reste <= 0 {
			continue
		}
		if p.Genres.EstFluxStock(c) {
			Ranger(p, nil, c, reste, t)
			continue
		}
		for _, b := range p.Ordonnes() {
			if reste <= 0 {
				break
			}
			if !b.Vivant(t) || !b.Tuile.Envoie() {
				continue
			}
			reste -= Ranger(p, b, c, reste, t)
		}
		if reste > 0 {
			perdu.Set(c, reste)
		}
	}
	return perdu
}
