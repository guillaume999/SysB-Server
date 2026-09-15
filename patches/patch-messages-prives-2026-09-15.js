// ============================================================================
//  patch-messages-prives-2026-09-15.js — MESSAGES PRIVES ENTRE JOUEURS
//
//  À COLLER DANS LA CONSOLE DU NAVIGATEUR (F12), sur
//  https://pb-sysb.physiooffice.com/_/ , connecté en superuser.
//
//  CE QU'IL FAIT — crée trois collections (et rien d'autre) :
//    1. messages_prives  expediteur, destinataire, expediteur_nom,
//                        destinataire_nom, contenu (1000 max), lu
//    2. blocages         bloqueur, bloque       (unique sur le couple)
//    3. signalements     signaleur, message, [message_chat], contenu_signale,
//                        auteur_signale, auteur_nom, motif, traite
//
//  LES RÈGLES D'API — elles disent QUI ; le serveur Go dit le reste
//  (`routes/messages.go` + `pb/crochet_messages.go`) :
//    · messages_prives  lecture : l'expéditeur et le destinataire, PERSONNE
//                       d'autre (pas même l'admin — la modération passe par
//                       les signalements).
//                       création : connecté, `expediteur` = soi, pas à soi.
//                       Le serveur refuse ensuite : bloqué, trop vite, vide,
//                       trop long — et RECOPIE lui-même les deux pseudos.
//                       modification : le destinataire, et seulement `lu`.
//                       suppression : superuser seulement.
//    · blocages         lecture / suppression : celui qui a bloqué.
//                       création : connecté, `bloqueur` = soi.
//    · signalements     création : connecté, `signaleur` = soi — le serveur
//                       vérifie qu'il est le DESTINATAIRE du message, et
//                       recopie le message signalé.
//                       lecture / modification / suppression : admin.
//
//  ⚠️ LE CONTRÔLE DES RÈGLES PASSE AVANT LE CROCHET GO (PocketBase v0.39,
//  apis/record_crud.go) : la règle ne voit que le corps envoyé. C'est pour ça
//  que le client envoie `expediteur` / `bloqueur` / `signaleur` lui-même, et
//  que les champs remplis par le serveur ne sont PAS requis.
//
//  ⚠️ CASCADE : supprimer un compte supprime ses messages (envoyés ET reçus)
//  et ses blocages. Un signalement survit : son auteur est simplement vidé,
//  le nom et le texte recopiés restent lisibles.
//
//  ⚠️ IDEMPOTENT : une collection déjà là est laissée telle quelle. Il n'écrit
//  AUCUN record.
//
//  ⚠️ ORDRE : ce patch AVANT le déploiement du serveur Go qui porte
//  `crochet_messages.go`. L'inverse ne casse rien (les crochets visent des
//  collections absentes), mais le jeu répondrait 404 à l'ouverture des messages.
//
//  COMMENT S'EN SERVIR
//    1. coller tel quel   → il n'écrit RIEN, il affiche le plan ;
//    2. remettre GO = true, recoller ;
//    3. relire les ✅.
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
  if (!users.fields.some((f) => f.name === "pseudo"))
    throw new Error("users.pseudo introuvable — les messages recopient le pseudo");

  const ADMIN = "@request.auth.role = 'admin'";
  const CONNECTE = "@request.auth.id != ''";
  const DATES = [
    { name: "created", type: "autodate", onCreate: true, onUpdate: false },
    { name: "updated", type: "autodate", onCreate: true, onUpdate: true },
  ];
  const versUser = (name, { required = true, cascade = true } = {}) => ({
    name, type: "relation", required, collectionId: users.id,
    cascadeDelete: cascade, minSelect: 0, maxSelect: 1,
  });
  const texte = (name, max, required = false) => ({ name, type: "text", required, max });

  const plan = [
    {
      nom: "messages_prives",
      corps: () => {
        const intouchables = ["contenu", "expediteur", "destinataire", "expediteur_nom", "destinataire_nom"]
          .map((c) => `@request.body.${c}:isset = false`).join(" && ");
        return {
          name: "messages_prives",
          type: "base",
          listRule: `${CONNECTE} && (expediteur = @request.auth.id || destinataire = @request.auth.id)`,
          viewRule: `${CONNECTE} && (expediteur = @request.auth.id || destinataire = @request.auth.id)`,
          createRule:
            `${CONNECTE} && @request.body.expediteur = @request.auth.id` +
            ` && @request.body.destinataire != @request.auth.id && @request.body.lu:isset = false`,
          updateRule: `${CONNECTE} && destinataire = @request.auth.id && ${intouchables}`,
          deleteRule: null,
          fields: [
            versUser("expediteur"),
            versUser("destinataire"),
            texte("expediteur_nom", 100),
            texte("destinataire_nom", 100),
            texte("contenu", 1000, true),
            { name: "lu", type: "bool", required: false },
            ...DATES,
          ],
          indexes: [
            "CREATE INDEX idx_mp_dest_lu ON messages_prives (destinataire, lu)",
            "CREATE INDEX idx_mp_exp_created ON messages_prives (expediteur, created)",
            "CREATE INDEX idx_mp_dest_created ON messages_prives (destinataire, created)",
          ],
        };
      },
    },
    {
      nom: "blocages",
      corps: () => ({
        name: "blocages",
        type: "base",
        listRule: `${CONNECTE} && bloqueur = @request.auth.id`,
        viewRule: `${CONNECTE} && bloqueur = @request.auth.id`,
        createRule: `${CONNECTE} && @request.body.bloqueur = @request.auth.id`,
        updateRule: null,
        deleteRule: `${CONNECTE} && bloqueur = @request.auth.id`,
        fields: [versUser("bloqueur"), versUser("bloque"), ...DATES],
        indexes: ["CREATE UNIQUE INDEX idx_blocages_couple ON blocages (bloqueur, bloque)"],
      }),
    },
    {
      nom: "signalements",
      corps: () => ({
        name: "signalements",
        type: "base",
        listRule: ADMIN,
        viewRule: ADMIN,
        createRule: `${CONNECTE} && @request.body.signaleur = @request.auth.id`,
        updateRule: ADMIN,
        deleteRule: ADMIN,
        fields: [
          versUser("signaleur"),
          { name: "message", type: "relation", required: false,
            collectionId: trouver("messages_prives").id, cascadeDelete: false, minSelect: 0, maxSelect: 1 },
          // ⚠️ Posé ici seulement si le chat existe déjà ; sinon c'est le patch
          // du chat qui l'ajoute. Les deux patches passent dans n'importe quel ordre.
          ...(trouver("chat_general")
            ? [{ name: "message_chat", type: "relation", required: false,
                 collectionId: trouver("chat_general").id, cascadeDelete: false, minSelect: 0, maxSelect: 1 }]
            : []),
          texte("contenu_signale", 1000),
          versUser("auteur_signale", { required: false, cascade: false }),
          texte("auteur_nom", 100),
          texte("motif", 500),
          { name: "traite", type: "bool", required: false },
          ...DATES,
        ],
        indexes: ["CREATE INDEX idx_signalements_traite ON signalements (traite, created)"],
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

  // --- L'écriture (messages_prives AVANT signalements, qui le cite) --------
  for (const p of aCreer) {
    await appel("/api/collections", { method: "POST", body: JSON.stringify(p.corps()) });
    console.log(`créée : ${p.nom}`);
    collections = await lire();
  }

  // --- Vérification --------------------------------------------------------
  const fin = await lire();
  for (const p of plan) {
    const c = fin.find((x) => x.name === p.nom);
    const dates = !!c && ["created", "updated"].every((n) => c.fields.some((f) => f.name === n));
    console.log(`${c ? "✅" : "❌"} ${p.nom} existe${c ? ` — dates ${dates ? "✅" : "❌"}` : ""}`);
    if (c) console.log(`   lecture « ${c.listRule} » · création « ${c.createRule} »`);
  }
  // Contrôle anonyme : SANS jeton, la lecture doit rendre une liste VIDE (règle
  // non satisfaite = 200 sans items) ou un refus — jamais un message.
  for (const nom of ["messages_prives", "blocages", "signalements"]) {
    const r = await fetch(`${API}/api/collections/${nom}/records?perPage=1`);
    const j = await r.json().catch(() => ({}));
    const fuite = r.ok && (j.totalItems ?? 0) > 0;
    console.log(`${fuite ? "❌ FUITE" : "✅"} ${nom} sans connexion : ${r.status}, ${j.totalItems ?? "-"} ligne(s)`);
  }
})();
