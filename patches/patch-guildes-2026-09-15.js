// ============================================================================
//  patch-guildes-2026-09-15.js — LES GUILDES
//
//  À COLLER DANS LA CONSOLE DU NAVIGATEUR (F12), sur
//  https://pb-sysb.physiooffice.com/_/ , connecté en superuser.
//
//  CE QU'IL FAIT — crée cinq collections (et rien d'autre) :
//    1. reglages_guildes  membres_max, officiers_max, delai_salon_s,
//                         age_min_creation  (UNE fiche, écrite par l'onglet
//                         admin « Guildes » du site ; sans fiche, le serveur
//                         applique ses défauts : 20 / 3 / 10 s / âge 1)
//    2. guildes           nom (unique, casse ignorée), description, chef
//    3. membres_guilde    guilde, joueur (UNE guilde par joueur), joueur_nom, role
//    4. demandes_guilde   guilde, guilde_nom, joueur, joueur_nom, sens
//                         (demande | invitation), unique (guilde, joueur)
//    5. messages_guilde   guilde, auteur, auteur_nom, contenu (500)
//
//  LES RÈGLES D'API — ⚠️ AUCUNE ÉCRITURE JOUEUR : tout passe par les routes
//  du serveur Go (`/api/sysb/guildes`, `/api/sysb/guilde/...`), qui jugent
//  (`routes/guildes.go`). L'admin, lui :
//    · lit et écrit `reglages_guildes` ;
//    · lit guildes, membres, demandes et le salon ; dissout une guilde
//      (suppression) et modère le salon (suppression d'un message).
//
//  ⚠️ CASCADE : dissoudre une guilde emporte ses membres, ses demandes et son
//  salon. Supprimer le compte du chef dissout la guilde. Supprimer un autre
//  compte le retire de sa guilde ; ses messages restent (nom recopié).
//
//  ⚠️ IDEMPOTENT, et il n'écrit AUCUN record.
//  ⚠️ ORDRE : ce patch AVANT le déploiement du serveur Go qui porte
//  `pb/guildes.go`, et avant le site (onglets Guildes / Ma guilde).
//
//  COMMENT S'EN SERVIR : coller tel quel (plan seulement), puis GO = true.
// ============================================================================

const GO = true; // ← passe à true pour écrire

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
  let collections = await lire();
  const trouver = (nom) => collections.find((x) => x.name === nom);
  const users = trouver("users");
  if (!users) throw new Error("collection users introuvable");
  if (!trouver("ages")) throw new Error("collection ages introuvable — l'âge de jeu s'y lit");

  const ADMIN = "@request.auth.role = 'admin'";
  const DATES = [
    { name: "created", type: "autodate", onCreate: true, onUpdate: false },
    { name: "updated", type: "autodate", onCreate: true, onUpdate: true },
  ];
  const nombre = (name, min) => ({ name, type: "number", required: false, onlyInt: true, min });
  const texte = (name, max, required = false) => ({ name, type: "text", required, max });
  const rel = (name, cible, { required = true, cascade = true } = {}) => ({
    name, type: "relation", required, collectionId: trouver(cible).id,
    cascadeDelete: cascade, minSelect: 0, maxSelect: 1,
  });
  const lectureAdmin = { listRule: ADMIN, viewRule: ADMIN, createRule: null, updateRule: null };

  const plan = [
    {
      nom: "reglages_guildes",
      corps: () => ({
        name: "reglages_guildes", type: "base",
        listRule: ADMIN, viewRule: ADMIN, createRule: ADMIN, updateRule: ADMIN, deleteRule: ADMIN,
        fields: [
          nombre("membres_max", 1),
          nombre("officiers_max", 0),
          nombre("delai_salon_s", 0),
          nombre("age_min_creation", 0),
          ...DATES,
        ],
      }),
    },
    {
      nom: "guildes",
      corps: () => ({
        name: "guildes", type: "base", ...lectureAdmin, deleteRule: ADMIN,
        fields: [
          { ...texte("nom", 30, true), min: 3 },
          texte("description", 500),
          rel("chef", "users"),
          ...DATES,
        ],
        indexes: ["CREATE UNIQUE INDEX idx_guildes_nom ON guildes (nom COLLATE NOCASE)"],
      }),
    },
    {
      nom: "membres_guilde",
      corps: () => ({
        name: "membres_guilde", type: "base", ...lectureAdmin, deleteRule: null,
        fields: [
          rel("guilde", "guildes"),
          rel("joueur", "users"),
          texte("joueur_nom", 100),
          { name: "role", type: "select", required: true, maxSelect: 1, values: ["chef", "officier", "membre"] },
          ...DATES,
        ],
        indexes: [
          "CREATE UNIQUE INDEX idx_membres_guilde_joueur ON membres_guilde (joueur)",
          "CREATE INDEX idx_membres_guilde_guilde ON membres_guilde (guilde)",
        ],
      }),
    },
    {
      nom: "demandes_guilde",
      corps: () => ({
        name: "demandes_guilde", type: "base", ...lectureAdmin, deleteRule: null,
        fields: [
          rel("guilde", "guildes"),
          texte("guilde_nom", 30),
          rel("joueur", "users"),
          texte("joueur_nom", 100),
          { name: "sens", type: "select", required: true, maxSelect: 1, values: ["demande", "invitation"] },
          ...DATES,
        ],
        indexes: ["CREATE UNIQUE INDEX idx_demandes_guilde_couple ON demandes_guilde (guilde, joueur)"],
      }),
    },
    {
      nom: "messages_guilde",
      corps: () => ({
        name: "messages_guilde", type: "base", ...lectureAdmin, deleteRule: ADMIN,
        fields: [
          rel("guilde", "guildes"),
          rel("auteur", "users", { required: false, cascade: false }),
          texte("auteur_nom", 100),
          texte("contenu", 500, true),
          ...DATES,
        ],
        indexes: [
          "CREATE INDEX idx_messages_guilde_created ON messages_guilde (guilde, created)",
          "CREATE INDEX idx_messages_guilde_auteur ON messages_guilde (guilde, auteur, created)",
        ],
      }),
    },
  ];

  // --- Le plan -------------------------------------------------------------
  const aCreer = plan.filter((p) => !trouver(p.nom));
  for (const p of plan)
    console.log(`%c${p.nom}%c : ${trouver(p.nom) ? "déjà là — laissée telle quelle" : "à créer"}`, "font-weight:bold", "");
  if (aCreer.length === 0) {
    console.log("%cTout est déjà en place — rien à faire.", "color:#5cb85c");
    return;
  }
  if (!GO) {
    console.log("%cPLAN SEULEMENT — rien n'a été écrit. Repasse avec GO = true.", "color:#f0ad4e");
    return;
  }

  // --- L'écriture, dans l'ordre (guildes avant ce qui la cite) -------------
  for (const p of aCreer) {
    await appel("/api/collections", { method: "POST", body: JSON.stringify(p.corps()) });
    console.log(`créée : ${p.nom}`);
    collections = await lire();
  }

  // --- Vérification --------------------------------------------------------
  const fin = await lire();
  for (const p of plan) {
    const c = fin.find((x) => x.name === p.nom);
    console.log(`${c ? "✅" : "❌"} ${p.nom}${c ? ` — lecture « ${c.listRule ?? "superuser"} » · création « ${c.createRule ?? "superuser"} »` : ""}`);
  }
  for (const nom of plan.map((p) => p.nom)) {
    const r = await fetch(`${API}/api/collections/${nom}/records?perPage=1`);
    const j = await r.json().catch(() => ({}));
    const fuite = r.ok && (j.totalItems ?? 0) > 0;
    console.log(`${fuite ? "❌ FUITE" : "✅"} ${nom} sans connexion : ${r.status}, ${j.totalItems ?? "-"} ligne(s)`);
  }
})();
