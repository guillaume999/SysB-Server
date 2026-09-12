# SysB — le serveur

Le moteur a cycles de SysB, en Go, et le binaire PocketBase qui l'expose.

Ce depot ne contient **que le serveur**. Le jeu Unity et le site vivent
ailleurs : ici, il n'y a que ce que GitHub doit compiler.

## Ce qu'il y a dedans

| dossier | ce que c'est |
|---|---|
| `moteur/` | le moteur a cycles — batiments, navettes, ressources, temps. Ne connait ni PocketBase ni HTTP. |
| `routes/` | ce que font `etat`, `passe` et `geste`. Ne connait pas PocketBase non plus : il parle a une interface `Depot`. |
| `pb/` | la seule partie qui touche PocketBase. ~200 lignes. |
| `cmd/sysb/` | le `main` du serveur. |
| `harnais/`, `cmd/*` | les outils de comparaison avec l'ancien moteur JS. |

Cette separation n'est pas de la decoration : elle veut dire que changer de
version de PocketBase ne peut casser que `pb/`, et que le moteur se teste sans
serveur ni base.

## Comment ca se deploie

Un `push` sur `main` declenche `.github/workflows/image.yml`, qui lance les
tests puis publie :

    ghcr.io/guillaume999/sysb-pocketbase:latest

Portainer tire cette image. Rien ne se construit a la main, ni sur le PC, ni
sur le NAS.

## ⚠️ Les deux choses a ne pas oublier

1. **`pb_data` n'est pas ici et n'y sera jamais** — ce sont les comptes des
   joueurs. Le volume reste sur le NAS.
2. **Les anciens `pb_hooks/*.pb.js` ne doivent plus etre montes.** Les routes
   sont dans le binaire ; deux fois la meme URL, et c'est le JS qui gagne.

## Ce qui n'est pas encore porte

`recherche.pb.js`, `passe-blanc`, `verdict-blanc`, `ameliorer` (niveau > 1),
les technos `entretien` / `effets`.
