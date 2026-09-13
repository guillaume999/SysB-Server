# LA PROCÉDURE — coder ici, GitHub compile, le NAS sert

> Réécrit le 12/09, corrigé le 13/09 (le miroir `SysB-Server/` et
> `publier-le-serveur` n'existent plus — ce fichier les nommait encore, et c'est
> comme ça qu'on envoie quelqu'un lancer un script supprimé).
> **Une boucle, cinq étapes, aucune compilation à la main.**
>
> ⚠️⚠️ **CE FICHIER DÉCRIVAIT AUTRE CHOSE JUSQU'AU 12/09** : il faisait installer
> Go, compiler `sysb.exe` en local, rapatrier une copie de `pb_data` du NAS et
> lancer un PocketBase sur le PC pour essayer les routes. **Ce n'est pas la
> procédure.** C'était bon pour la toute première mise en place ; ça n'a rien à
> faire dans la boucle de tous les jours, et ça fabrique un deuxième PocketBase
> qui n'est pas celui qui sert. Il n'y a qu'un PocketBase : **celui du NAS**.

---

## 1. Coder — dans `serveur-go/`, et nulle part ailleurs

⚠️⚠️ **CE DOSSIER **EST** LE DÉPÔT depuis le 13/09.** Il n'y a plus de miroir :
le `SysB-Server/` qu'un script régénérait a disparu, et `publier-le-serveur.ps1`
avec lui. Ce que tu vois ici est exactement ce qui part et exactement ce que
GitHub compile. **Édite sans crainte : plus rien ne t'écrase.**

⚠️ Le dossier est **imbriqué dans le dépôt du projet Unity**. C'est la ligne
`serveur-go/` du `.gitignore` de SysB qui empêche l'autre dépôt de le voir — ne
pas la retirer.

## 2. Pousser — `serveur-go\pousser.bat`

Double-clic. Il commit ce dossier et le pousse sur `guillaume999/SysB-Server`.
Il demande un message de commit : **Entrée** met la date du jour.

⚠️ Il refuse de partir si `go.sum` manque (`go mod tidy`), si une base de
données a atterri dans le dossier, si un fichier indispensable est ignoré par
git, ou s'il n'y a aucun workflow.

⚠️ **Ne pas le confondre avec `SysB\pousser-sur-github.bat`**, qui pousse le
**projet Unity** (`guillaume999/SySB`, branche `main1`). Deux dossiers, deux
dépôts, deux scripts — et le jour où l'on se trompe, on cherche son travail
dans le mauvais dépôt.

## 3. GitHub compile ET teste — tout seul, ~2 min

Le push sur `main` déclenche `.github/workflows/image.yml`, qui enchaîne :

    go vet ./...
    go test ./...
    docker build (linux/amd64 + linux/arm64)
    push -> ghcr.io/guillaume999/sysb-pocketbase:latest

⚠️⚠️ **C'EST ICI QUE LE CODE EST COMPILÉ ET TESTÉ, PAS SUR TON PC.** Si les
tests sont rouges, **aucune image ne part** — c'est le garde-fou, et c'est
pour ça qu'il n'y a rien à lancer en local.

Suivre : https://github.com/guillaume999/SysB-Server/actions

## 4. Déployer — Portainer

Stack `pocketbase-sysb` → re-tirer l'image → redéployer.
⚠️ `pb_data` ne bouge pas. ⚠️ `pb_hooks` ne doit PAS être monté (les routes sont
dans le binaire ; deux fois la même URL, et c'est le JS qui gagne).

## 5. Tester — sur le NAS, avec le vrai PocketBase

- l'admin répond : https://pb-sysb.physiooffice.com/_/
- `GET /api/sysb/etat` rend un plateau (c'est la route qui n'écrit jamais)
- le jeu Unity se connecte et avance

**Retour arrière** : reposer l'ancien compose (image `muchobien/pocketbase` +
montage `pb_hooks`) et redéployer. Une minute, `pb_data` intact.

---

## ⚠️ Ce qui reste à faire à la main, une fois pour toutes

- ~~`assurer` n'est pas portée~~ — **portée le 13/09** (`routes/assurer.go`).
  Elle refuse encore, mais pour de bonnes raisons : **404** s'il n'y a aucun
  modèle de ce type dans `templates`, **422** si le modèle n'a pas de dimensions
  ou une grille qui ne fait pas `largeur × hauteur`.
  ⚠️ **Elle ne transcrit PAS `assurer.pb.js`** : le JS écrivait un `t` et un
  `chantier` **par état**, c'est-à-dire le format d'avant le moteur à cycles.
  Le plateau neuf part au format §10, à l'heure du serveur, cycles à zéro.
- La première fois seulement : l'image sort **privée** sur GHCR et Portainer ne
  peut pas la tirer → github.com/guillaume999?tab=packages → `sysb-pocketbase`
  → Package settings → Change visibility → Public.

## ⚠️ Si tu lances quand même `go test` sur ton PC

Sur cette machine, `sysb/routes` échoue sur :

    An Application Control policy has blocked this file.

**Ce n'est pas un test rouge** : c'est **Smart App Control** de Windows qui
refuse d'exécuter le binaire de test que Go vient de fabriquer dans `Temp`.
`moteur` et `pb` passent, et sur GitHub (Linux) les trois passent. Même piège
que l'erreur Burst 4551 — voir la note `sysb_burst_smart_app_control`.

C'est une raison de plus de laisser GitHub faire les tests.
