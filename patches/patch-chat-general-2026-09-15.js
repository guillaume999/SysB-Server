// ============================================================================
//  patch-chat-general-2026-09-15.js — LE CHAT GENERAL DE L'ACCUEIL
//
//  À COLLER DANS LA CONSOLE DU NAVIGATEUR (F12), sur
//  https://pb-sysb.physiooffice.com/_/ , connecté en superuser.
//
//  CE QU'IL FAIT :
//    1. crée `chat_general` : auteur, auteur_nom, contenu (300 max) ;
//    2. si `signalements` existe déjà (patch des messages privés passé), lui
//       AJOUTE le champ `message_chat` — sans toucher aux autres champs.
//       Si elle n'existe pas encore, c'est le patch des messages privés qui le
//       posera : les deux patches se passent dans n'importe quel ordre.
//
//  LES RÈGLES D'API :
//    · lecture   : tout compte connecté (le chat vit sur l'accueil, après la
//                  connexion — un visiteur n'a rien à y lire).
//    · création  : connecté, `auteur` = soi. Le serveur Go juge ensuite :
//                  UN MESSAGE PAR MINUTE (429 sinon), texte nettoyé, pseudo
//                  recopié par lui (`pb/crochet_chat.go`).
//    · modification : personne. Un message du chat ne se réécrit pas.
//    · suppression  : l'admin (modération). Le serveur purge aussi, chaque
//                     nuit à 4 h, ce qui a plus de 7 jours.
//
//  ⚠️ CASCADE : supprimer un compte supprime ses messages du chat.
//  ⚠️ IDEMPOTENT, et il n'écrit AUCUN record.
//  ⚠️ ORDRE : ce patch AVANT le déploiement du serveur Go qui porte
//  `crochet_chat.go` — sinon le jeu lit un 404 à l'ouverture de l'accueil.
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
  if (!users.fields.some((f) => f.name === "pseudo"))
    throw new Error("users.pseudo introuvable — le chat recopie le pseudo");

  const ADMIN = "@request.auth.role = 'admin'";
  const CONNECTE = "@request.auth.id != ''";

  const aCreer = !trouver("chat_general");
  const sig = trouver("signalements");
  const aCompleter = !!sig && !sig.fields.some((f) => f.name === "message_chat");

  console.log(`%cchat_general%c : ${aCreer ? "à créer" : "déjà là — laissée telle quelle"}`, "font-weight:bold", "");
  console.log(`%csignalements.message_chat%c : ${!sig ? "collection absente — le patch des messages privés le posera" : aCompleter ? "à ajouter" : "déjà là"}`, "font-weight:bold", "");
  if (!aCreer && !aCompleter) {
    console.log("%cTout est déjà en place — rien à faire.", "color:#5cb85c");
    return;
  }
  if (!GO) {
    console.log("%cPLAN SEULEMENT — rien n'a été écrit. Repasse avec GO = true.", "color:#f0ad4e");
    return;
  }

  if (aCreer) {
    await appel("/api/collections", {
      method: "POST",
      body: JSON.stringify({
        name: "chat_general",
        type: "base",
        listRule: CONNECTE,
        viewRule: CONNECTE,
        createRule: `${CONNECTE} && @request.body.auteur = @request.auth.id`,
        updateRule: null,
        deleteRule: ADMIN,
        fields: [
          { name: "auteur", type: "relation", required: true, collectionId: users.id,
            cascadeDelete: true, minSelect: 0, maxSelect: 1 },
          { name: "auteur_nom", type: "text", required: false, max: 100 },
          { name: "contenu", type: "text", required: true, max: 300 },
          { name: "created", type: "autodate", onCreate: true, onUpdate: false },
          { name: "updated", type: "autodate", onCreate: true, onUpdate: true },
        ],
        indexes: [
          "CREATE INDEX idx_chat_created ON chat_general (created)",
          "CREATE INDEX idx_chat_auteur_created ON chat_general (auteur, created)",
        ],
      }),
    });
    console.log("créée : chat_general");
    collections = await lire();
  }

  if (aCompleter) {
    // ⚠️ `fields` COMPLET — existants + le neuf. N'envoyer que le neuf
    // effacerait tous les autres (CLAUDE.md §10).
    const chat = trouver("chat_general");
    await appel(`/api/collections/${sig.id}`, {
      method: "PATCH",
      body: JSON.stringify({
        fields: [
          ...sig.fields,
          { name: "message_chat", type: "relation", required: false, collectionId: chat.id,
            cascadeDelete: false, minSelect: 0, maxSelect: 1 },
        ],
      }),
    });
    console.log("ajouté : signalements.message_chat");
  }

  // --- Vérification --------------------------------------------------------
  const fin = await lire();
  const c = fin.find((x) => x.name === "chat_general");
  console.log(`${c ? "✅" : "❌"} chat_general existe${c ? ` — lecture « ${c.listRule} » · création « ${c.createRule} »` : ""}`);
  const s = fin.find((x) => x.name === "signalements");
  if (s) {
    const n = s.fields.length;
    const ok = s.fields.some((f) => f.name === "message_chat");
    console.log(`${ok ? "✅" : "❌"} signalements.message_chat (${n} champs — ${sig ? sig.fields.length : "?"} avant)`);
  }
  const r = await fetch(`${API}/api/collections/chat_general/records?perPage=1`);
  const j = await r.json().catch(() => ({}));
  console.log(`${r.ok && (j.totalItems ?? 0) > 0 ? "❌ FUITE" : "✅"} chat_general sans connexion : ${r.status}, ${j.totalItems ?? "-"} ligne(s)`);
})();
