# API.md — Référence complète de l'API

Documentation de **tous** les points d'entrée (endpoints) de l'API Bibliothèque : méthode, URL,  
authentification et rôle requis, paramètres, corps attendu et retourné, codes HTTP et erreurs  
possibles, avec un **exemple `curl` complet** pour chacun.

- **URL de base** : `http://localhost:8080`
- **Préfixe des ressources** : `/api/v1`
- **Format** : JSON, UTF-8

## Table des matières

- [Conventions générales](#conventions-générales)
  - [Enveloppe de réponse](#enveloppe-de-réponse)
  - [Authentification](#authentification)
  - [Rôles et autorisations](#rôles-et-autorisations)
  - [Pagination, tri, recherche et filtrage](#pagination-tri-recherche-et-filtrage)
  - [Codes d'erreur](#codes-derreur)
- [Authentification](#ressource--authentification)
- [Profil personnel (« moi »)](#ressource--profil-personnel--moi-)
- [Utilisateurs](#ressource--utilisateurs)
- [Auteurs](#ressource--auteurs)
- [Catégories](#ressource--catégories)
- [Livres](#ressource--livres)
- [Emprunts](#ressource--emprunts)
- [Observabilité](#ressource--observabilité)

---

## Conventions générales

### Enveloppe de réponse

**Toutes** les réponses partagent la même structure. Le client teste d'abord `succes`, puis lit
`donnees` (ou `erreur`).

**Succès :**

```json
{
  "succes": true,
  "donnees": { "...": "..." },
  "meta": { "page": 1, "taille_par_page": 20, "total_elements": 28, "total_pages": 2 }
}
```

`meta` n'est présent que pour les **listes paginées**. Un `DELETE` réussi renvoie **`204 No
Content`** (aucun corps).

**Erreur :**

```json
{
  "succes": false,
  "erreur": {
    "code": "VALIDATION",
    "message": "Un ou plusieurs champs sont invalides.",
    "details": { "email": "adresse e-mail invalide" }
  }
}
```

`details` (objet champ → explication) n'apparaît que pour les erreurs de **validation** (422).

> **Sécurité.** Aucune erreur ne divulgue de détail technique (SQL, chemin…). Les erreurs `5xx`  
> sont journalisées côté serveur avec leur cause complète et un `identifiant_requete`, mais le  
> client ne reçoit qu'un message générique.

### Authentification

Les routes protégées attendent le **jeton d'accès JWT** dans l'en-tête :

```
Authorization: Bearer <jeton_acces>
```

- Le **jeton d'accès** est à **courte durée** (15 min par défaut). S'il expire, on obtient un
  nouveau couple via `POST /api/v1/auth/rafraichir` (sans se ré-identifier).
- Le **refresh token** est à **longue durée** (7 jours) et subit une **rotation** : à chaque
  rafraîchissement, l'ancien est révoqué et remplacé.

Récupérer et mémoriser un jeton (exemples ci-dessous) :

```bash
JETON=$(curl -s -X POST http://localhost:8080/api/v1/auth/connexion \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@bibliotheque.fr","mot_de_passe":"MotDePasse123!"}' \
  | jq -r '.donnees.jetons.jeton_acces')
```

### Rôles et autorisations

Trois rôles : `admin`, `bibliothecaire`, `membre`.

| Domaine                                   | Public | `membre` | `bibliothecaire` | `admin` |
|-------------------------------------------|:------:|:--------:|:----------------:|:-------:|
| Consulter le catalogue (livres/auteurs/catégories) |   ✅   |    ✅    |        ✅        |   ✅    |
| Profil personnel (`/moi`)                 |        |    ✅    |        ✅        |   ✅    |
| Emprunter / rendre (pour soi)             |        |    ✅    |        ✅        |   ✅    |
| Emprunter **pour un autre** membre        |        |          |        ✅        |   ✅    |
| Écrire dans le catalogue (créer/modifier/supprimer) |     |          |        ✅        |   ✅    |
| Lister/consulter **tous** les emprunts    |        |          |        ✅        |   ✅    |
| Gérer les utilisateurs                    |        |          |                  |   ✅    |
| Suppression **définitive** (`?definitif=true`) |    |          |                  |   ✅    |

### Pagination, tri, recherche et filtrage

Les endpoints de liste acceptent :

| Paramètre   | Défaut | Description                                                            |
|-------------|:------:|-----------------------------------------------------------------------|
| `page`      | `1`    | Numéro de page (≥ 1)                                                   |
| `taille`    | `20`   | Éléments par page (**max 100** ; au-delà, ramené à 100)               |
| `tri`       | *(par ressource)* | Champ de tri, **validé par liste blanche** (anti-injection)  |
| `ordre`     | `asc`  | `asc` ou `desc`                                                        |
| `recherche` | —      | Terme de recherche simple (`LIKE`)                                     |

Les **champs de tri autorisés** et les **filtres** dépendent de la ressource (précisés dans chaque  
section). Un `tri` inconnu est **ignoré** (repli sur le tri par défaut), jamais une erreur.

### Codes d'erreur

| `code` métier          | Statut HTTP | Signification                                                   |
|------------------------|:-----------:|----------------------------------------------------------------|
| `REQUETE_INVALIDE`     | `400`       | JSON illisible, champ de type incorrect, identifiant mal formé |
| `NON_AUTHENTIFIE`      | `401`       | Jeton absent, invalide ou expiré ; identifiants erronés        |
| `INTERDIT`             | `403`       | Authentifié mais rôle insuffisant ; compte désactivé           |
| `NON_TROUVE`           | `404`       | Ressource inexistante                                          |
| `CONFLIT`              | `409`       | Doublon (UNIQUE), règle métier bloquante (stock, quota, FK)    |
| `REQUETE_INVALIDE`     | `413`       | Corps de requête trop volumineux                              |
| `VALIDATION`           | `422`       | Requête bien formée mais champs invalides (`details` fourni)   |
| `TROP_DE_REQUETES`     | `429`       | Limite de débit atteinte (en-tête `Retry-After`)              |
| `ERREUR_INTERNE`       | `500`       | Erreur serveur inattendue (cause journalisée, non exposée)    |
| `SERVICE_INDISPONIBLE` | `503`       | Dépendance en panne (ex. base injoignable, via `/ready`)      |

---

## Ressource — Authentification

Routes **publiques** sous `/api/v1/auth`.

### POST `/api/v1/auth/inscription`

Crée un compte **membre** (le rôle est forcé à `membre` — protection anti Mass-Assignment).

- **Auth** : aucune.
- **Corps** :

  | Champ         | Type   | Règles                                  |
  |---------------|--------|-----------------------------------------|
  | `email`       | string | requis, e-mail valide, ≤ 254 caractères |
  | `mot_de_passe`| string | requis, 8 à 72 caractères               |
  | `nom`         | string | requis, ≤ 100 caractères                |
  | `prenom`      | string | requis, ≤ 100 caractères                |

- **Réponse** : `201 Created` — l'objet `Utilisateur`.
- **Erreurs** : `400` (JSON invalide / champ non autorisé), `422` (validation), `409` (e-mail déjà
  utilisé).

```bash
curl -s -X POST http://localhost:8080/api/v1/auth/inscription \
  -H "Content-Type: application/json" \
  -d '{
        "email": "nouveau.membre@exemple.fr",
        "mot_de_passe": "MotDePasse123!",
        "nom": "Nouveau",
        "prenom": "Membre"
      }' | jq
```

```json
{
  "succes": true,
  "donnees": {
    "id": "5f1c…-uuid",
    "email": "nouveau.membre@exemple.fr",
    "nom": "Nouveau",
    "prenom": "Membre",
    "role": "membre",
    "actif": true,
    "cree_le": "2026-07-06T10:00:00Z",
    "modifie_le": "2026-07-06T10:00:00Z"
  }
}
```

### POST `/api/v1/auth/connexion`

Vérifie les identifiants et renvoie le profil **et** une paire de jetons.

- **Auth** : aucune.
- **Corps** : `{ "email": string, "mot_de_passe": string }` (tous deux requis).
- **Réponse** : `200 OK` — `{ "utilisateur": Utilisateur, "jetons": PaireDeJetons }`.
- **Erreurs** : `400`, `422`, `401` (identifiants invalides — **message identique** que l'e-mail
  soit inconnu ou le mot de passe erroné, pour ne pas révéler l'existence d'un compte), `403`
  (compte désactivé).

```bash
curl -s -X POST http://localhost:8080/api/v1/auth/connexion \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@bibliotheque.fr","mot_de_passe":"MotDePasse123!"}' | jq
```

```json
{
  "succes": true,
  "donnees": {
    "utilisateur": { "id": "…", "email": "admin@bibliotheque.fr", "role": "admin", "actif": true, "…": "…" },
    "jetons": {
      "jeton_acces": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9…",
      "jeton_rafraichissement": "R2c3…opaque",
      "type_jeton": "Bearer",
      "expire_le": "2026-07-06T10:15:00Z"
    }
  }
}
```

### POST `/api/v1/auth/rafraichir`

Échange un refresh token valide contre une **nouvelle** paire de jetons (rotation : l'ancien est
révoqué).

- **Auth** : aucune (le refresh token fait foi).
- **Corps** : `{ "jeton_rafraichissement": string }`.
- **Réponse** : `200 OK` — `PaireDeJetons`.
- **Erreurs** : `400` (jeton manquant), `401` (invalide / expiré / révoqué), `403` (compte
  désactivé).

```bash
curl -s -X POST http://localhost:8080/api/v1/auth/rafraichir \
  -H "Content-Type: application/json" \
  -d '{"jeton_rafraichissement":"R2c3…opaque"}' | jq
```

### POST `/api/v1/auth/deconnexion`

Révoque le refresh token fourni. Le jeton d'accès reste techniquement valide jusqu'à sa courte  
expiration (limite connue des JWT sans liste de révocation, acceptable vu la durée réduite).

- **Auth** : aucune.
- **Corps** : `{ "jeton_rafraichissement": string }`.
- **Réponse** : `204 No Content`.
- **Erreurs** : `400` (jeton manquant).

```bash
curl -s -o /dev/null -w "%{http_code}\n" -X POST http://localhost:8080/api/v1/auth/deconnexion \
  -H "Content-Type: application/json" \
  -d '{"jeton_rafraichissement":"R2c3…opaque"}'
# 204
```

---

## Ressource — Profil personnel (« moi »)

Routes **authentifiées**, tout rôle. L'identité provient du jeton (jamais du corps ou de l'URL).

### GET `/api/v1/moi`

Renvoie le profil de l'utilisateur connecté.

- **Auth** : Bearer (tout rôle). **Réponse** : `200 OK` — `Utilisateur`. **Erreurs** : `401`.

```bash
curl -s -H "Authorization: Bearer $JETON" http://localhost:8080/api/v1/moi | jq
```

### PATCH `/api/v1/moi`

Modifie **son propre** `nom` / `prenom`. Les champs `role` et `actif` sont **ignorés** (un membre  
ne peut pas s'auto-promouvoir).

- **Auth** : Bearer (tout rôle).
- **Corps** (partiel) : `{ "nom"?: string, "prenom"?: string }`.
- **Réponse** : `200 OK` — `Utilisateur`. **Erreurs** : `400`, `401`, `422`.

```bash
curl -s -X PATCH http://localhost:8080/api/v1/moi \
  -H "Authorization: Bearer $JETON" -H "Content-Type: application/json" \
  -d '{"prenom":"Chloé-Anne"}' | jq
```

### GET `/api/v1/moi/emprunts`

Liste **mes** emprunts (paginée).

- **Auth** : Bearer (tout rôle).
- **Filtre** : `statut` (`en_cours` | `rendu` | `en_retard`). **Tri** : `date_emprunt` (défaut),
  `date_retour`, `statut`.
- **Réponse** : `200 OK` — liste d'`Emprunt` + `meta`.

```bash
curl -s -H "Authorization: Bearer $JETON" \
  "http://localhost:8080/api/v1/moi/emprunts?statut=en_cours&tri=date_retour&ordre=asc" | jq
```

### GET `/api/v1/moi/statistiques`

Indicateurs d'emprunt de l'utilisateur connecté (via la procédure `pr_statistiques_utilisateur`).

- **Auth** : Bearer (tout rôle). **Réponse** : `200 OK`.

```bash
curl -s -H "Authorization: Bearer $JETON" http://localhost:8080/api/v1/moi/statistiques | jq
```

```json
{
  "succes": true,
  "donnees": { "nb_total": 5, "nb_en_cours": 2, "nb_en_retard": 1, "total_penalites": 8.0 }
}
```

---

## Ressource — Utilisateurs

Routes **réservées à l'administrateur** (`admin`), sous `/api/v1/utilisateurs`.

- **Tri autorisé** : `nom`, `email`, `role`, `date` (→ `cree_le`, défaut).
- **Filtre** : `role`. **Recherche** : sur `nom`, `prenom`, `email`.

| Méthode & URL                          | Corps                                   | Succès | Erreurs notables                   |
|----------------------------------------|-----------------------------------------|:------:|------------------------------------|
| `GET /api/v1/utilisateurs`             | —                                       | `200`  | `401`, `403`                       |
| `POST /api/v1/utilisateurs`            | `CreerUtilisateurEntree`                | `201`  | `400`, `401`, `403`, `422`, `409`  |
| `GET /api/v1/utilisateurs/{id}`        | —                                       | `200`  | `400`, `401`, `403`, `404`         |
| `PUT /api/v1/utilisateurs/{id}`        | `{ "nom", "prenom" }`                    | `200`  | `400`, `401`, `403`, `404`, `422`  |
| `PATCH /api/v1/utilisateurs/{id}`      | `{ "nom"?, "prenom"?, "role"?, "actif"? }` | `200` | `400`, `401`, `403`, `404`, `422`  |
| `DELETE /api/v1/utilisateurs/{id}`     | — (`?definitif=true` pour physique)     | `204`  | `401`, `403`, `404`                |

`CreerUtilisateurEntree` : `{ "email", "mot_de_passe", "nom", "prenom", "role" }` où `role` ∈
{`admin`, `bibliothecaire`, `membre`}. Contrairement à l'inscription publique, l'admin fixe le rôle.

En `PATCH`, les champs `role` et `actif` ne sont appliqués que par un admin (garanti par la route).
`DELETE` fait une suppression **logique** (compte désactivé et masqué) ; avec `?definitif=true`,
suppression **physique** (les emprunts liés partent en cascade).

```bash
# Créer un bibliothécaire (admin requis)
curl -s -X POST http://localhost:8080/api/v1/utilisateurs \
  -H "Authorization: Bearer $JETON_ADMIN" -H "Content-Type: application/json" \
  -d '{
        "email": "biblio2@bibliotheque.fr",
        "mot_de_passe": "MotDePasse123!",
        "nom": "Nour", "prenom": "Sami",
        "role": "bibliothecaire"
      }' | jq

# Lister les membres, triés par nom
curl -s -H "Authorization: Bearer $JETON_ADMIN" \
  "http://localhost:8080/api/v1/utilisateurs?role=membre&tri=nom&taille=50" | jq

# Désactiver un compte (PATCH, admin)
curl -s -X PATCH http://localhost:8080/api/v1/utilisateurs/<UUID> \
  -H "Authorization: Bearer $JETON_ADMIN" -H "Content-Type: application/json" \
  -d '{"actif": false}' | jq

# Suppression définitive
curl -s -o /dev/null -w "%{http_code}\n" -X DELETE \
  "http://localhost:8080/api/v1/utilisateurs/<UUID>?definitif=true" \
  -H "Authorization: Bearer $JETON_ADMIN"
```

---

## Ressource — Auteurs

**Lecture publique** ; **écriture** réservée à `bibliothecaire`/`admin`. Sous `/api/v1/auteurs`.

- **Tri autorisé** : `nom` (défaut), `date_naissance`, `date` (→ `cree_le`).
- **Filtre** : `nationalite`. **Recherche** : sur `nom`, `prenom`.

| Méthode & URL                  | Auth              | Corps                    | Succès | Erreurs notables                  |
|--------------------------------|-------------------|--------------------------|:------:|-----------------------------------|
| `GET /api/v1/auteurs`          | Public            | —                        | `200`  | —                                 |
| `GET /api/v1/auteurs/{id}`     | Public            | —                        | `200`  | `400`, `404`                      |
| `POST /api/v1/auteurs`         | biblio/admin      | `CreerAuteurEntree`      | `201`  | `400`, `401`, `403`, `422`        |
| `PUT /api/v1/auteurs/{id}`     | biblio/admin      | `MettreAJourAuteurEntree`| `200`  | `400`, `401`, `403`, `404`, `422` |
| `PATCH /api/v1/auteurs/{id}`   | biblio/admin      | `ModifierAuteurEntree`   | `200`  | `400`, `401`, `403`, `404`, `422` |
| `DELETE /api/v1/auteurs/{id}`  | biblio/admin      | —                        | `204`  | `401`, `403`, `404`, `409`        |

Champs d'`Auteur` (entrée) : `nom` (requis à la création), `prenom`, `nationalite`,
`date_naissance` (facultatif, format `AAAA-MM-JJ`, validé), `biographie`. La **suppression** est
**physique** ; elle renvoie **`409`** si des livres référencent l'auteur (FK `ON DELETE RESTRICT`).

```bash
# Créer un auteur (biblio/admin)
curl -s -X POST http://localhost:8080/api/v1/auteurs \
  -H "Authorization: Bearer $JETON" -H "Content-Type: application/json" \
  -d '{
        "nom": "Zola", "prenom": "Émile",
        "nationalite": "française", "date_naissance": "1840-04-02",
        "biographie": "Chef de file du naturalisme."
      }' | jq

# Rechercher des auteurs français nommés « hugo »
curl -s "http://localhost:8080/api/v1/auteurs?recherche=hugo&nationalite=française" | jq
```

---

## Ressource — Catégories

**Lecture publique** ; **écriture** réservée à `bibliothecaire`/`admin`. Sous `/api/v1/categories`.

- **Tri autorisé** : `nom` (défaut), `date` (→ `cree_le`). **Recherche** : sur `nom`. Aucun filtre.

| Méthode & URL                    | Auth         | Corps                       | Succès | Erreurs notables                  |
|----------------------------------|--------------|-----------------------------|:------:|-----------------------------------|
| `GET /api/v1/categories`         | Public       | —                           | `200`  | —                                 |
| `GET /api/v1/categories/{id}`    | Public       | —                           | `200`  | `400`, `404`                      |
| `POST /api/v1/categories`        | biblio/admin | `{ "nom", "description" }`  | `201`  | `400`, `401`, `403`, `422`, `409` |
| `PUT /api/v1/categories/{id}`    | biblio/admin | `{ "nom", "description" }`  | `200`  | `400`, `401`, `403`, `404`, `409` |
| `PATCH /api/v1/categories/{id}`  | biblio/admin | `{ "nom"?, "description"? }` | `200` | `400`, `401`, `403`, `404`, `409` |
| `DELETE /api/v1/categories/{id}` | biblio/admin | —                           | `204`  | `401`, `403`, `404`, `409`        |

Le **nom** est **unique** : un doublon renvoie `409`. La **suppression** renvoie `409` si des  
livres appartiennent à la catégorie.

```bash
curl -s -X POST http://localhost:8080/api/v1/categories \
  -H "Authorization: Bearer $JETON" -H "Content-Type: application/json" \
  -d '{"nom":"Théâtre","description":"Pièces et textes dramatiques"}' | jq
```

---

## Ressource — Livres

**Lecture publique** ; **écriture** réservée à `bibliothecaire`/`admin`. Sous `/api/v1/livres`.

- **Tri autorisé** : `titre` (défaut), `annee` (→ `annee_publication`), `prix`, `date` (→
  `cree_le`).
- **Filtres** : `categorie` (UUID de catégorie), `auteur` (UUID d'auteur), `disponible=true`.
  **Recherche** : sur `titre`.

| Méthode & URL                | Auth              | Corps                    | Succès | Erreurs notables                        |
|------------------------------|-------------------|--------------------------|:------:|-----------------------------------------|
| `GET /api/v1/livres`         | Public            | —                        | `200`  | —                                       |
| `GET /api/v1/livres/{id}`    | Public            | —                        | `200`  | `400`, `404`                            |
| `POST /api/v1/livres`        | biblio/admin      | `CreerLivreEntree`       | `201`  | `400`, `401`, `403`, `422`, `409`       |
| `PUT /api/v1/livres/{id}`    | biblio/admin      | `MettreAJourLivreEntree` | `200`  | `400`, `401`, `403`, `404`, `422`, `409`|
| `PATCH /api/v1/livres/{id}`  | biblio/admin      | `ModifierLivreEntree`    | `200`  | `400`, `401`, `403`, `404`, `422`, `409`|
| `DELETE /api/v1/livres/{id}` | biblio/admin      | — (`?definitif=true` : admin) | `204` | `401`, `403`, `404`, `409`          |

`CreerLivreEntree` :

| Champ                | Type    | Règles                                                        |
|----------------------|---------|---------------------------------------------------------------|
| `titre`              | string  | requis, ≤ 255                                                 |
| `isbn`               | string  | requis, **ISBN-13 valide** (clé de contrôle vérifiée)         |
| `auteur_id`          | string  | requis, **UUID** d'un auteur existant                        |
| `categorie_id`       | string  | requis, **UUID** d'une catégorie existante                   |
| `annee_publication`  | int     | 1400 à 2200                                                   |
| `nombre_exemplaires` | int     | 1 à 100 000                                                   |
| `resume`             | string  | facultatif                                                    |
| `prix`               | number  | ≥ 0                                                           |
| `langue`             | string  | facultatif, ≤ 50 (défaut « français »)                       |

À la création, `exemplaires_disponibles` = `nombre_exemplaires`. Un `auteur_id` / `categorie_id`
inexistant renvoie **`422`** (référence invalide). Un `isbn` en doublon renvoie **`409`**. En  
mise à jour, réduire `nombre_exemplaires` sous le nombre d'exemplaires **actuellement empruntés**  
renvoie **`409`**. La **lecture** renvoie l'objet enrichi (nom d'auteur, nom de catégorie,
`disponible`) issu de la vue `vue_livres_details`.

`DELETE` fait une suppression **logique** (masquée du catalogue, historique préservé) ; avec
`?definitif=true` (**admin uniquement**), suppression **physique**, qui renvoie `409` si des
emprunts référencent le livre.

```bash
# Créer un livre (récupérez d'abord des UUID d'auteur et de catégorie)
AUTEUR=$(curl -s "http://localhost:8080/api/v1/auteurs?recherche=Camus" | jq -r '.donnees[0].id')
CATEG=$(curl -s "http://localhost:8080/api/v1/categories?recherche=Roman" | jq -r '.donnees[0].id')

curl -s -X POST http://localhost:8080/api/v1/livres \
  -H "Authorization: Bearer $JETON" -H "Content-Type: application/json" \
  -d "{
        \"titre\": \"La Chute\",
        \"isbn\": \"9782070360024\",
        \"auteur_id\": \"$AUTEUR\",
        \"categorie_id\": \"$CATEG\",
        \"annee_publication\": 1956,
        \"nombre_exemplaires\": 3,
        \"prix\": 7.40,
        \"resume\": \"Un monologue à Amsterdam.\"
      }" | jq

# Consulter un livre
curl -s http://localhost:8080/api/v1/livres/<UUID> | jq

# Ajuster le stock (PATCH partiel)
curl -s -X PATCH http://localhost:8080/api/v1/livres/<UUID> \
  -H "Authorization: Bearer $JETON" -H "Content-Type: application/json" \
  -d '{"nombre_exemplaires": 5}' | jq
```

Exemple de réponse (`GET /api/v1/livres/{id}`) :

```json
{
  "succes": true,
  "donnees": {
    "id": "…-uuid",
    "titre": "L'Étranger",
    "isbn": "9782010000096",
    "annee_publication": 1942,
    "nombre_exemplaires": 5,
    "exemplaires_disponibles": 4,
    "prix": 7.9,
    "langue": "français",
    "resume": "Meursault face à l'absurde.",
    "auteur_id": "…-uuid",
    "auteur": "Albert Camus",
    "categorie_id": "…-uuid",
    "categorie": "Roman",
    "disponible": true,
    "cree_le": "2026-07-06T09:00:00Z",
    "modifie_le": "2026-07-06T09:00:00Z"
  }
}
```

---

## Ressource — Emprunts

Sous `/api/v1/emprunts`. Emprunter et rendre sont ouverts à **tout utilisateur authentifié** (pour  
lui-même) ; la **consultation globale** est réservée à `bibliothecaire`/`admin`.

- **Tri autorisé** (listes) : `date_emprunt` (défaut), `date_retour` (→ `date_retour_prevue`),
  `statut`. **Filtre** : `statut`.

### POST `/api/v1/emprunts` — emprunter

- **Auth** : Bearer (tout rôle).
- **Corps** :

  | Champ           | Type   | Règles                                                                 |
  |-----------------|--------|------------------------------------------------------------------------|
  | `livre_id`      | string | requis, **UUID** du livre                                              |
  | `duree_jours`   | int    | facultatif, 1 à 90 (défaut **14**)                                     |
  | `utilisateur_id`| string | facultatif — **réservé à biblio/admin** pour emprunter au nom d'un membre |

  Un **membre** ne peut emprunter que pour lui-même : s'il renseigne un `utilisateur_id` différent
  du sien, il reçoit **`403`**. Le paramètre est ignoré pour un membre qui met son propre UUID.

- **Réponse** : `201 Created` — l'`Emprunt` créé (enrichi du nom de l'emprunteur et du titre).
- **Erreurs** : `400`/`422` (entrée), `401`, `403` (membre empruntant pour autrui), `404` (livre ou
  utilisateur introuvable/inactif), `409` (aucun exemplaire disponible **ou** quota de **5**
  emprunts simultanés atteint).

```bash
LIVRE=$(curl -s "http://localhost:8080/api/v1/livres?disponible=true" | jq -r '.donnees[0].id')

curl -s -X POST http://localhost:8080/api/v1/emprunts \
  -H "Authorization: Bearer $JETON" -H "Content-Type: application/json" \
  -d "{\"livre_id\":\"$LIVRE\",\"duree_jours\":21}" | jq
```

### POST `/api/v1/emprunts/{id}/retour` — rendre

Enregistre le retour et renvoie l'emprunt clôturé (statut `rendu`, `penalite` éventuelle calculée
à 0,50 €/jour de retard).

- **Auth** : Bearer (tout rôle). **Réponse** : `200 OK` — `Emprunt`.
- **Erreurs** : `400` (UUID invalide), `401`, `404` (emprunt introuvable), `409` (déjà rendu).

```bash
curl -s -X POST http://localhost:8080/api/v1/emprunts/<UUID>/retour \
  -H "Authorization: Bearer $JETON" | jq
```

### GET `/api/v1/emprunts` — lister (biblio/admin)

Liste **tous** les emprunts, avec filtre `statut` et pagination.

```bash
curl -s -H "Authorization: Bearer $JETON_BIBLIO" \
  "http://localhost:8080/api/v1/emprunts?statut=en_retard&tri=date_retour&ordre=asc" | jq
```

### GET `/api/v1/emprunts/{id}` — consulter (biblio/admin)

- **Auth** : Bearer (`bibliothecaire`/`admin`). **Réponse** : `200 OK` — `Emprunt`. **Erreurs** :
  `400`, `401`, `403`, `404`.

Exemple d'`Emprunt` :

```json
{
  "succes": true,
  "donnees": {
    "id": "…-uuid",
    "date_emprunt": "2026-06-20T00:00:00Z",
    "date_retour_prevue": "2026-07-04T00:00:00Z",
    "date_retour_effective": "2026-07-06T00:00:00Z",
    "statut": "rendu",
    "penalite": 1.0,
    "utilisateur_id": "…-uuid",
    "utilisateur": "Chloé Durand",
    "livre_id": "…-uuid",
    "livre": "Les Misérables",
    "cree_le": "2026-06-20T08:00:00Z",
    "modifie_le": "2026-07-06T08:00:00Z"
  }
}
```

---

## Ressource — Observabilité

Routes **publiques**, hors préfixe `/api/v1`.

### GET `/health` — liveness

Répond `200` tant que le processus est vivant ; ne dépend d'aucune ressource externe.

```bash
curl -s http://localhost:8080/health | jq
# {"succes":true,"donnees":{"statut":"ok","version":"1.0.0","duree_fonctionnement":"1h2m3s"}}
```

### GET `/ready` — readiness

Vérifie que PostgreSQL répond (`ping` borné à 2 s).

- **Succès** : `200` — `{ "statut": "pret", "base_de_donnees": "ok" }`.
- **Échec** : `503` — code `SERVICE_INDISPONIBLE` (base injoignable).

```bash
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8080/ready
```

### GET `/metrics` — métriques Prometheus

Renvoie les métriques au **format texte Prometheus** (pas l'enveloppe JSON). Principales séries :

| Métrique                                        | Type       | Description                                   |
|-------------------------------------------------|------------|-----------------------------------------------|
| `bibliotheque_http_requetes_total`              | Counter    | Requêtes par `methode`, `route`, `statut`     |
| `bibliotheque_http_duree_requete_secondes`      | Histogram  | Latence par `methode`, `route`                |
| `bibliotheque_http_requetes_en_cours`           | Gauge      | Requêtes en cours de traitement               |
| *(+ métriques standard Go et processus)*        |            | GC, goroutines, CPU, mémoire…                 |

```bash
curl -s http://localhost:8080/metrics | grep '^bibliotheque_'
```

> La `route` utilisée en label est le **patron** (`GET /api/v1/livres/{id}`), pas le chemin réel,  
> afin d'éviter l'explosion de cardinalité (une série par identifiant).
