package pb

import "testing"

// ⚠️⚠️ LE TEST QUI MANQUAIT LE 12/09.
//
// `types.JSONRaw` est declare ici par sa FORME, pas par son nom : c'est ce qui
// permet de l'eprouver sans le module PocketBase — le module que la session
// Claude ne peut pas atteindre, et dans lequel la panne est nee.
type jsonRaw []byte // la forme de `types.JSONRaw`

type texteNomme string // la forme d'un `type Nom string`

func TestNormaliserRendLesOctetsDUnTypeNomme(t *testing.T) {
	// Le cas qui a tout casse : 28 tuiles lues, 0 palier, 0 refusee.
	v := Normaliser(jsonRaw(`[{"niveau":1,"cycle_minutes":2}]`))
	b, ok := v.([]byte)
	if !ok {
		t.Fatalf("⚠️ un champ json de PocketBase doit ressortir en []byte, recu %T", v)
	}
	if string(b) != `[{"niveau":1,"cycle_minutes":2}]` {
		t.Errorf("contenu abime : %s", b)
	}

	if s, ok := Normaliser(texteNomme("colonie")).(string); !ok || s != "colonie" {
		t.Errorf("un type nomme sur string doit ressortir en string : %#v", s)
	}
}

// ⚠️ CE QU'ELLE NE DOIT PAS FAIRE : decoder, ou toucher a ce qui va bien.
func TestNormaliserNeTouchePasAuReste(t *testing.T) {
	for _, v := range []any{
		nil, "texte", []byte("abc"), true, 3, int64(3), 4.5,
		map[string]any{"or": 1.0}, []any{1.0},
	} {
		if got := Normaliser(v); !memeForme(v, got) {
			t.Errorf("%T ne doit pas changer : %#v -> %#v", v, v, got)
		}
	}
	// ⚠️ Un type qui n'est ni octets ni texte passe INCHANGE — surtout pas
	// converti en texte. Un `types.DateTime` transforme ici arriverait au moteur
	// avec ses guillemets json, et personne ne le verrait avant longtemps.
	type dateNommee struct{ T string }
	d := dateNommee{"2026-09-12"}
	if got := Normaliser(d); got != any(d) {
		t.Errorf("une structure doit passer inchangee : %#v", got)
	}
}

func memeForme(a, b any) bool {
	if a == nil {
		return b == nil
	}
	sa, aok := a.([]byte)
	sb, bok := b.([]byte)
	if aok || bok {
		return aok && bok && string(sa) == string(sb)
	}
	ma, aok := a.(map[string]any)
	mb, bok := b.(map[string]any)
	if aok || bok {
		return aok && bok && len(ma) == len(mb)
	}
	la, aok := a.([]any)
	lb, bok := b.([]any)
	if aok || bok {
		return aok && bok && len(la) == len(lb)
	}
	return a == b
}
