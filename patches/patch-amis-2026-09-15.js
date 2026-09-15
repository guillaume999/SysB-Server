// ============================================================================
//  patch-amis-2026-09-15.js — LES AMIS (messages privés réservés aux amis)
//
//  À COLLER DANS LA CONSOLE DU NAVIGATEUR (F12), sur
//  https://pb-sysb.physiooffice.com/_/ , connecté en superuser.
//
//  CE QU'IL FAIT — crée une collection, `amities` (et rien d'autre) :
//    demandeur, destinataire, demandeur_nom, destinataire_nom,
//    statut (attente | acceptee). UNE ligne par couple.
//
//  LES RÈGLES D'API :
//    · lecture     : les deux intéressés.
//    · création    : connecté, `demandeur` = soi, pas à soi, statut `attente`.
//                    Le serveur Go refuse ensuite : bloqué, déjà liés,
//                    30 amis (+ demandes envoyées) atteints ; il recopie les
//                    pseudos (`pb/crochet_amis.go`).
//    · modification : le destinataire, `attente` → `acceptee` et rien d'autre.
//                    Le serveur refuse si l'un des deux a déjà 30 amis.
//    · suppression : l'un ou l'autre (refuser, annuler, retirer un ami).
//
//  ⚠️⚠️ ORDRE : ce patch AVANT le déploiement du serveur Go qui porte
//  `crochet_amis.go`. Sans la collection, le serveur ne peut pas lire les
//  amitiés et REFUSE TOUS LES MESSAGES PRIVÉS (« pas amis »).
//  ⚠️ CASCADE : supprimer un compte supprime ses amitiés.
//  ⚠️ IDEMPOTENT, et il n'écrit AUCUN record.
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
  const collections = await lire();
  const users = collections.find((x) => x.name === "users");
  if (!users) throw new Error("collection users introuvable");
  if (!users.fields.some((f) => f.name === "pseudo"))
    throw new Error("users.pseudo introuvable — les amitiés recopient le pseudo");

  if (collections.some((x) => x.name === "amities")) {
    console.log("%camities%c : déjà là — laissée telle quelle. Rien à faire.", "font-weight:bold", "color:#5cb85c");
    return;
  }
  console.log("%camities%c : à créer", "font-weight:bold", "");
  if (!GO) {
    console.log("%cPLAN SEULEMENT — rien n'a été écrit. Repasse avec GO = true.", "color:#f0ad4e");
    return;
  }

  const CONNECTE = "@request.auth.id != ''";
  const MOI = "(demandeur = @request.auth.id || destinataire = @request.auth.id)";
  const versUser = (name) => ({
    name, type: "relation", required: true, collectionId: users.id,
    cascadeDelete: true, minSelect: 0, maxSelect: 1,
  });
  const intouchables = ["demandeur", "destinataire", "demandeur_nom", "destinataire_nom"]
    .map((c) => `@request.body.${c}:isset = false`).join(" && ");

  await appel("/api/collections", {
    method: "POST",
    body: JSON.stringify({
      name: "amities",
      type: "base",
      listRule: `${CONNECTE} && ${MOI}`,
      viewRule: `${CONNECTE} && ${MOI}`,
      createRule:
        `${CONNECTE} && @request.body.demandeur = @request.auth.id` +
        ` && @request.body.destinataire != @request.auth.id && @request.body.statut = 'attente'`,
      updateRule:
        `${CONNECTE} && destinataire = @request.auth.id && statut = 'attente'` +
        ` && @request.body.statut = 'acceptee' && ${intouchables}`,
      deleteRule: `${CONNECTE} && ${MOI}`,
      fields: [
        versUser("demandeur"),
        versUser("destinataire"),
        { name: "demandeur_nom", type: "text", required: false, max: 100 },
        { name: "destinataire_nom", type: "text", required: false, max: 100 },
        { name: "statut", type: "select", required: true, maxSelect: 1, values: ["attente", "acceptee"] },
        { name: "created", type: "autodate", onCreate: true, onUpdate: false },
        { name: "updated", type: "autodate", onCreate: true, onUpdate: true },
      ],
      indexes: [
        "CREATE UNIQUE INDEX idx_amities_couple ON amities (demandeur, destinataire)",
        "CREATE INDEX idx_amities_dest ON amities (destinataire, statut)",
      ],
    }),
  });
  console.log("créée : amities");

  const c = (await lire()).find((x) => x.name === "amities");
  console.log(`${c ? "✅" : "❌"} amities existe${c ? ` — création « ${c.createRule} »` : ""}`);
  if (c) console.log(`   modification « ${c.updateRule} »`);
  const r = await fetch(`${API}/api/collections/amities/records?perPage=1`);
  const j = await r.json().catch(() => ({}));
  console.log(`${r.ok && (j.totalItems ?? 0) > 0 ? "❌ FUITE" : "✅"} amities sans connexion : ${r.status}, ${j.totalItems ?? "-"} ligne(s)`);
})();
