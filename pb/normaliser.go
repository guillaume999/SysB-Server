// ============================================================
//  pb/normaliser.go — CE QUE L'ADAPTATEUR DONNE AU MOTEUR
//
//  ⚠️ PAS DE `//go:build pocketbase` EN TETE, ET C'EST TOUT L'INTERET. Ce
//  fichier repare un travers de PocketBase SANS importer PocketBase : il
//  reconnait la FORME (des octets, ou du texte, sous un type nomme), jamais le
//  NOM du type. Donc il compile et se teste comme le reste du moteur, alors que
//  `adaptateur.go`, lui, ne peut etre compile qu'avec le module.
//
//  C'est la difference qui compte : la panne du 12/09 est nee dans du code que
//  rien ne pouvait eprouver. Sa reparation, elle, a un test.
//
//  ⚠️⚠️ LE TRAVERS, EN UNE PHRASE. PocketBase rend un champ `json` dans un
//  `types.JSONRaw` — un TYPE NOMME dont `[]byte` est le sous-jacent. Un
//  `switch v.(type)` de Go compare des types EXACTS : cote moteur, `case
//  []byte:` ne voyait rien, et TOUS les champs json valaient nil (paliers,
//  logistique, placement, etats, reserve, technos).
//
//  ⚠️ POURQUOI ICI ET PAS DANS `moteur/lecture.go` : le moteur ne connait pas
//  PocketBase, c'est la frontiere que tout le portage defend. Une bizarrerie de
//  PocketBase se repare du cote PocketBase de la frontiere — a un seul endroit,
//  a l'entree.
// ============================================================

package pb

import "reflect"

// Normaliser : une valeur lue sur un record, ramenee a une forme que
// `moteur/lecture.go` connait.
//
//	types.JSONRaw (et tout type nomme sur []byte) -> []byte
//	tout type nomme sur string                    -> string
//	le reste                                      -> inchange
//
// ⚠️ ELLE NE DECODE RIEN. Elle change la forme, pas le sens : c'est le moteur
// qui lit le json, et lui seul. Une normalisation qui deciderait aussi du
// contenu serait un second lecteur.
//
// ⚠️ ET ELLE NE TOUCHE QUE CES DEUX FORMES. Un `types.DateTime` passe
// inchange : le convertir en texte ici lui collerait ses guillemets json, et
// personne ne verrait la difference avant longtemps.
func Normaliser(v any) any {
	switch v.(type) {
	case nil, string, []byte, bool, int, int64, float64,
		map[string]any, []any:
		// Les formes que le moteur lit deja. Le cas courant, et il ne coute rien.
		return v
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Slice:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return rv.Bytes() // types.JSONRaw
		}
	case reflect.String:
		return rv.String()
	}
	return v
}
