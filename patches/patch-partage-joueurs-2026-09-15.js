// ============================================================================
//  patch-partage-joueurs-2026-09-15.js — le PARTAGE PAR JOUEUR
//
//  À COLLER DANS LA CONSOLE DU NAVIGATEUR (F12), sur
//  https://pb-sysb.physiooffice.com/_/ , connecté en superuser.
//
//  CE QU'IL FAIT
//    1. tuile3dmodel.joueurs_autorises   relation MULTIPLE → users
//    2. icones.joueurs_autorises         relation MULTIPLE → users
//    3. icones.usage                     ajoute la valeur « autre »
//    4. RÉPARE tuile3dmodel.planetes_autorisees et icones.planetes_autorisees,
//       ainsi qu'un joueurs_autorises déjà posé par la première version de ce
//       patch : ils passent de relation SIMPLE à relation MULTIPLE.
//
//  ⚠️⚠️ LE PIÈGE (relevé en prod le 15/09) : `maxSelect: 0` ne veut PAS dire
//  « sans plafond ». PocketBase range le champ en relation SIMPLE (une seule
//  valeur, rendue en texte). Le patch 4 du 14/09 avait posé
//  `planetes_autorisees` ainsi : une icône ou un modèle ne pouvait être ouvert
//  qu'à UNE planète, et l'onglet Partage du site plantait (page noire).
//  → toujours un plafond EXPLICITE (ici 999). La valeur déjà en place est
//  conservée par PocketBase au passage en multiple.
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

  // ⚠️ PAS 0 : 0 fait une relation SIMPLE. Voir l'en-tête.
  const PLAFOND = 999;

  const collections = await lire();
  const users = par(collections, "users");

  const champJoueurs = {
    name: "joueurs_autorises",
    type: "relation",
    required: false,
    collectionId: users.id,
    cascadeDelete: false,
    minSelect: 0,
    maxSelect: PLAFOND,
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
    for (const f of fields) {
      if (
        (f.name === "joueurs_autorises" || f.name === "planetes_autorisees") &&
        f.type === "relation" &&
        !(f.maxSelect > 1)
      ) {
        aFaire.push(`${f.name} : simple (maxSelect ${f.maxSelect}) → multiple`);
        f.maxSelect = PLAFOND;
      }
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
  for (const nom of ["tuile3dmodel", "icones"]) {
    const col = par(fin, nom);
    for (const champ of ["joueurs_autorises", "planetes_autorisees"]) {
      const f = col.fields.find((x) => x.name === champ);
      const ok = !!f && f.maxSelect > 1;
      console.log(`${ok ? "✅" : "❌"} ${nom}.${champ} multiple (maxSelect ${f?.maxSelect})`);
    }
  }
  const okAutre = (par(fin, "icones").fields.find((f) => f.name === "usage")?.values || []).includes("autre");
  console.log(`${okAutre ? "✅" : "❌"} icones.usage contient « autre »`);
  // Contrôle sur une donnée : la relation doit revenir en TABLEAU.
  const un = await appel("/api/collections/tuile3dmodel/records?perPage=1");
  const v = un.items?.[0]?.planetes_autorisees;
  console.log(`${Array.isArray(v) ? "✅" : "❌"} un record rend planetes_autorisees en tableau :`, v);
})();
