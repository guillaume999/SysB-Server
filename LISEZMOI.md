# LE MOTEUR À CYCLES, EN Go — étapes 1, 2 et 3

> Écrit le 2026-09-12. **Portage** de `pb_hooks/moteur/cycles/` vers Go, décidé
> pour sortir de goja (spec `SPEC_MOTEUR_CYCLES.md` §11.1).

## ⚠️ Ce que c'est, et ce que ce n'est pas

**C'est un PORTAGE, pas une troisième traduction.** Il a été écrit en lisant le
moteur JS, pas seulement la spec. La confrontation contre le miroir Python
attrapera donc les fautes mécaniques, mais **plus les ambiguïtés du texte** —
c'est exactement ce que dit le §8quater : *« sur les trous corrigés après coup,
j'ai écrit les deux côtés : ces parties-là ne sont plus deux opinions
indépendantes »*. À garder en tête avant de croire un banc vert.

## Ce qui est fait, et ce qui le prouve

| | |
|---|---|
| `moteur/` — géométrie, sac, tuiles, objets, ressources, indice, proximité, navettes, cycles, trésorerie, états, **placement, geste** | **4 043 lignes** |
| **les 126 mesures des vecteurs** | **115 / 126 collent**, 11 écarts |
| les 11 écarts | **exactement** S3 ×2, S4 ×1, V3 ×2, F2 ×3, F3 ×3 — les mêmes qu'`ECARTS-2026-09-11.md` |
| invariance aux cadences (5, 60, 600, 3600, 10⁶ s) | **verte sur les 28 scénarios de temps** |
| ⚠️⚠️ **la sortie COMPLÈTE des 63 scénarios** | **identique au JS** — temps, placement ET geste |

C'est ce dernier point qui compte. 126 mesures, c'est une passoire ; ce qui est
comparé ici, c'est **tout ce que les deux moteurs savent dire** :

- pour un scénario de **temps** : `ecrireEtats` + la réserve — chaque coffre,
  chaque `t_cycle`, chaque `en_marche`, chaque navette en vol et sa cale ;
- pour un **placement** : la liste des blocages et des avertissements **mot pour
  mot**, plus le devis ligne à ligne (ressource, quantité, mode), `offert`,
  `offerts`, `deja`, `interdiction` ;
- pour un **geste** : `ok`, le texte du refus, les blocages, les
  avertissements, `paye` / `mobilise` / `perdu` / `rendu`, `chantier`,
  `ancien` / `apres` — **et l'état que le geste laisse derrière lui**.

    go run ./cmd/rejouer ../vecteurs/scenarios.json   # le banc
    go run ./cmd/photos  ../vecteurs/scenarios.json   # l'état complet, à diffuser contre le JS
    go run ./cmd/banc                                 # le coût d'une absence

## ⚠️⚠️ LA PERFORMANCE — et pourquoi le premier portage ne suffisait pas

Le moteur a ete ecrit DEUX FOIS le 12/09, et la difference vaut d'etre lue.

**Le premier jet transcrivait le JS** : chaque coffre etait une `map[string]int`.
Resultat : a peine plus rapide que node. Le profileur disait pourquoi — **41 %
du temps a hacher des noms de ressources** (`aeshashbody`,
`mapaccess1_faststr`). V8 range un petit objet a cles fixes dans une classe
cachee ; une map Go ne sait pas faire ca. **Le langage n'y etait pour rien : la
traduction etait mauvaise.**

**La refonte supprime les chaines du chemin chaud.** Une ressource devient un
`Code` (un entier) au chargement, un coffre devient un `Sac` (un tableau indexe
par ce `Code`), et plus une seule chaine n'est hachee pendant une passe. Voir
`sac.go`.

| colonie | absence | goja | node / V8 | Go, 1er jet | **Go, refondu** |
|---|---|---|---|---|---|
| 10 bâtiments | 1 jour | 8,5 s | 0,24 s | 0,08 s | **0,023 s** |
| 20 bâtiments | 1 jour | 20,4 s | 0,23 s | 0,16 s | **0,040 s** |
| 40 bâtiments | 1 jour | 57,5 s | 0,52 s | 0,35 s | **0,105 s** |
| 60 bâtiments | 1 jour | 125 s | 1,15 s | 0,72 s | **0,174 s** |
| 60 bâtiments | 7 jours | ≈ 15 min | 6,96 s | 4,8 s | **1,19 s** |
| 200 bâtiments | 7 jours | — | 36,2 s | 27,7 s | **6,25 s** |
| 200 bâtiments | 30 jours | — | *n'a pas fini* | *n'a pas fini* | **29,5 s** |

**×690 contre goja. ×6 contre node.** Une journee d'absence sur 60 cases passe
de deux minutes a deux dixiemes de seconde ; un MOIS d'absence sur 200 cases
tient en trente secondes, la ou ni goja ni node n'arrivaient au bout.

⚠️ **Ce qui reste au profil** : plus une seule chaine, mais 31 % dans
`growslice` avant qu'on dimensionne les deux tranches de `ciblesDe` — ramene a
la portion congrue. Le reste est du ramasse-miettes. La prochaine marche serait
de reutiliser les tampons d'une rupture a l'autre, et elle ne vaut probablement
plus le risque.

## Les optimisations, et comment CHACUNE a ete verifiee

Aucune ne change une regle. Toutes ont le meme controle : **les 28 photos
doivent rester identiques, au JS comme au Go d'avant**. Si une photo bouge,
c'est qu'on a change une regle sans le vouloir.

| | ce que c'est | pourquoi c'est sur |
|---|---|---|
| `Code` + `Sac` | une ressource est un entier, un coffre un tableau | le chargement est le seul endroit qui voit encore une chaine |
| `Appro.Transportables` | la liste des ressources d'une regle, calculee une fois | ne depend que de la regle |
| `candidatsGeo` | la geometrie des cibles, en cache pour la passe | ⚠️ **la geometrie SEULE** : `Vivant(t)` se redemande a chaque usage, parce qu'un chantier se termine en cours de passe. Mettre `aPorteeDe` entier en cache aurait fige les chantiers |
| `Facteur` | le facteur de proximite, en cache par batiment | ne depend que du terrain et de la tuile ; `ViderCaches()` l'invalide |
| `Ordonnes` | l'ordre `(z, x)`, trie une fois | ne change qu'a une pose ou une destruction |
| `Ligne.Tranches` | triees au chargement | le JS retriait a chaque production indexee |
| tranches dimensionnees | `ciblesDe` n'alloue plus a l'aveugle | pure allocation |

⚠️ `Plateau.ViderCaches()` est a appeler des qu'un **geste** change le terrain
ou la liste des batiments. Une passe, elle, ne change rien de ce qui est en
cache — c'est ce qui rend ces caches licites.

## ⚠️ Ce qui n'est PAS porté, et qui ne l'était pas non plus en JS

- **`ameliorer`** (poser un niveau > 1) — `geste.js` le disait déjà : non porté.
- **L'entretien et les effets des technos** — le moteur à cycles ne les porte
  pas du tout (décision du 11/09, spec §8bis B). C'est ce qui tient S3, S4 et V3
  ouverts, et c'est une décision de jeu, pas un trou de portage.
- **Le troisième état d'une techno** (« acquise mais en veille ») a disparu avec
  l'entretien : `RaisonVerrou` ne connaît plus que « acquise » ou « pas
  acquise ». V3 attend la même règle que S3 et S4.

## Étape 3 — ce qui est déjà là

| | |
|---|---|
| `lecture.go` | lire un champ PocketBase sous ses quatre formes, sans jamais planter |
| `plateau.go` | le codec base64 du terrain, un record `plateaux` ↔ le moteur, `Ecrire` et son garde-fou sur le champ `t` |
| `catalogue.go` | des records `ressources` / `tuiles` / `technologies` aux objets du moteur, avec les refus et les alertes |
| `partie.go` | **le bloc que lit Unity**, les garde-fous d'avant écriture, le diff des changements |
| `etats.go` → `VueDuClient`, `FlotteVue` | ce que le client dessine : satisfaction en %, position des navettes, la flotte telle que la fiche l'affiche |
| `cycles.go` → `AvancerBudget` | **le rattrapage par tranches** (spec §11.1) |

**6 115 lignes, 4 fichiers de tests.** Le tour complet d'une route est testé de
bout en bout : catalogue → record → rattrapage → garde-fous → écriture → bloc.

### ⚠️⚠️ LE RATTRAPAGE PAR TRANCHES — la garde ne condamne plus

L'ancien `Avancer` **levait** quand il dépassait `LimiteEvenements`. La
transaction était annulée, donc `plateau.t` ne bougeait pas, donc l'appel
suivant refaisait le même chemin jusqu'à la même exception : **une colonie qui
franchissait ce seuil ne se rattrapait plus jamais**, elle répondait 500 à
chaque rafraîchissement.

`AvancerBudget(p, jusqua, maxEvenements)` rend un `Progres{Fini, T}` : il
s'arrête sur le budget, **écrit le `t` réellement atteint**, et l'appelant
rappelle. Le §8ter avait déjà tranché que *« `passe` écrit dès que le temps a
avancé »* — il ne manquait que ce champ.

**Et c'est prouvé, pas supposé** (`tranches_test.go`) : le même intervalle rejoué
par tranches de **1, 7 puis 97 ruptures** — 4 318 appels pour le premier — rend
un état **identique au bit près** à celui du rattrapage en un seul coup. C'est
le cousin de l'invariance aux cadences, mais il ne la remplace pas : la cadence
découpe sur des instants ronds, le budget s'arrête à une rupture quelconque, au
milieu d'un vol de navette.

### ⚠️⚠️ UNE DIFFÉRENCE STRUCTURELLE AVEC LE JS, dans `catalogue.go`

Le JS fabriquait ses tuiles **à la demande** (`tuilePour(tid, niveau)` avec un
cache). Ici une ressource est un **entier attribué au chargement** : une tuile
construite *après* qu'un code soit apparu porterait un tableau de stockage trop
court, **en silence**. On construit donc TOUS les couples (tuile, niveau) d'un
coup, puis on ferme le catalogue avec `Refiger`.

C'est d'ailleurs ce que le JS voulait déjà — *« on juge TOUT le catalogue tout
de suite »* — il ne le faisait qu'au niveau 1. Et c'est testé : une règle d'appro
sans `ressources` doit voir un code déclaré par une ressource rencontrée **après**
la tuile qui la porte. Sans `Refiger`, ce test tombe.

⚠️ Même raison pour l'ordre de chargement des tuiles : il est trié par `tileId`,
parce que c'est lui qui décide la numérotation des codes. Une map Go se parcourt
au hasard — deux chargements du même catalogue auraient donné deux numérotations,
donc deux états écrits différents pour la même partie.

### Ce que la couche base a vérifié

- **le codec base64 du terrain**, croisé contre le JS sur **500 tirages
  aléatoires** : Go écrit ce que le JS relit, et réciproquement. C'est le seul
  encodage que Unity, PocketBase et les deux moteurs doivent lire pareil — une
  divergence décalerait tout le terrain d'une case.
- **le garde-fou du champ `t`** : un record factice qui *avale* le champ (ce que
  PocketBase fait en silence quand la colonne n'existe pas) doit faire lever
  `Ecrire`. Testé.
- **une tuile refusée garde sa case** : elle est figée et réécrite telle quelle,
  pas effacée. Testé.
- **le catalogue refuse et le DIT** : une tuile qui porte encore `par_minute` ne
  joue pas, et la raison part dans `Alertes`. Une tuile décochée sur le site rend
  `nil` **sans** alerte — c'est un état du catalogue, pas une faute. Testé.
- **les garde-fous d'avant écriture** : stock négatif, temps qui recule, case
  perdue sans geste. Les pannes les plus chères de ce projet ont toutes répondu
  200 ; ces trois-là refusent d'écrire un état qui ne peut pas être vrai. Testé.
- **le rattrapage par tranches descend jusqu'à la partie**, pas seulement au
  moteur : `Partie.Avancer(t, budget)` en 14 appels rend le même état qu'en un
  seul. Testé.

### ⚠️ Un piège du JS qui n'existe pas en Go

`estTableauDOctets` / `texteUtf8` (une cinquantaine de lignes de `catalogue.js`)
ne défendaient que contre **goja** : il expose un `[]byte` comme un tableau de
nombres, donc `Array.isArray()` répondait vrai sur un champ json non décodé —
d'où les *772 cases* d'un plateau qui en avait 8, le 31/08. En Go un `[]byte`
est un `[]byte` : tout ce détecteur disparaît.

## Les routes — `routes/` compilé et testé, `pb/` non

**La frontière tombe juste avant la première décision, et c'est délibéré.**
Tout ce qui peut se tromper — les garde-fous, l'ordre des opérations, le jeton
de concurrence, ce qu'on écrit et quand — vit dans `routes/`, qui ne connaît pas
PocketBase et qui **se compile et se teste ici**. Ce qui reste dans `pb/` est de
la plomberie : un routeur, une authentification, une transaction.

| | |
|---|---|
| `routes/` — `Etat`, `Passe`, `Geste` | ✅ compilé, **13 tests verts** |
| `pb/adaptateur.go` | ⚠️ **jamais compilé** |
| `cmd/sysb/main.go`, `Dockerfile`, `docker-compose.sysb-go.yml` | ⚠️ **jamais construits** |

### Ce que les tests des routes garantissent

- **`etat` n'écrit RIEN** — ni le `t`, ni rien d'autre — mais le bloc rendu est
  rattrapé en mémoire. C'est la seule route dont on est sûr qu'elle ne peut pas
  abîmer une partie.
- **`passe` ne touche JAMAIS à `version`.** Ce compteur dit « le TERRAIN a-t-il
  changé ? » ; l'incrémenter faisait refuser le geste d'un joueur par son propre
  rafraîchissement (09/09).
- **`passe` écrit dès que le temps a avancé**, même si rien n'a bougé dans les
  coffres : `t` EST l'information.
- **un plateau horodaté dans le futur n'est pas rattrapé**, et la raison le dit.
- **le rattrapage par tranches** remonte jusqu'à la route : `fini: false`, et
  chaque tranche écrit son `t` — sinon l'appel suivant repart du même instant.
- **`geste` monte `version` même quand il REFUSE** : le client doit savoir que sa
  lecture est périmée, même si rien n'a bougé.
- **un jeton de version périmé refuse et rend l'état frais**, sans toucher au sol.
- **une panne d'écriture répond 500**, elle ne répond pas 200.
- **un plateau qui n'est pas à moi est introuvable** (404) ou refusé (403).

### ⚠️ Ce qu'il reste à faire chez toi, et ce qui peut casser

    cd SysB/serveur-go
    go mod tidy                                  # va chercher PocketBase
    go build -tags pocketbase -o sysb ./cmd/sysb
    docker build -t sysb-pocketbase:1 .

Le `//go:build pocketbase` en tête de `pb/` et `cmd/sysb/` est là pour que
`go build ./...` et `go test ./...` restent verts **sans** le module. Retire-le
le jour où le module fait partie du projet pour de bon.

**Écrit pour PocketBase v0.39.2** — la version que sert l'instance, lue en bas
de l'admin le 12/09, et **pinglée dans `go.mod`**. L'API Go a changé en
profondeur à la v0.23 ; v0.39 est bien au-delà, donc les formes utilisées
(`app.OnServe()`, `se.Router`, `core.RequestEvent`, `e.Auth`,
`app.RunInTransaction`) sont les bonnes.

⚠️ **Attends-toi quand même à une ou deux corrections** : je n'ai pas pu
vérifier les signatures contre le vrai module. Si une ne colle pas, c'est dans
`pb/` et nulle part ailleurs.

⚠️ **Ne suis pas `latest`.** Un binaire maison sur une base qui bouge, c'est un
matin où plus rien ne compile.

⚠️ **Sors les anciens `pb_hooks/*.pb.js` du dossier monté** au moment de
basculer : deux routes du même nom, c'est la même classe de panne que deux
écrivains sur `plateaux`. `pb_data` ne bouge pas — on revient en arrière en
reposant l'ancien compose.

### ⚠️⚠️ NON PORTÉE, ET CELLE-LÀ BLOQUE : `assurer`

`POST /api/sysb/assurer` fabrique le plateau d'un joueur depuis le modèle de
l'admin (`assurer.pb.js`, 15,6 Ko : il copie la grille, **ré-horodate les états
à l'heure serveur** pour ne pas offrir au nouveau venu des semaines de
production, et pose `version` à 1). Elle n'existe pas en Go.

⚠️ **Et elle ne peut pas se remplacer côté client** : `plateaux` est fermée en
`role = 'admin'` en lecture comme en écriture depuis le 04/09 — aucun jeton de
joueur ne peut créer ce record. Conséquence mesurée, décidée **reportée le
12/09** :

| qui | ce qui se passe sur le serveur Go |
|---|---|
| un compte qui a déjà ses plateaux | **rien ne change**, tout marche |
| un compte neuf, ou un type de plateau jamais ouvert | **aucun plateau**, `POST /api/sysb/assurer` → 404 |

Donc : basculer en phase 3 est sans risque pour les comptes existants, mais
**aucune nouvelle colonie ne peut naître** tant que ce n'est pas porté. Le jeu
le dit maintenant à l'écran au lieu de rester vide (voir `SysBApi.Assurer`).

### ⚠️ `?type=` N'EXISTE PLUS — et le jeu ne le demande plus

Le JS acceptait `GET /api/sysb/etat?type=ground`. Le Go **ignorerait** le
paramètre et rendrait la LISTE avec un 200 : Unity lisait alors `plateau` à la
racine, ne trouvait rien, et affichait un écran vide **sans erreur** — la
famille des pannes qui répondent 200.

Tranché le 12/09 : le paramètre reste dehors, et c'est **le client** qui fait
les deux pas — `GET /api/sysb/etat` (la liste), il y cherche le
`typeOfPlateau`, puis `GET /api/sysb/etat?plateau=<id>`. Une route de moins à
maintenir, et un aller-retour de plus à l'ouverture.

⚠️ **Conséquence : `largeur`, `hauteur` et `typeOfPlateau` dans la liste ne sont
plus du confort d'affichage, ils sont load-bearing.** Test :
`TestLaListePorteLesDimensions`.

### Non portées : `recherche` et les deux routes à blanc

`recherche.pb.js` (28 Ko) achète une techno et la paie sur le plateau hôte ;
`passe-blanc` et `verdict-blanc` sont des essais à vide. Elles ne bloquent pas
une partie — mais tant qu'elles ne sont pas portées, **la recherche reste sur
les hooks JS**, ce qui veut dire faire tourner les deux serveurs. À faire avant
de basculer pour de bon.

## Ce qui reste

- ⚠️⚠️ **`assurer`** — la seule qui empêche un compte neuf de jouer. Voir plus haut.
- **`recherche`, `passe-blanc`, `verdict-blanc`** — voir plus haut. ✅ Vérifié le
  12/09 : **Unity n'appelle aucune des trois.** `TechnologieUI.Lancer` affiche un
  bandeau (« la recherche n'est pas encore branchée ») depuis le 06/09, et les
  routes à blanc n'ont jamais eu d'appelant dans le jeu. La question laissée
  ouverte par `ETAPES.md` (« si l'une de ces routes est utilisée par le jeu, il
  faut le savoir avant la phase 3 ») est donc **fermée** : le jeu n'appelle que
  cinq URL, et elles sont listées ci-dessous.

### Les cinq URL que le jeu appelle, et rien d'autre

Relevé exhaustif le 12/09 (`PocketBase.Call` n'apparaît que dans `SysBApi.cs`) :

| appel Unity | route Go |
|---|---|
| `GET /api/sysb/etat` | ✅ |
| `GET /api/sysb/etat?plateau=<id>` | ✅ |
| `POST /api/sysb/passe?plateau=<id>` | ✅ |
| `POST /api/sysb/geste` | ✅ |
| `POST /api/sysb/assurer` | ❌ **404** — reporté |

- **La première compilation et le déploiement**, qui demandent ta machine.
- **`ameliorer`** (poser un niveau > 1) et **l'entretien / les effets des
  technos** : non portés, comme en JS. Voir plus haut.
- ⚠️ **Rien de tout ça n'a encore été compilé contre PocketBase** : le module
  n'est pas joignable depuis la session (le proxy Go est bloqué). La couche
  `moteur/` est écrite exprès pour que PocketBase n'y apparaisse pas — c'est
  `Enregistrement`, une interface à deux méthodes, qui fait la frontière.

## Ce que le portage a trouvé dans le moteur JS

Trois choses, toutes écrites en commentaire à l'endroit concerné :

1. **`PlacesLogees` s'appuyait sur l'ordre de SAISIE** des clés de `stockage`
   (JS) — une map Go n'en a pas. Le Go trie. Aucune tuile du catalogue ne
   déclare deux `mobilise`, donc les deux coïncident aujourd'hui ; le jour où
   l'une le ferait, JS et Go divergeraient en silence. *(À trancher.)*
2. **Les groupes « en direct » dépendaient de l'ordre de création** : chaque
   groupe PRÉLÈVE, donc celui qui passe en premier se sert en premier. Une map
   Go les aurait parcourus au hasard → deux résultats pour la même colonie. Go
   garde une tranche ordonnée.
3. **`Retirer(p, nil, ...)`** — la trésorerie passe `null` comme bâtiment pour un
   `FluxStock`. Gratuit en JS, à vérifier à chaque déréférencement en Go.
4. **La liste de repli d'une règle d'appro sans `ressources`**, c'est
   `Object.keys(genres.table)` — **la table des genres, pas toutes les
   ressources connues**. Une ressource nommée par une ligne de tuile mais
   absente de la table ne voyage donc pas. En Go, où tout est indexé, il aurait
   été naturel de prendre « tous les codes inscrits » et d'élargir la règle en
   silence. `Genres.CodesTable()` existe pour ça.

Aucune des trois n'est un bug du JS. Les deux premières sont des endroits où le
JS avait raison *par accident de langage*.
