// ============================================================================
//  patch-partage-joueurs-2026-09-15.js — le PARTAGE PAR JOUEUR
//
//  À COLLER DANS LA CONSOLE DU NAVIGATEUR (F12), sur
//  https://pb-sysb.physiooffice.com/_/ , connecté en superuser.
//
//  CE QU'IL FAIT
//    1. tuile3dmodel.joueurs_autorises   relation multiple → users
//    2. icones.joueurs_autorises         relation multiple → users
//    3. icones.usage                     ajoute la valeur « autre »
//
//  POURQUOI — les onglets Partage des fiches 3DmodelTuile et Icônes du site
//  (15/09) ouvrent un modèle 3D ou une icône à TOUS les joueurs
//  (`toutes_planetes`, qui existe déjà) ou à des JOUEURS NOMMÉS (ce champ).
//  La règle, identique sur le site et dans le serveur Go :
//
//      autorisé  =  toutes_planetes
//                || la planète est dans planetes_autorisees
//                || son propriétaire est dans joueurs_autorises
//                || la planète n'a PAS de propriétaire   (planète game)
//
//  ⚠️ LISTE VIDE = À PERSONNE. Le patch pose des champs vides : il n'ouvre
//  rien à personne. `planetes_autorisees` (panneau Planète) n'est pas touché.
//
//  ⚠️ PAS de cascadeDelete : supprimer un compte ne doit pas réécrire en
//  silence les modèles 3D et les icônes. L'id orphelin reste, et le site le
//  montre (« compte supprimé ») pour qu'on le retire.
//
//  ⚠️ ORDRE : lancer ce patch AVANT de déployer le site du 15/09 au soir — sans
//  le champ, l'enregistrement d'une fiche répond 400 sur `joueurs_autorises`.
//  Le serveur Go, lui, lit un champ absent comme une liste vide : il peut
//  partir avant ou après.
//
//  ⚠️ IDEMPOTENT, et il n'écrit AUCUN record.
//
//  COMMENT S'EN SERVIR
//    1. coller tel quel   → il n'écrit RIEN, il affiche le plan ;
//    2. remettre GO = true, recoller ;
//    3. relire les ✅.
// ============================================================================

const GO = false; // ← passe à true pour écrire

(async () => {
  const jeton = JSON.parse(localStorage["__pb_superusers__/_"]).token;
  const API = location.origin;

  const appel = async (chemin, options = {}) => {
    const r = await fetch(`${API}${chemin}`, {
      ...options,
      headers: { "Content-Type": "application/json", Authorization: jeton, ...options.headers },
    });
    const corps = await r.json().catch(() => ({}));
    if (!r.ok)
      throw new Error(`${options.method || "GET"} ${chemin} → ${r.status} ${JSON.stringify(corps)}`);
    return corps;
  };

  const lire = async () => (await appel("/api/collections?perPage=200")).items;
  const par = (liste, nom) => {
    const c = liste.find((x) => x.name === nom);
    if (!c) throw new Error(`collection introuvable : ${nom} — les patches planètes sont-ils passés ?`);
    return c;
  };

  const collections = await lire();
  const users = par(collections, "users");

  const champJoueurs = {
    name: "joueurs_autorises",
    type: "relation",
    required: false,
    collectionId: users.id,
    cascadeDelete: false,
    minSelect: 0,
    maxSelect: 0, // 0 = sans plafond
  };

  // --- Le plan -------------------------------------------------------------
  const travaux = []; // { col, fields }
  for (const nom of ["tuile3dmodel", "icones"]) {
    const col = par(collections, nom);
    const fields = col.fields.map((f) => ({ ...f }));
    const aFaire = [];

    if (!fields.some((f) => f.name === "joueurs_autorises")) {
      fields.push(champJoueurs);
      aFaire.push("joueurs_autorises");
    }
    if (nom === "icones") {
      const usage = fields.find((f) => f.name === "usage");
      if (!usage) throw new Error("icones.usage introuvable — le patch 3 est-il passé ?");
      if (!(usage.values || []).includes("autre")) {
        usage.values = [...(usage.values || []), "autre"];
        aFaire.push("usage += autre");
      }
    }

    console.log(
      `%c${nom}%c : ${aFaire.length ? "à faire → " + aFaire.join(", ") : "déjà complet"}`,
      "font-weight:bold",
      "",
    );
    if (aFaire.length) travaux.push({ col, fields });
  }

  if (travaux.length === 0) {
    console.log("%cTout est déjà en place — rien à faire.", "color:#5cb85c");
    return;
  }
  if (!GO) {
    console.log("%cPLAN SEULEMENT — rien n'a été écrit. Repasse avec GO = true.", "color:#f0ad4e");
    return;
  }

  // --- L'écriture ----------------------------------------------------------
  // ⚠️ La liste COMPLÈTE des champs : envoyer les seuls nouveaux efface le reste.
  for (const { col, fields } of travaux) {
    await appel(`/api/collections/${col.id}`, { method: "PATCH", body: JSON.stringify({ fields }) });
    console.log(`écrit : ${col.name}`);
  }

  // --- Vérification --------------------------------------------------------
  const fin = await lire();
  const ok3d = par(fin, "tuile3dmodel").fields.some((f) => f.name === "joueurs_autorises");
  const icones = par(fin, "icones");
  const okIc = icones.fields.some((f) => f.name === "joueurs_autorises");
  const okAutre = (icones.fields.find((f) => f.name === "usage")?.values || []).includes("autre");
  console.log(`${ok3d ? "✅" : "❌"} tuile3dmodel.joueurs_autorises`);
  console.log(`${okIc ? "✅" : "❌"} icones.joueurs_autorises`);
  console.log(`${okAutre ? "✅" : "❌"} icones.usage contient « autre »`);
})();
