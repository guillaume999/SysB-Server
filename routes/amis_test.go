package routes

import "testing"

func demande() DemandeAmi {
	return DemandeAmi{Demandeur: "a", Destinataire: "b", DestinataireExiste: true}
}

func TestDemandeAmi(t *testing.T) {
	if r := JugerDemandeAmi(demande()); r != "" {
		t.Fatalf("demande normale refusee : %q", r)
	}
	cas := map[string]func(*DemandeAmi){
		VerdictDestinataire: func(d *DemandeAmi) { d.DestinataireExiste = false },
		VerdictAmiSoi:       func(d *DemandeAmi) { d.Destinataire = "a" },
		VerdictBloque:       func(d *DemandeAmi) { d.Bloque = true },
		VerdictAmiDeja:      func(d *DemandeAmi) { d.DejaLies = true },
		VerdictAmisPlein:    func(d *DemandeAmi) { d.EngagesDemandeur = MaxAmis },
	}
	for attendu, modif := range cas {
		d := demande()
		modif(&d)
		if r := JugerDemandeAmi(d); r != attendu {
			t.Errorf("attendu %q, recu %q", attendu, r)
		}
	}
	d := demande()
	d.EngagesDemandeur = MaxAmis - 1
	if r := JugerDemandeAmi(d); r != "" {
		t.Errorf("29 engages : la 30e demande doit passer, recu %q", r)
	}
}

func acceptation() Acceptation {
	return Acceptation{Auteur: "b", Destinataire: "b", StatutAvant: StatutAttente, StatutApres: StatutAcceptee}
}

func TestAcceptation(t *testing.T) {
	if r := JugerAcceptation(acceptation()); r != "" {
		t.Fatalf("acceptation normale refusee : %q", r)
	}
	cas := map[string]func(*Acceptation){
		VerdictAmiDroit:   func(a *Acceptation) { a.Auteur = "a" },
		VerdictAmiModif:   func(a *Acceptation) { a.AutresChamps = []string{"demandeur"} },
		VerdictAutrePlein: func(a *Acceptation) { a.AmisAutre = MaxAmis },
		VerdictAmisPlein:  func(a *Acceptation) { a.AmisAuteur = MaxAmis },
	}
	for attendu, modif := range cas {
		a := acceptation()
		modif(&a)
		if r := JugerAcceptation(a); r != attendu {
			t.Errorf("attendu %q, recu %q", attendu, r)
		}
	}
	// Repasser une amitie acceptee en attente, ou re-accepter : refuse.
	a := acceptation()
	a.StatutAvant = StatutAcceptee
	if JugerAcceptation(a) != VerdictAmiModif {
		t.Error("re-acceptation acceptee")
	}
	a = acceptation()
	a.StatutApres = StatutAttente
	if JugerAcceptation(a) != VerdictAmiModif {
		t.Error("retour en attente accepte")
	}
	a = acceptation()
	a.AutresChamps = []string{"updated"}
	a.AmisAuteur, a.AmisAutre = MaxAmis-1, MaxAmis-1
	if r := JugerAcceptation(a); r != "" {
		t.Errorf("29 amis de chaque cote : l'acceptation doit passer, recu %q", r)
	}
}
