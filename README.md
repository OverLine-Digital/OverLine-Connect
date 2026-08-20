# OverLine Connect

Base technique du produit OverLine Connect (OverLine Digital), issue de la
restructuration de WaSender-API.

## Structure

```
overline-connect/
├── cmd/server/             point d'entrée — assemble config + manager + services + serveur HTTP
├── internal/
│   ├── whatsapp/            interface Provider (contrat WhatsApp) + Manager (une session par entreprise)
│   │   └── meow/             implémentation whatsmeow (dev/test)
│   ├── auth/                inscription, connexion, clés API
│   │   ├── sqlite/           implémentation active (temporaire)
│   │   └── postgres/         implémentation prête, pas encore activable (voir plus bas)
│   ├── crm/                  historique des conversations (Inbox)
│   │   └── sqlite/           implémentation du stockage
│   ├── webui/                dashboard web statique (embarqué dans le binaire)
│   ├── httpapi/               routes HTTP + middleware
│   └── config/                configuration via variables d'environnement
├── secret/
│   ├── auth.db               base entreprises/clés API
│   ├── crm.db                conversations et messages
│   └── whatsapp/              un fichier de session whatsmeow par entreprise (<company_id>.db)
└── temp/                      fichiers temporaires
```

## Vue d'ensemble de ce qui existe

1. **Auth** — une entreprise s'inscrit (`/auth/signup`), reçoit une clé API,
   l'utilise pour toutes les requêtes suivantes (`Authorization: Bearer olc_...`).
2. **WhatsApp multi-tenant** — chaque entreprise connecte sa propre session
   WhatsApp (`/whatsapp/connect` → QR code via `/whatsapp/status`), isolée
   des autres entreprises.
3. **CRM / Inbox** — tout message reçu ou envoyé est automatiquement
   enregistré et consultable via `/conversations` et
   `/conversations/:chatJID/messages`.
4. **Dashboard web** — une page unique (`/`) pour tester tout ça sans curl :
   connexion par clé API, QR code affiché à l'écran, inbox, envoi de message.

## Dashboard web

Servi directement à la racine (`GET /`), embarqué dans le binaire (pas de
fichier séparé à déployer). Fonctionnalités :
- Coller sa clé API (stockée dans le navigateur, jamais envoyée ailleurs
  qu'à ton propre serveur)
- Bouton "Connecter WhatsApp" → affiche le QR code (rendu via la librairie
  `qrcode.js`, chargée depuis un CDN — nécessite que le navigateur ait accès
  à internet, le serveur backend lui n'a besoin d'aucun accès réseau
  supplémentaire)
- Liste des conversations, rafraîchie automatiquement (sondage toutes les
  5 secondes)
- Vue d'une conversation + champ d'envoi de message

C'est un outil de test/administration minimal, pas encore l'interface
commerciale complète décrite dans le plan produit (pas de CRM pipeline, pas
de vue multi-agents, etc.).

## Limites connues (honnêtes, à lire avant de déployer)

- **SQLite partout** (auth, crm) au lieu de PostgreSQL. Cet environnement de
  développement n'a pas d'accès réseau pour ajouter un driver PostgreSQL.
  `internal/auth/postgres` est déjà écrit et prêt, mais **ne compile pas
  tel quel** tant que tu n'as pas toi-même lancé :
  ```
  go get github.com/jackc/pgx/v5/stdlib
  ```
  Ensuite, dans `cmd/server/main.go`, remplacer l'import et l'appel
  `authsqlite.New(cfg.AuthDBPath)` par `postgres.New(cfg.AuthDBURL)`, et
  définir `AUTH_DB_URL`. Le CRM (`internal/crm`) n'a pas encore
  d'implémentation PostgreSQL préparée — à écrire sur le même modèle si tu
  veux migrer aussi cette partie.
- **État des sessions WhatsApp en mémoire** : un redémarrage du serveur
  oblige chaque entreprise à rappeler `/whatsapp/connect` (la session
  elle-même reste valide sur disque et se reconnecte vite, sans re-scan).
- **Compilation non vérifiée ici** : cet environnement n'a pas de
  toolchain Go installée. Le code a été relu ligne par ligne pour la
  cohérence (imports, signatures, accolades), mais **lance `go build
  ./cmd/server` toi-même avant tout déploiement**.
- **Messages non-texte** (image/document reçus) ne sont pas encore
  capturés par le CRM — seul le texte est extrait et stocké pour l'instant.
- **Rôles/permissions** : une seule clé API par entreprise, pas de
  distinction d'utilisateurs au sein d'une même entreprise.
- Aucun module IA, commerce, paiement, automatisation — pas encore abordés.

## Lancer le projet

```bash
go build ./cmd/server
./server
# ou directement :
go run ./cmd/server
```

Puis ouvrir `http://localhost:8080` dans un navigateur pour utiliser le
dashboard, ou suivre le flow API ci-dessous.

### 1. Créer une entreprise et obtenir une clé API

```bash
curl -X POST http://localhost:8080/auth/signup \
  -H "Content-Type: application/json" \
  -d '{"name":"Ma Boutique","email":"contact@maboutique.com","password":"motdepasse123"}'
```

### 2. Démarrer la connexion WhatsApp

```bash
curl -X POST http://localhost:8080/whatsapp/connect \
  -H "Authorization: Bearer olc_..."
```

### 3. Récupérer le QR code et scanner

```bash
curl http://localhost:8080/whatsapp/status \
  -H "Authorization: Bearer olc_..."
# {"status":"awaiting_scan","qr_code":"2@AbCdEf..."}
```

(Le dashboard web fait ce rendu visuellement — plus simple que de générer
l'image soi-même à partir de la chaîne brute.)

### 4. Envoyer un message

```bash
curl -X POST http://localhost:8080/sendtext \
  -H "Authorization: Bearer olc_..." \
  -H "Content-Type: application/json" \
  -d '{"to":"243900000000","text":"Bonjour !"}'
```

### 5. Consulter l'historique

```bash
curl http://localhost:8080/conversations \
  -H "Authorization: Bearer olc_..."

curl "http://localhost:8080/conversations/243900000000@s.whatsapp.net/messages" \
  -H "Authorization: Bearer olc_..."
```

## Variables d'environnement

| Variable                | Défaut                | Description                                    |
|--------------------------|------------------------|-------------------------------------------------|
| `PORT`                  | `8080`                 | Port d'écoute HTTP                              |
| `WHATSAPP_SESSIONS_DIR` | `secret/whatsapp`      | Dossier des sessions whatsmeow (1/entreprise)   |
| `AUTH_DB_PATH`          | `secret/auth.db`       | Fichier SQLite entreprises/clés API             |
| `AUTH_DB_URL`           | (vide)                 | URL PostgreSQL — utilisée seulement si tu actives `internal/auth/postgres` |
| `CRM_DB_PATH`           | `secret/crm.db`        | Fichier SQLite conversations/messages           |

## Prochaine étape

À définir ensemble : pipeline CRM (prospect → qualifié → commande), premier
module IA (réponse basée sur une base de connaissances), ou migration
PostgreSQL complète (auth + crm) une fois que tu as un accès réseau pour
`go get`.
