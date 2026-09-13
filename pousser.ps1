# ============================================================
#  pousser.ps1 — ENVOIE CE DOSSIER SUR GITHUB
#
#  Ce dossier EST le depot. Rien n'est copie ailleurs : ce que tu vois ici est
#  exactement ce qui part, et exactement ce que GitHub va compiler.
#
#  Un push sur `main` declenche `.github/workflows/image.yml` : tests, puis
#  construction et publication de
#      ghcr.io/guillaume999/sysb-pocketbase:latest
#  que Portainer tire.
#
#  ⚠️ IL N'Y A PLUS DE MIROIR (2026-09-13). Ce dossier etait recopie dans un
#  `SysB-Server/` qu'un script regenerait ; les deux versions pouvaient diverger.
#  Le dossier est desormais le depot lui-meme, et `publier-le-serveur.ps1` a ete
#  supprime. Edite ici, sans crainte : plus rien ne t'ecrase.
#
#  ⚠️ IL EST IMBRIQUE DANS LE DEPOT DU PROJET UNITY. La ligne `serveur-go/` du
#  `.gitignore` de SysB est ce qui empeche l'autre depot de le voir. Ne la retire
#  pas : sans elle, le depot du jeu s'en plaint a chaque commit, ou pire,
#  l'enregistre comme un lien vide.
# ============================================================

$ErrorActionPreference = 'Continue'
$url = 'https://github.com/guillaume999/SysB-Server.git'

function Titre($t) { Write-Host ''; Write-Host "=== $t ===" -ForegroundColor Cyan }
function Bien($t)  { Write-Host "  $t" -ForegroundColor Green }
function Alerte($t){ Write-Host "  $t" -ForegroundColor Yellow }
function Halte($t) { Write-Host ''; Write-Host "  ARRET : $t" -ForegroundColor Red
                     Write-Host ''; Read-Host 'Entree pour fermer'; exit 1 }

Set-Location $PSScriptRoot

Write-Host ''
Write-Host '=============================================' -ForegroundColor Cyan
Write-Host '  SysB-Server — pousser sur GitHub' -ForegroundColor Cyan
Write-Host '=============================================' -ForegroundColor Cyan
Write-Host "  $PSScriptRoot"

# --- 1. Les garde-fous -----------------------------------------------------
Titre 'Verifications'

if (-not (Get-Command git -ErrorAction SilentlyContinue)) {
  Halte 'git est introuvable. https://git-scm.com/download/win'
}
Bien 'git present'

# ⚠️ Sans `go.sum`, la construction chez GitHub echoue sur « missing go.sum
# entry » — et l'erreur arrive deux minutes plus tard, loin d'ici.
if (-not (Test-Path 'go.sum')) { Halte 'go.sum est absent.' }
Bien 'go.sum present'

# ⚠️ CEINTURE ET BRETELLES. `pb_data` contient les comptes des joueurs. Meme si
# `.gitignore` le couvre, on refuse de pousser s'il a atterri ici.
$bases = Get-ChildItem . -Recurse -Force -Include 'data.db','*.db-wal','*.db-shm' -ErrorAction SilentlyContinue |
         Where-Object { $_.FullName -notmatch '\\\.git\\' }
if ($bases) {
  $bases | ForEach-Object { Write-Host "  $($_.FullName)" -ForegroundColor Red }
  Halte 'Une base de donnees se trouve dans le dossier. Retire-la avant de pousser.'
}
Bien 'aucune base de donnees ici'

# ⚠️ LE MAIN DOIT PARTIR. Un motif `.gitignore` trop large peut avaler un
# dossier entier sans le moindre avertissement : le 12/09, la ligne `sysb`
# (sans barre oblique) a emporte `cmd/sysb/`, et on ne l'a su que deux minutes
# plus tard, dans le journal de GitHub. On demande donc a git lui-meme, ici,
# avant d'envoyer quoi que ce soit.
Titre 'Le main part-il vraiment ?'
if (Test-Path '.git') {
  $indispensables = @('cmd/sysb/main.go', 'pb/adaptateur.go', 'Dockerfile',
                      'go.mod', 'go.sum')
  $manquants = @()
  foreach ($f in $indispensables) {
    git check-ignore -q $f
    if ($LASTEXITCODE -eq 0) { $manquants += "$f (ignore par .gitignore)" }
    elseif (-not (Test-Path $f)) { $manquants += "$f (absent du dossier)" }
  }
  if ($manquants) {
    $manquants | ForEach-Object { Write-Host "  $_" -ForegroundColor Red }
    Halte 'Des fichiers indispensables ne partiraient pas. L image ne se construirait pas.'
  }
  Bien 'les fichiers indispensables partent bien'
} else {
  Alerte 'depot pas encore initialise, verification reportee'
}

# --- 2. Le workflow a sa place ---------------------------------------------
# Les outils distants de Claude n'ont pas le droit d'ecrire dans un dossier
# `.github`. Le fichier arrive donc a plat, et c'est ici qu'il reprend le chemin
# exact sans lequel GitHub ne le voit pas.
Titre 'Le workflow'
if (Test-Path 'workflow-image.yml') {
  New-Item -ItemType Directory -Path '.github\workflows' -Force | Out-Null
  Copy-Item 'workflow-image.yml' '.github\workflows\image.yml' -Force
  Bien 'workflow-image.yml -> .github/workflows/image.yml'
} elseif (Test-Path '.github\workflows\image.yml') {
  Bien 'deja en place'
} else {
  Halte 'Aucun workflow : GitHub ne construirait pas d image.'
}

# --- 3. Le depot -----------------------------------------------------------
Titre 'Le depot'
if (-not (Test-Path '.git')) {
  Alerte 'Premier envoi : initialisation.'
  git init | Out-Null
  git branch -M main
  git remote add origin $url
  Bien "branche main, origin -> $url"
} else {
  $o = git remote get-url origin 2>$null
  if ($o -ne $url) {
    Alerte "origin vaut « $o », on le remet sur $url"
    git remote set-url origin $url
  }
  Bien 'depot deja initialise'
}

# --- 4. Commit -------------------------------------------------------------
Titre 'Commit'
git add -A
$change = git status --porcelain
if (-not $change) {
  Bien 'Rien n a change depuis le dernier envoi.'
  Write-Host ''
  Alerte 'Pour reconstruire l image quand meme : onglet Actions sur GitHub,'
  Alerte 'workflow « image », bouton « Run workflow ».'
  Write-Host ''
  Read-Host 'Entree pour fermer'; exit 0
}
$change | Select-Object -First 40 | ForEach-Object { Write-Host "  $_" }
if ($change.Count -gt 40) { Write-Host "  ... et $($change.Count - 40) autres" }

Write-Host ''
$msg = Read-Host 'Message du commit (Entree = date du jour)'
if (-not $msg) { $msg = "serveur — $(Get-Date -Format 'dd/MM/yyyy HH:mm')" }
git commit -m $msg | Out-Null
Bien 'commit fait'

# --- 5. Push ---------------------------------------------------------------
Titre 'Envoi'
git push -u origin main
if ($LASTEXITCODE -ne 0) { Halte 'Le push a echoue — le message de git ci-dessus dit pourquoi.' }

Write-Host ''
Write-Host '  =========================================' -ForegroundColor Green
Write-Host '   Envoye. GitHub compile (environ 2 min).' -ForegroundColor Green
Write-Host '  =========================================' -ForegroundColor Green
Write-Host ''
Write-Host '  Suivre : https://github.com/guillaume999/SysB-Server/actions'
Write-Host '  Image  : ghcr.io/guillaume999/sysb-pocketbase:latest'
Write-Host ''
Alerte '⚠️ LA PREMIERE FOIS SEULEMENT : l image sort PRIVEE et Portainer ne'
Alerte '   pourra pas la tirer. Une bonne fois pour toutes :'
Alerte '   github.com/guillaume999?tab=packages -> sysb-pocketbase'
Alerte '   -> Package settings -> Change visibility -> Public'
Write-Host ''
Read-Host 'Entree pour fermer'
