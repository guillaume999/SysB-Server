# LA PROCÉDURE — coder ici, GitHub compile, le NAS sert

> Réécrit le 12/09. **Une boucle, cinq étapes, aucune compilation à la main.**
>
> ⚠️⚠️ **CE FICHIER DÉCRIVAIT AUTRE CHOSE JUSQU'AU 12/09** : il faisait installer
> Go, compiler `sysb.exe` en local, rapatrier une copie de `pb_data` du NAS et
> lancer un PocketBase sur le PC pour essayer les routes. **Ce n'est pas la
> procédure.** C'était bon pour la toute première mise en place ; ça n'a rien à
> faire dans la boucle de tous les jours, et ça fabrique un deuxième PocketBase
> qui n'est pas celui qui sert. Il n'y a qu'un PocketBase : **celui du NAS**.

---

## 1. Coder — dans `serveur-go/`, et nulle part ailleurs

`serveur-go/` est la source de vérité. `SysB-Server/` en est une **copie
générée** par le script de l'étape 2 : on n'y touche jamais à la main, elle sera
écrasée sans prévenir.

## 2. Publier — `publier-le-serveur.bat`

Double-clic. Il recopie la liste blanche vers `SysB-Server/`, commit, push.
Il demande un message de commit : **Entrée** met la date du jour.

⚠️ Il refuse de partir si `go.sum` manque (`go mod tidy`) ou si une base de
données a atterri dans le dépôt.

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
