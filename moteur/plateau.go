// ============================================================
//  moteur/plateau.go — un record `plateaux` <-> les cases du moteur
//
//  Miroir de `PlateauData.cs` et de `PlateauPocketBaseService.DepuisRecord`.
//
//  DEUX ENCODAGES, UNE SEULE COUCHE DE JEU. Une case porte UNE tuile, point.
//  Mais deux encodages cohabitent parce qu'ils ne portent pas la meme chose :
//
//   · `tilesBase64` — un octet par case, index `z * largeur + x`. Il dit CE
//     QU'IL Y A. 10 000 cases tiennent en 13 Ko.
//   · `etats` — un tableau json, une entree pour les SEULES cases ayant
//     quelque chose a retenir. Il dit OU EN EST chaque batiment.
//
//  Tout mettre en json couterait ~700 Ko sur un grand plateau, dont 95 % de
//  cases d'herbe repetant le meme objet vide ; tout mettre en octets est
//  impossible, un octet ne porte ni un stock ni un horodatage.
//
//  ⚠️ TOLERANT PAR CONSTRUCTION. Un plateau qui refuse de se charger est pire
//  qu'un plateau qui repart a vide sur ses etats : un champ absent, vide ou
//  illisible rend une liste vide, jamais une panne.
//
//  ⚠️ UNE CASE DONT LA TUILE EST REFUSEE PAR LE CATALOGUE N'EST PAS EFFACEE :
//  elle est mise DE COTE (`Figes`) et reecrite telle quelle. Une faute de
//  saisie sur le site ne doit pas vider les cases des joueurs au premier
//  rafraichissement.
//
//  ⚠️ PAS DE `atob` NI DE `encoding/base64` AVEC PADDING STRICT : le decodeur
//  ci-dessous accepte ce que PocketBase et Unity ecrivent, y compris sans
//  padding, et rend une liste vide plutot qu'une erreur sur un caractere
//  impossible. C'est le meme parti pris que les vingt lignes du JS.
// ============================================================

package moteur

import "strings"

const alphabet64 = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

// Base64VersOctets : rend une liste vide si l'entree est illisible.
func Base64VersOctets(texte string) []int {
	if texte == "" {
		return nil
	}
	var octets []int
	tampon, bits := 0, 0
	for i := 0; i < len(texte); i++ {
		c := texte[i]
		if c == '=' {
			break
		}
		v := strings.IndexByte(alphabet64, c)
		if v < 0 {
			// Espaces et retours a la ligne sont ignores ; tout le reste est
			// un caractere impossible et on renonce — comme le JS.
			if c == '\n' || c == '\r' || c == ' ' || c == '\t' {
				continue
			}
			return nil
		}
		tampon = (tampon << 6) | v
		bits += 6
		if bits >= 8 {
			bits -= 8
			octets = append(octets, (tampon>>bits)&0xFF)
		}
	}
	return octets
}

// OctetsVersBase64 : le retour, et pour la meme raison.
//
// ⚠️ SEUL LE GESTE S'EN SERT. La resolution ne touche JAMAIS au terrain : si un
// jour cette fonction est appelee depuis une passe, c'est un bug.
func OctetsVersBase64(octets []int) string {
	var sb strings.Builder
	for i := 0; i < len(octets); i += 3 {
		a := octets[i] & 0xFF
		b, c := 0, 0
		if i+1 < len(octets) {
			b = octets[i+1] & 0xFF
		}
		if i+2 < len(octets) {
			c = octets[i+2] & 0xFF
		}
		sb.WriteByte(alphabet64[a>>2])
		sb.WriteByte(alphabet64[((a&0x03)<<4)|(b>>4)])
		if i+1 < len(octets) {
			sb.WriteByte(alphabet64[((b&0x0F)<<2)|(c>>6)])
		} else {
			sb.WriteByte('=')
		}
		if i+2 < len(octets) {
			sb.WriteByte(alphabet64[c&0x3F])
		} else {
			sb.WriteByte('=')
		}
	}
	return sb.String()
}

// IndexCase : -1 si la case est hors plateau.
func IndexCase(largeur, hauteur, x, z int) int {
	if x < 0 || z < 0 || x >= largeur || z >= hauteur {
		return -1
	}
	return z*largeur + x
}

// Catalogue : ce que le chargement d'un plateau attend d'un catalogue.
type Catalogue interface {
	TuilePour(tid, niveau int) *Tuile
	Genres() *Genres
}

// Partie : un record `plateaux` charge, pret a jouer et pret a se reecrire.
type Partie struct {
	Id, OwnerId, Nom, TypeOfPlateau string
	Version                         int
	Largeur, Hauteur                int
	Tiles                           []int
	// Les etats que le catalogue a REFUSES, gardes tels quels pour etre
	// reecrits sans y toucher.
	Figes   []any
	Plateau *Plateau
	TDepart int

	TAvant       int
	EtatsAvant   []EtatEcrit
	ReserveAvant Sac
}

// ChargerPartie : un record -> la partie.
//
// `maintenant` ne sert qu'au cas du plateau NEUF : un `t` absent ou nul veut
// dire « il commence maintenant ». Sans ca, `Avancer` rejouerait 56 ans de
// cycles depuis l'epoque Unix.
func ChargerPartie(r Enregistrement, cat Catalogue, maintenant int) *Partie {
	g := cat.Genres()
	largeur := Entier(Champ(r, "largeur"), 0)
	hauteur := Entier(Champ(r, "hauteur"), 0)
	tiles := Base64VersOctets(Texte(Champ(r, "tilesBase64")))

	t := Entier(Champ(r, "t"), 0)
	if t == 0 {
		t = maintenant
	}

	var bats []*Batiment
	var figes []any
	for _, brut := range ListeJson(Champ(r, "etats")) {
		e, ok := brut.(map[string]any)
		if !ok || e == nil {
			continue
		}
		x, z := Entier(e["x"], 0), Entier(e["z"], 0)
		i := IndexCase(largeur, hauteur, x, z)
		if i < 0 || i >= len(tiles) {
			figes = append(figes, brut) // hors plateau
			continue
		}
		tid := tiles[i]
		niveau := Entier(e["niveau"], 1)
		if niveau < 1 {
			niveau = 1
		}
		tuile := cat.TuilePour(tid, niveau)
		if tuile == nil {
			figes = append(figes, brut) // tuile refusee : on la GARDE
			continue
		}
		// ⚠️ UN BATIMENT PAR ETAT RETENU, AVEC LE PALIER DE SA CASE : deux cases
		// de meme type mais de niveaux differents ne lisent pas la meme tuile.
		e["x"], e["z"], e["niveau"] = float64(x), float64(z), float64(niveau)
		bats = append(bats, CreerBatiment(g, e, tuile))
	}

	p := CreerPlateau(t, bats, nil, g, &MondeDuPlateau{Largeur: largeur, Hauteur: hauteur, Tiles: tiles}, nil)
	for code, q := range LireReserve(g, Champ(r, "reserve")) {
		p.Reserve.Set(code, q)
	}

	partie := &Partie{
		Id:            Texte(Champ(r, "id")),
		OwnerId:       Texte(Champ(r, "ownerId")),
		Nom:           Texte(Champ(r, "nom")),
		TypeOfPlateau: Texte(Champ(r, "typeOfPlateau")),
		Version:       Entier(Champ(r, "version"), 0),
		Largeur:       largeur, Hauteur: hauteur, Tiles: tiles,
		Figes: figes, Plateau: p, TDepart: t,
	}
	partie.TAvant = t
	partie.EtatsAvant = EcrireEtats(p)
	partie.ReserveAvant = p.Reserve.Copie()
	return partie
}

// LireReserve : la reserve du plateau, ASSAINIE — seules les entrees
// `code: entier >= 0` survivent.
func LireReserve(g *Genres, v any) map[Code]int {
	out := map[Code]int{}
	m := ObjetJson(v)
	if m == nil {
		return out
	}
	for code, val := range m {
		switch val.(type) {
		case float64, string:
		default:
			continue
		}
		if n := Entier(val, 0); n > 0 {
			out[g.Reg.Inscrire(code)] = n
		}
	}
	return out
}

// VersReserve : prete a repartir en base — un objet, jamais une chaine, et sans
// les zeros : une cle a 0 n'apprend rien.
func VersReserve(p *Plateau) map[string]int {
	return versNoms(p.Genres.Reg, &p.Reserve)
}

// DifferenceReserve : ce qui a bouge, `{ code: delta }`.
func DifferenceReserve(g *Genres, avant, apres Sac) map[string]int {
	bouge := map[string]int{}
	vus := map[Code]bool{}
	for _, c := range avant.NonNuls() {
		vus[c] = true
	}
	for _, c := range apres.NonNuls() {
		vus[c] = true
	}
	for _, c := range g.Reg.Alpha() {
		if !vus[c] {
			continue
		}
		if d := apres.Get(c) - avant.Get(c); d != 0 {
			bouge[g.Reg.Nom(c)] = d
		}
	}
	return bouge
}

// VersEtats : ceux que le moteur a joues, PLUS les figes, rendus tels qu'ils
// etaient.
//
// ⚠️ ON REND LE VRAI TABLEAU, PAS UNE CHAINE : un champ `json` PocketBase
// stockerait sinon le texte entre guillemets, et plus personne ne pourrait
// l'interroger cote serveur.
//
// ⚠️ ET JAMAIS `nil` : une tranche nulle s'ecrit `null` en base, pas `[]`. Le
// premier plateau fabrique par `assurer` (13/09) est sorti avec `etats: null`
// — le moteur le relit sans broncher, mais tout lecteur qui fait `etats.map()`
// (le site) casse sur `null`. Le §10 dit UNE LISTE, un plateau vide en est une
// de zero element. `EcrireEtats` partait deja d'une tranche vide ; ici la
// garantie se perdait au recopiage.
func (partie *Partie) VersEtats() []any {
	out := []any{}
	for _, e := range EcrireEtats(partie.Plateau) {
		out = append(out, e)
	}
	out = append(out, partie.Figes...)
	return out
}

// Ecrire : l'etat, la reserve et le temps, prets a repartir en base.
//
// ⚠️⚠️ ELLE NE VERIFIE PLUS QUE `t` EST RETENU, ET IL FAUT SAVOIR POURQUOI.
// Elle le faisait : elle reposait la valeur, la relisait, et levait si les deux
// differaient. **Ce garde-fou ne pouvait pas fonctionner** — releve le 13/09
// dans la source de PocketBase v0.39.2 : `Record.Set` sur un champ qui n'existe
// PAS dans la collection retombe sur `SetRaw`, la valeur est gardee dans le
// record, et `Get` la rend. La relecture tombait donc toujours juste, et la
// valeur etait perdue **au moment du SAVE**, pas du `Set`.
//
// ⚠️ SON ESSAI ETAIT VERT PARCE QUE SON FAUX AVALAIT AU `Set` — une forme que
// PocketBase n'a jamais eue. Meme famille que le 12/09 : un harnais qui
// confirme l'hypothese au lieu de l'eprouver. Et le trou etait REEL : au 13/09
// la collection `plateaux` n'avait toujours pas de champ `t`, donc le temps
// n'etait jamais range et **plus rien ne produisait**.
//
// ⚠️ LE VRAI GARDE-FOU EST DANS `routes.gardeSchema` : il demande au depot les
// champs que la collection retient VRAIMENT. Depuis `moteur/` on ne peut pas
// voir un schema — c'est justement la frontiere que ce paquet defend.
func (partie *Partie) Ecrire(r Enregistrement) {
	r.Set("etats", partie.VersEtats())
	r.Set("reserve", VersReserve(partie.Plateau))
	r.Set("t", partie.Plateau.T)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [24]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
