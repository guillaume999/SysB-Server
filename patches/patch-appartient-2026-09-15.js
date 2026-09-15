// ============================================================
//  patch-appartient-2026-09-15.js
//  Ajoute `templates.appartient` et le remplit sur l'existant.
//
//  OU LE LANCER : l'admin PocketBase (pb-sysb.physiooffice.com/_/), connecte
//  en superuser, console F12 (onglet Console), coller TOUT le fichier.
//
//  1. Laisser GO = false et coller : le patch LIT et dit ce qu'il ferait.
//  2. Changer la LIGNE `const GO = false` en `true` DANS CE FICHIER, puis
//     recoller tout. (Taper `GO = true` dans la console ne marche pas.)
//
//  CE QU'IL FAIT :
//    · champ texte `appartient` sur `templates` (non requis) ;
//    · chaque modele dont `appartient` est vide recoit :
//        - l'id du proprietaire de sa planete, si c'est une planete de joueur ;
//        - "game" sinon (Terre, Jupiter, modele sans planete).
//
//  ⚠️ VIDE NE VEUT PAS DIRE « game ». Apres ce patch, un modele vide est un
//  modele que personne n'a range — le site le signale.
//  ⚠️ IDEMPOTENT : champ deja la = saute, modele deja rempli = laisse.
//  ⚠️ A LANCER AVANT de deployer le serveur du 15/09 : sans le champ, le
//  serveur refuse de creer les planetes des joueurs (et le journalise).
// ============================================================

(async () => {
  const GO = false;

  const cle = Object.keys(localStorage).find((k) => k.startsWith("__pb_superusers__"));
  const token = cle ? JSON.parse(localStorage.getItem(cle) || "{}").token : "";
  if (!token) {
    console.error("❌ Pas de session superuser dans cette page. Connecte-toi a l'admin PocketBase.");
    return;
  }
  const api = async (chemin, options = {}) => {
    const r = await fetch(chemin, {
      ...options,
      headers: { "Content-Type": "application/json", Authorization: token, ...(options.headers || {}) },
    });
    const corps = await r.json().catch(() => ({}));
    if (!r.ok) throw new Error(`${options.method || "GET"} ${chemin} → ${r.status} ${JSON.stringify(corps)}`);
    return corps;
  };
  const tout = async (collection) => {
    const out = [];
    for (let page = 1; ; page++) {
      const r = await api(`/api/collections/${collection}/records?perPage=500&page=${page}`);
      out.push(...r.items);
      if (page >= r.totalPages) return out;
    }
  };

  // ─── 1. Le champ ─────────────────────────────────────────────────────────
  const col = await api("/api/collections/templates");
  const aLeChamp = col.fields.some((f) => f.name === "appartient");
  if (aLeChamp) {
    console.log("✓ `templates.appartient` existe deja.");
  } else if (!GO) {
    console.log("→ ajouterait le champ texte `templates.appartient`.");
  } else {
    // ⚠️ TOUS les champs existants + le neuf : n'envoyer que le neuf
    //    effacerait les autres.
    await api(`/api/collections/${col.id}`, {
      method: "PATCH",
      body: JSON.stringify({
        fields: [...col.fields, { name: "appartient", type: "text", required: false, max: 50 }],
      }),
    });
    console.log("✅ champ `templates.appartient` ajoute.");
  }

  // ─── 2. L'existant ───────────────────────────────────────────────────────
  const planetes = await tout("planetes");
  const proprietaire = new Map(planetes.map((p) => [p.id, (p.proprietaire || "").trim()]));
  const modeles = await tout("templates");
  let faits = 0;
  for (const m of modeles) {
    if ((m.appartient || "").trim()) continue;
    const valeur = proprietaire.get(m.planete || "") || "game";
    console.log(`${GO ? "✅" : "→"} « ${m.nom} » (${m.typeOfPlateau}) : appartient = ${valeur}`);
    if (GO) {
      await api(`/api/collections/templates/records/${m.id}`, {
        method: "PATCH",
        body: JSON.stringify({ appartient: valeur }),
      });
    }
    faits++;
  }
  console.log(
    faits === 0
      ? "✓ Aucun modele a remplir."
      : `${GO ? "✅" : "→"} ${faits} modele(s) ${GO ? "remplis" : "a remplir"}.` +
          (GO ? "" : " Passe GO a true dans le fichier et recolle."),
  );
})();
