// ============================================================================
//  patch-conception-joueur-2026-09-15.js — LES JOUEURS CONÇOIVENT
//
//  À COLLER DANS LA CONSOLE DU NAVIGATEUR (F12), sur
//  https://pb-sysb.physiooffice.com/_/ , connecté en superuser.
//
//  CE QU'IL FAIT
//    1. crée la collection `limites` (onglet Limites du site) :
//         nom · general · joueurs · largeur_max · hauteur_max
//         · tuiles_max · ressources_max · technos_max
//    2. relève `tilesBase64` à 250 000 caractères sur `templates` et
//       `plateaux` (une grille de 200 x 200 sur DEUX octets en demande 106 668) ;
//    3. pose l'index UNIQUE sur `tuiles.tileId` s'il manque (c'est lui qui
//       empêche deux créations simultanées de prendre le même numéro) ;
//    4. ouvre l'écriture aux joueurs, SUR LEUR PLANÈTE SEULEMENT :
//         tuiles / ressources / technologies
//           create : admin || planete.proprietaire = moi
//           update / delete : admin || planete.proprietaire = moi
//         templates
//           update : admin || (planete.proprietaire = moi
//                              && ni planete, ni appartient, ni typeOfPlateau
//                                 dans le corps)
//           create / delete : RESTENT admin (le serveur fabrique les modèles)
//
//  ⚠️⚠️ LES RÈGLES NE SUFFISENT PAS, ET C'EST VOULU : les quotas, la taille
//  des plateaux, la grille (pas de tuile du jeu peinte chez soi) et le numéro
//  des tuiles sont tenus par le SERVEUR GO (`pb/crochet_conception.go`).
//  ⚠️ ORDRE : DÉPLOYER LE SERVEUR D'ABORD, puis lancer ce patch. Dans l'autre
//  ordre, un joueur pourrait écrire sur sa planète sans quota pendant l'écart.
//
//  ⚠️ ESSAYÉ SUR UN VRAI POCKETBASE 0.39.2 (banc local, 15/09) avant d'être
//  livré : la règle de création qui lit `planete.proprietaire` est bien
//  évaluée sur le record ENVOYÉ (PocketBase crée un record fantôme pour juger).
//
//  ⚠️ IDEMPOTENT : ce qui est déjà en place est laissé. Il n'écrit AUCUN record.
//  ⚠️ JAMAIS « Settings → Import collections ».
//
//  COMMENT S'EN SERVIR
//    1. coller tel quel → il n'écrit RIEN, il affiche le plan ;
//    2. passer `GO` à true DANS LE FICHIER, puis recoller le tout.
//  ⚠️ Ne pas taper `GO = true` dans la console (c'est une `const`).
// ============================================================================

(async () => {
  const GO = false; // ← passe à true pour écrire

  const ADMIN = "@request.auth.role = 'admin'";
  const CHEZ_MOI = "planete.proprietaire = @request.auth.id";
  const REGLES_CONCUES = {
    createRule: `${ADMIN} || ${CHEZ_MOI}`,
    updateRule: `${ADMIN} || ${CHEZ_MOI}`,
    deleteRule: `${ADMIN} || ${CHEZ_MOI}`,
  };
  const REGLES_TEMPLATES = {
    updateRule:
      `${ADMIN} || (${CHEZ_MOI} && @request.body.planete:isset = false` +
      ` && @request.body.appartient:isset = false && @request.body.typeOfPlateau:isset = false)`,
    createRule: ADMIN,
    deleteRule: ADMIN,
  };
  const TAILLE_GRILLE = 250000;

  const jeton = JSON.parse(localStorage["__pb_superusers__/_"]).token;
  const API = location.origin;
  const appel = async (chemin, options = {}) => {
    const r = await fetch(`${API}${chemin}`, {
      ...options,
      headers: { "Content-Type": "application/json", Authorization: jeton, ...options.headers },
    });
    const corps = await r.json().catch(() => ({}));
    if (!r.ok) throw new Error(`${options.method || "GET"} ${chemin} → ${r.status} ${JSON.stringify(corps)}`);
    return corps;
  };
  const lire = async () => (await appel("/api/collections?perPage=200")).items;
  let collections = await lire();
  const par = (nom) => {
    const c = collections.find((x) => x.name === nom);
    if (!c) throw new Error(`collection introuvable : ${nom}`);
    return c;
  };

  // --- Préalables : sans eux, les règles citeraient des champs absents -----
  for (const nom of ["tuiles", "ressources", "technologies", "templates"]) {
    if (!par(nom).fields.some((f) => f.name === "planete"))
      throw new Error(`${nom}.planete introuvable — patch-relation-planete-2026-09-14.js est-il passé ?`);
  }
  if (!par("planetes").fields.some((f) => f.name === "proprietaire"))
    throw new Error("planetes.proprietaire introuvable — patch-planetes-2026-09-14.js est-il passé ?");
  const users = par("users");

  const plan = []; // { quoi, faire: async () => {} }

  // --- 1. limites ----------------------------------------------------------
  if (!collections.some((c) => c.name === "limites")) {
    const nombre = (name) => ({ name, type: "number", required: false, onlyInt: true, min: 0 });
    plan.push({
      quoi: "créer la collection `limites`",
      faire: () =>
        appel("/api/collections", {
          method: "POST",
          body: JSON.stringify({
            name: "limites",
            type: "base",
            // Un joueur lit SES fiches (et la générale) : le site lui montre
            // « 3 / 10 tuiles ». Il n'en écrit aucune.
            listRule: `${ADMIN} || general = true || joueurs.id ?= @request.auth.id`,
            viewRule: `${ADMIN} || general = true || joueurs.id ?= @request.auth.id`,
            createRule: ADMIN,
            updateRule: ADMIN,
            deleteRule: ADMIN,
            fields: [
              { name: "nom", type: "text", required: true, max: 100 },
              { name: "general", type: "bool", required: false },
              // ⚠️ maxSelect EXPLICITE > 1 : 0 fait une relation SIMPLE (panne du 15/09).
              { name: "joueurs", type: "relation", required: false, collectionId: users.id,
                cascadeDelete: false, minSelect: 0, maxSelect: 999 },
              nombre("largeur_max"),
              nombre("hauteur_max"),
              nombre("tuiles_max"),
              nombre("ressources_max"),
              nombre("technos_max"),
              // ⚠️ PocketBase n'ajoute pas les dates à une collection créée par l'API.
              { name: "created", type: "autodate", onCreate: true, onUpdate: false },
              { name: "updated", type: "autodate", onCreate: true, onUpdate: true },
            ],
          }),
        }),
    });
  }

  // --- 2. la taille des grilles ----------------------------------------------
  for (const nom of ["templates", "plateaux"]) {
    const col = par(nom);
    const f = col.fields.find((x) => x.name === "tilesBase64");
    if (!f) throw new Error(`${nom}.tilesBase64 introuvable`);
    if (!(f.max >= TAILLE_GRILLE)) {
      plan.push({
        quoi: `${nom}.tilesBase64 : max ${f.max} → ${TAILLE_GRILLE}`,
        faire: () => {
          // ⚠️ La liste COMPLÈTE des champs : envoyer le seul modifié efface le reste.
          const fields = col.fields.map((x) => (x.name === "tilesBase64" ? { ...x, max: TAILLE_GRILLE } : x));
          return appel(`/api/collections/${col.id}`, { method: "PATCH", body: JSON.stringify({ fields }) });
        },
      });
    }
  }

  // --- 3. l'index unique sur tuiles.tileId -----------------------------------
  {
    const col = par("tuiles");
    const unique = (col.indexes || []).some((i) => /CREATE\s+UNIQUE\s+INDEX/i.test(i) && /\(\s*`?tileId`?\s*\)/.test(i));
    if (!unique) {
      plan.push({
        quoi: "index UNIQUE sur tuiles.tileId",
        faire: async () => {
          // ⚠️ Deux tuiles au même numéro = l'index est refusé. Le dire avant.
          const tout = [];
          for (let page = 1; ; page++) {
            const r = await appel(`/api/collections/tuiles/records?perPage=500&page=${page}&fields=id,tileId,nom`);
            tout.push(...r.items);
            if (page >= r.totalPages || r.items.length === 0) break;
          }
          const vus = new Map();
          for (const t of tout) {
            if (vus.has(t.tileId)) throw new Error(`tileId ${t.tileId} en double : « ${vus.get(t.tileId)} » et « ${t.nom} » — à corriger avant l'index`);
            vus.set(t.tileId, t.nom);
          }
          return appel(`/api/collections/${col.id}`, {
            method: "PATCH",
            body: JSON.stringify({ indexes: [...(col.indexes || []), "CREATE UNIQUE INDEX `idx_tuiles_tileId_unique` ON `tuiles` (`tileId`)"] }),
          });
        },
      });
    }
  }

  // --- 4. les règles ---------------------------------------------------------
  const regles = (nom, voulues) => {
    const col = par(nom);
    const diff = Object.entries(voulues).filter(([k, v]) => col[k] !== v);
    if (diff.length === 0) return;
    plan.push({
      quoi: `${nom} : ` + diff.map(([k, v]) => `\n     ${k} « ${col[k]} » → « ${v} »`).join(""),
      faire: () => appel(`/api/collections/${col.id}`, { method: "PATCH", body: JSON.stringify(Object.fromEntries(diff)) }),
    });
  };
  for (const nom of ["tuiles", "ressources", "technologies"]) regles(nom, REGLES_CONCUES);
  regles("templates", REGLES_TEMPLATES);

  // --- Le plan ---------------------------------------------------------------
  if (plan.length === 0) {
    console.log("%cTout est déjà en place — rien à faire.", "color:#5cb85c");
    return;
  }
  console.log("%cPLAN :", "font-weight:bold");
  plan.forEach((p, i) => console.log(`  ${i + 1}. ${p.quoi}`));
  if (!GO) {
    console.log("%cPLAN SEULEMENT — rien n'a été écrit. Passe GO à true dans le fichier et recolle.", "color:#f0ad4e");
    return;
  }

  // --- L'écriture — dans l'ordre du plan -------------------------------------
  for (const p of plan) {
    await p.faire();
    console.log(`écrit : ${p.quoi.split("\n")[0]}`);
    collections = await lire();
  }

  // --- Vérification ----------------------------------------------------------
  collections = await lire();
  const verifier = (ok, texte) => console.log(`${ok ? "✅" : "❌"} ${texte}`);
  const lim = collections.find((c) => c.name === "limites");
  verifier(!!lim, "collection limites");
  if (lim) {
    const j = lim.fields.find((f) => f.name === "joueurs");
    verifier(j && j.maxSelect > 1, "limites.joueurs est une relation MULTIPLE");
  }
  for (const nom of ["templates", "plateaux"])
    verifier(par(nom).fields.find((f) => f.name === "tilesBase64").max >= TAILLE_GRILLE, `${nom}.tilesBase64 ≥ ${TAILLE_GRILLE}`);
  verifier((par("tuiles").indexes || []).some((i) => /UNIQUE/i.test(i) && /tileId/.test(i)), "index unique tuiles.tileId");
  for (const nom of ["tuiles", "ressources", "technologies"])
    verifier(Object.entries(REGLES_CONCUES).every(([k, v]) => par(nom)[k] === v), `règles de ${nom}`);
  verifier(Object.entries(REGLES_TEMPLATES).every(([k, v]) => par("templates")[k] === v), "règles de templates");
})();
