// ============================================================
//  moteur/lecture.go — lire un champ de PocketBase sans se faire avoir
//
//  ⚠️⚠️ LE PIEGE QUI A COUTE LA PREMIERE PASSE A BLANC (31/08) N'EXISTE PLUS EN
//  Go, ET IL FAUT SAVOIR POURQUOI.
//
//  PocketBase range un champ `json` dans un `types.JSONRaw`, c'est-a-dire un
//  `[]byte`. Sous goja, un `[]byte` s'expose comme un TABLEAU DE NOMBRES :
//  `Array.isArray()` repondait donc VRAI sur un champ json non decode, et le JS
//  rendait un tableau d'octets qui avait toutes les apparences d'un tableau de
//  donnees. Concretement : les 8 etats d'un plateau devenaient **772 cases**
//  — 772 etant la longueur en octets du json — toutes a `x=0, z=0, t=0`. Le
//  plateau annoncait 772 cases et une absence de 56 ans, sans une ligne
//  d'erreur.
//
//  En Go un `[]byte` EST un `[]byte` : on le passe a `json.Unmarshal` et c'est
//  fini. Tout le detecteur d'octets du JS (`estTableauDOctets`, `texteUtf8`)
//  disparait — il ne defendait que contre une confusion de goja.
//
//  ⚠️⚠️ MAIS CE FICHIER NE VOIT JAMAIS LA FORME BRUTE DE POCKETBASE, ET C'EST
//  UNE CONDITION, PAS UN HASARD. PocketBase rend un champ `json` dans un
//  `types.JSONRaw` : un TYPE NOMME dont `[]byte` est le sous-jacent. Or **un
//  `switch v.(type)` de Go compare des types EXACTS, jamais des types
//  sous-jacents** — le `case []byte:` ci-dessous ne le verrait PAS.
//
//  C'est `pb.Normaliser` qui ramene ces formes a `[]byte` AVANT que le moteur ne
//  les voie ; l'adaptateur est le seul endroit qui a le droit de connaitre
//  PocketBase. Ne pas « rajouter un filet ici au cas ou » : deux endroits qui
//  reparent la meme chose, c'est le jour ou l'un des deux ment.
//
//  ⚠️ CE QUE CA A COUTE, POUR QUE PERSONNE NE DEFASSE LA CHAINE : au premier
//  demarrage reel (12/09), sans cette normalisation, TOUS les champs json
//  valaient nil — `GET /api/sysb/etat` rendait 500 « CATALOGUE ILLISIBLE » avec
//  `Ressources: 79, Tuiles: 28` mais **`TuilesAvecPalier: 0` ET
//  `TuilesRefusees: 0`**. Ce couple de zeros est la signature : des tuiles mal
//  saisies seraient REFUSEES, pas vides.
//
//  ⚠️ Ce qui RESTE vrai : un champ json peut arriver en objet deja decode, en
//  chaine, en chaine VIDE (champ jamais rempli), ou absent. Les quatre se
//  lisent ici, et une lecture ratee rend le zero du type — jamais une panne.
//  Meme parti pris que `DecoderEtats` cote C# : un plateau qui refuse de se
//  charger est pire qu'un plateau qui repart a vide sur ses etats.
// ============================================================

package moteur

import (
	"encoding/json"
	"strconv"
	"strings"
)

// Enregistrement : ce que le moteur attend d'un record PocketBase, et rien de
// plus.
//
// ⚠️ C'EST VOULU QUE POCKETBASE N'APPARAISSE PAS DANS CE PAQUET. Les vecteurs
// alimentent le modele DIRECTEMENT ; un echec de vecteur accuse donc
// l'algorithme, jamais la lecture du json — et reciproquement. Melanger les
// deux rendrait chaque panne illisible.
type Enregistrement interface {
	Get(nom string) any
	Set(nom string, valeur any)
}

// Entier : `parseInt` defensif — rend `defaut` si ce n'est pas un nombre.
func Entier(v any, defaut int) int {
	switch x := v.(type) {
	case nil:
		return defaut
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	case bool:
		if x {
			return 1
		}
		return 0
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return defaut
		}
		// `parseInt` de JS s'arrete au premier caractere non numerique ;
		// `Atoi` refuse. On prend le prefixe, comme JS.
		i := 0
		if i < len(s) && (s[i] == '-' || s[i] == '+') {
			i++
		}
		j := i
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		if j == i {
			return defaut
		}
		n, err := strconv.Atoi(s[:j])
		if err != nil {
			return defaut
		}
		return n
	}
	return defaut
}

// Texte : jamais nil, jamais "null".
func Texte(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case []byte:
		return string(x)
	}
	return ""
}

// LireJson : un champ `json` de PocketBase, sous n'importe laquelle de ses
// formes. Rend nil quand il n'y a rien a lire — jamais d'erreur.
func LireJson(v any) any {
	switch x := v.(type) {
	case nil:
		return nil
	case []byte:
		return depuisTexte(string(x))
	case string:
		return depuisTexte(x)
	case map[string]any, []any:
		return x // deja decode
	}
	return nil
}

func depuisTexte(s string) any {
	s = strings.TrimSpace(s)
	if s == "" || s == "null" {
		return nil
	}
	var out any
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil
	}
	return out
}

// Champ : un champ d'un record, ou nil.
func Champ(r Enregistrement, nom string) any {
	if r == nil {
		return nil
	}
	return r.Get(nom)
}

// ObjetJson / ListeJson : les deux formes utiles, deja typees.
func ObjetJson(v any) map[string]any {
	if m, ok := LireJson(v).(map[string]any); ok {
		return m
	}
	return nil
}

func ListeJson(v any) []any {
	if l, ok := LireJson(v).([]any); ok {
		return l
	}
	return nil
}
