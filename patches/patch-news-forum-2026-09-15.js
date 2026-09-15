// ============================================================================
//  patch-news-forum-2026-09-15.js — NEWS et FORUM
//
//  À COLLER DANS LA CONSOLE DU NAVIGATEUR (F12), sur
//  https://pb-sysb.physiooffice.com/_/ , connecté en superuser.
//
//  CE QU'IL FAIT — crée quatre collections (et rien d'autre) :
//    1. news      titre, contenu (Markdown), image, publiee, auteur
//    2. salons    nom, description, ordre, sujets_joueurs, reponses_joueurs
//    3. sujets    salon, titre, contenu, auteur, auteur_nom
//    4. messages  sujet, contenu, auteur, auteur_nom
//
//  LES RÈGLES D'API — c'est ELLES qui protègent, pas le site :
//    · news      lecture : tout le monde, SAUF les brouillons (admin seul).
//                écriture : admin.
//    · salons    lecture : tout le monde. écriture : admin.
//    · sujets    lecture : tout le monde.
//                création : un compte connecté, EN SON NOM, et seulement si
//                le salon a `sujets_joueurs` coché (l'admin, lui, toujours).
//    · messages  lecture : tout le monde.
//                création : un compte connecté, en son nom, et seulement si
//                le salon du sujet a `reponses_joueurs` coché (admin toujours).
//    · sujets / messages — modification : l'auteur (sans pouvoir changer
//      d'auteur, de nom ni de salon/sujet) ou l'admin ; suppression : l'auteur
//      ou l'admin (modération).
//
//  ⚠️ `auteur_nom` EST RECOPIÉ, et la règle l'oblige à valoir le `pseudo` du
//  compte connecté. Pourquoi une copie : un visiteur anonyme lit le forum, mais
//  la règle de `users` lui interdit de lire les comptes — l'expand de `auteur`
//  lui rendrait du vide. Un pseudo changé plus tard ne réécrit pas les vieux
//  messages : c'est voulu (c'est ce qui a été signé à l'époque).
//
//  ⚠️ PocketBase ≥ 0.23 n'ajoute PLUS `created` / `updated` tout seul à une
//  collection créée par l'API : ils sont déclarés ici (autodate), sinon le
//  site ne pourrait ni trier ni dater.
//
//  ⚠️ CASCADE : supprimer un salon supprime ses sujets, supprimer un sujet
//  supprime ses réponses. Supprimer un COMPTE ne supprime rien (pas de
//  cascade sur `auteur`) — le nom recopié reste affiché.
//
//  ⚠️ IDEMPOTENT : une collection déjà là est laissée telle quelle (on
//  n'écrase pas des règles retouchées à la main). Il n'écrit AUCUN record.
//
//  ⚠️ ORDRE : lancer ce patch AVANT de déployer le site — sans les
//  collections, les pages News et Forum répondent 404.
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
  let collections = await lire();
  const trouver = (nom) => collections.find((x) => x.name === nom);
  const users = trouver("users");
  if (!users) throw new Error("collection users introuvable");
  if (!users.fields.some((f) => f.name === "pseudo"))
    throw new Error("users.pseudo introuvable — la règle du forum s'appuie dessus");

  const ADMIN = "@request.auth.role = 'admin'";
  const CONNECTE = "@request.auth.id != ''";
  const DATES = [
    { name: "created", type: "autodate", onCreate: true, onUpdate: false },
    { name: "updated", type: "autodate", onCreate: true, onUpdate: true },
  ];
  const auteur = { name: "auteur", type: "relation", required: true, collectionId: users.id, cascadeDelete: false, minSelect: 0, maxSelect: 1 };
  const auteurNom = { name: "auteur_nom", type: "text", required: false, max: 100 };
  const texte = (name, max, required = true) => ({ name, type: "text", required, max });

  // Chaque collection est décrite par une fonction : `sujets` a besoin de
  // l'id de `salons`, qui n'existe qu'après sa création.
  const plan = [
    {
      nom: "news",
      corps: () => ({
        name: "news",
        type: "base",
        listRule: `publiee = true || ${ADMIN}`,
        viewRule: `publiee = true || ${ADMIN}`,
        createRule: ADMIN,
        updateRule: ADMIN,
        deleteRule: ADMIN,
        fields: [
          texte("titre", 200),
          texte("contenu", 50000, false),
          { name: "image", type: "file", required: false, maxSelect: 1, maxSize: 5242880,
            mimeTypes: ["image/png", "image/jpeg", "image/webp", "image/gif"] },
          { name: "publiee", type: "bool", required: false },
          { ...auteur, required: false },
          ...DATES,
        ],
        indexes: ["CREATE INDEX idx_news_created ON news (created)"],
      }),
    },
    {
      nom: "salons",
      corps: () => ({
        name: "salons",
        type: "base",
        listRule: "",
        viewRule: "",
        createRule: ADMIN,
        updateRule: ADMIN,
        deleteRule: ADMIN,
        fields: [
          texte("nom", 100),
          texte("description", 1000, false),
          { name: "ordre", type: "number", required: false, onlyInt: true },
          { name: "sujets_joueurs", type: "bool", required: false },
          { name: "reponses_joueurs", type: "bool", required: false },
          ...DATES,
        ],
      }),
    },
    {
      nom: "sujets",
      corps: () => {
        const moi = `auteur = @request.auth.id && @request.body.auteur:isset = false && @request.body.auteur_nom:isset = false && @request.body.salon:isset = false`;
        return {
          name: "sujets",
          type: "base",
          listRule: "",
          viewRule: "",
          createRule:
            `${CONNECTE} && @request.body.auteur = @request.auth.id && @request.body.auteur_nom = @request.auth.pseudo` +
            ` && (${ADMIN} || salon.sujets_joueurs = true)`,
          updateRule: `${CONNECTE} && ((${moi}) || ${ADMIN})`,
          deleteRule: `${CONNECTE} && (auteur = @request.auth.id || ${ADMIN})`,
          fields: [
            { name: "salon", type: "relation", required: true, collectionId: trouver("salons").id, cascadeDelete: true, minSelect: 0, maxSelect: 1 },
            texte("titre", 200),
            texte("contenu", 20000),
            auteur,
            auteurNom,
            ...DATES,
          ],
          indexes: ["CREATE INDEX idx_sujets_salon ON sujets (salon)"],
        };
      },
    },
    {
      nom: "messages",
      corps: () => {
        const moi = `auteur = @request.auth.id && @request.body.auteur:isset = false && @request.body.auteur_nom:isset = false && @request.body.sujet:isset = false`;
        return {
          name: "messages",
          type: "base",
          listRule: "",
          viewRule: "",
          createRule:
            `${CONNECTE} && @request.body.auteur = @request.auth.id && @request.body.auteur_nom = @request.auth.pseudo` +
            ` && (${ADMIN} || sujet.salon.reponses_joueurs = true)`,
          updateRule: `${CONNECTE} && ((${moi}) || ${ADMIN})`,
          deleteRule: `${CONNECTE} && (auteur = @request.auth.id || ${ADMIN})`,
          fields: [
            { name: "sujet", type: "relation", required: true, collectionId: trouver("sujets").id, cascadeDelete: true, minSelect: 0, maxSelect: 1 },
            texte("contenu", 20000),
            auteur,
            auteurNom,
            ...DATES,
          ],
          indexes: ["CREATE INDEX idx_messages_sujet ON messages (sujet)"],
        };
      },
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

  // --- L'écriture (dans l'ordre : salons avant sujets avant messages) ------
  for (const p of aCreer) {
    const corps = p.corps();
    await appel("/api/collections", { method: "POST", body: JSON.stringify(corps) });
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
  // Contrôle anonyme : la lecture publique doit répondre 200 sans jeton.
  for (const nom of ["news", "salons", "sujets", "messages"]) {
    const r = await fetch(`${API}/api/collections/${nom}/records?perPage=1`);
    console.log(`${r.ok ? "✅" : "❌"} ${nom} lisible sans connexion (${r.status})`);
  }
})();
