# SECURITE.md — Menaces et protections, expliquées

Ce document décrit, **menace par menace**, les risques de sécurité d'une API web et **la parade  
concrète** mise en place dans ce projet, avec le pointeur vers le code correspondant. L'objectif  
est double : comprendre **pourquoi** chaque protection existe, et **comment** elle est implémentée  
ici.

Principe directeur : la **défense en profondeur**. On ne se repose jamais sur une seule barrière ;  
on empile plusieurs protections indépendantes, à des niveaux différents (application, base,  
conteneur), pour qu'une faille dans l'une soit rattrapée par une autre.

## Table des matières

- [1. Injection SQL](#1-injection-sql)
- [2. XSS (Cross-Site Scripting)](#2-xss-cross-site-scripting)
- [3. CSRF (Cross-Site Request Forgery)](#3-csrf-cross-site-request-forgery)
- [4. Mass Assignment](#4-mass-assignment)
- [5. Force brute (Brute Force)](#5-force-brute-brute-force)
- [6. Slowloris (clients lents)](#6-slowloris-clients-lents)
- [7. DoS / DDoS](#7-dos--ddos)
- [8. En-têtes de sécurité HTTP](#8-en-têtes-de-sécurité-http)
- [9. Gestion des secrets](#9-gestion-des-secrets)
- [10. Conteneurs non-root et surface minimale](#10-conteneurs-non-root-et-surface-minimale)
- [11. Journalisation sécurisée](#11-journalisation-sécurisée)
- [12. Autres protections transverses](#12-autres-protections-transverses)
- [Récapitulatif](#récapitulatif)

---

## 1. Injection SQL

**La menace.** L'injection SQL consiste à insérer du code SQL malveillant via une entrée
utilisateur. Si l'on construit une requête par **concaténation de chaînes** —
`"… WHERE email = '" + saisie + "'"` — un attaquant qui saisit `' OR '1'='1` modifie la logique de
la requête et peut lire, altérer ou détruire des données. C'est la vulnérabilité n°1 historique des  
applications web.

**La parade dans ce projet : requêtes préparées paramétrées, partout.**

Chaque valeur passe par un **paramètre positionnel numéroté** — la syntaxe PostgreSQL `$1, $2, …`
(des repères **numérotés**, jamais anonymes) — envoyé **séparément** de la requête au serveur, qui
la traite comme une **donnée** et jamais comme du code. Le pilote **pgx v5** (utilisé via la  
bibliothèque standard `database/sql`) transmet ces paramètres hors du texte SQL, par le **protocole
étendu** de PostgreSQL. On ne concatène **jamais** une valeur utilisateur dans une requête (voir
l'en-tête de `internal/repository/commun.go`).

```go
// internal/repository/livre_repository.go — ParUUID
const requete = `SELECT ... FROM vue_livres_details WHERE uuid = $1`
r.db.QueryRowContext(ctx, requete, uuid)   // « uuid » ne peut pas altérer la requête
```

Même la recherche par titre (`ILIKE`, l'équivalent PostgreSQL de `LIKE` insensible à la casse)  
reste paramétrée. Le constructeur de conditions écrit un repère `?`, **puis le remplace** par le  
placeholder numéroté `$N` correspondant, dans l'ordre — la valeur demeure toujours un **argument** :

```go
// internal/repository/livre_repository.go — Lister
conditions.ajouter("titre ILIKE ?", "%"+params.Recherche+"%")   // repère « ? » → « $1 » ; le motif est un ARGUMENT
```

**Le cas particulier du `ORDER BY` : la liste blanche.** Une valeur peut être un paramètre `$N`,
mais **pas** un nom de colonne ni le sens du tri (`ASC`/`DESC`) : ceux-ci font partie de la
**structure** de la requête et ne peuvent **pas** être paramétrés. Interpoler directement le nom de
colonne fourni par le client via `?tri=` rouvrirait la faille. La parade est une **liste blanche** :  
le client envoie un nom logique (`titre`), traduit en nom de colonne réel **uniquement** s'il figure  
dans la liste autorisée.

```go
// internal/handler/livre_handler.go — la liste blanche
var colonnesTriLivre = map[string]string{
    "titre": "titre", "annee": "annee_publication", "prix": "prix", "date": "cree_le",
}
// internal/handler/commun.go — analyserParametresListe : seule une colonne validée est retenue
if colonneSQL, autorise := colonnesTriAutorisees[q.Get("tri")]; autorise {
    params.ColonneTri = colonneSQL           // sûr à interpoler
} else {
    params.ColonneTri = colonneParDefaut     // repli, jamais une valeur brute du client
}
```

Côté base, `clauseTriEtPagination` (dans `internal/repository/commun.go`) interpole cette
`ColonneTri` **déjà validée** dans le `ORDER BY`, puis numérote `LIMIT $N` / `OFFSET $M` à la suite
des paramètres du `WHERE`. Ainsi, `ColonneTri` ne contient **jamais** une valeur brute : toujours  
une colonne validée. Le sens du tri est borné à `ASC`/`DESC` (voir `models.ParametresListe` et sa  
méthode `Normaliser`, dans `internal/models/liste.go`). C'est **la** parade contre l'injection via  
le paramètre `tri`.

**Défenses en profondeur supplémentaires :**

- **Protocole étendu de pgx.** Les appels paramétrés passent par le protocole étendu de PostgreSQL :
  le texte SQL et les valeurs voyagent **séparément** (requêtes préparées **côté serveur**), ce qui
  neutralise l'injection au niveau du protocole (voir `internal/database/database.go`).
- **Pas d'empilement de requêtes.** Un appel paramétré n'exécute **qu'une seule** instruction :
  impossible de glisser une seconde requête (`; DROP TABLE …`) dans un même appel.
- **Rempart côté base : les types `DOMAIN` et les contraintes `CHECK`.** Même si une valeur
  malveillante franchissait toutes les barrières applicatives, la base la **rejetterait**. Les types
  **`DOMAIN`** `courriel` et `isbn13` (voir `sql/schema/02_types.sql`) portent une contrainte
  **`CHECK` (expression régulière)** *réutilisable* : toute colonne déclarée de ce type hérite
  automatiquement de la validation (un e-mail mal formé ou un ISBN non numérique est refusé à
  l'insertion). Les tables ajoutent des **`CHECK` métier** (`prix >= 0`, stock cohérent
  `exemplaires_disponibles <= nombre_exemplaires`, `annee_publication BETWEEN 1400 AND 2200` — voir
  `sql/schema/03_tables.sql`). C'est une défense en profondeur **propre à PostgreSQL** : la règle
  est définie **une seule fois** dans le `DOMAIN` et réutilisée par toutes les colonnes de ce type.
- **Aucune fuite d'erreur SQL.** Les erreurs de la base sont **traduites en erreurs métier** avant
  de repartir vers le client : jamais de message technique brut. PostgreSQL identifie ses erreurs
  par des **codes `SQLSTATE`** normalisés à cinq caractères (`23505` *unique_violation*, `23503`
  *foreign_key_violation*, `23001` *restrict_violation*, `23514` *check_violation*, `P0001`
  *raise_exception* d'un `RAISE EXCEPTION`). Le paquet `internal/database/erreurs.go` les mappe vers
  une réponse claire (`409 Conflit`, etc.) avec un message adapté, sans jamais exposer le code brut.
- **Moindre privilège en base.** Le rôle applicatif `app_bibliotheque` n'a ni `DROP`, ni `ALTER`, ni
  `TRUNCATE`, ni `CREATE` sur le schéma `public` (voir `sql/schema/01_roles.sql` et le §12). Même
  une injection réussie ne pourrait **pas** détruire le schéma.

Pour les détails du moteur (types, contraintes, rôles, codes `SQLSTATE`), voir
[DATABASE.md](../DATABASE.md) et [POSTGRESQL.md](../POSTGRESQL.md).

---

## 2. XSS (Cross-Site Scripting)

**La menace.** Le XSS injecte du **JavaScript** dans une page vue par une victime (par ex. un champ
qui contient `<script>…</script>` réaffiché sans échappement). Le script s'exécute dans le  
navigateur de la victime et peut voler des jetons, des cookies, etc.

**La parade dans ce projet.** L'API est **JSON pur** : elle ne rend **aucune page HTML**. Le risque
XSS se situerait côté client (le site qui consomme l'API). L'API réduit néanmoins la surface :

- **`Content-Type: application/json; charset=utf-8`** systématique (voir
  `internal/reponse/reponse.go`) : le contenu est déclaré comme des données, pas du HTML exécutable.
- **`X-Content-Type-Options: nosniff`** : empêche le navigateur de « deviner » le type d'un contenu
  et de l'exécuter comme du HTML/JS.
- **`Content-Security-Policy: default-src 'none'; frame-ancestors 'none'`** : pour une API JSON, on
  n'autorise **aucune** ressource active (voir `internal/middleware/securite.go`).
- L'encodeur JSON standard de Go **échappe** correctement les caractères ; les données stockées le
  sont telles quelles et renvoyées comme du JSON, à charge pour le client de les afficher en
  échappant (bonne pratique côté front).

---

## 3. CSRF (Cross-Site Request Forgery)

**La menace.** Le CSRF piège un utilisateur **déjà authentifié** pour qu'il envoie, à son insu, une
requête à une API où il a une session active (typiquement via un **cookie** envoyé automatiquement  
par le navigateur).

**La parade dans ce projet : l'authentification par jeton Bearer, pas par cookie.**

L'API n'utilise **pas** de cookie de session : l'identité voyage dans l'en-tête
`Authorization: Bearer <jeton>` (voir `internal/middleware/auth.go`). Or un en-tête `Authorization`
**n'est pas** envoyé automatiquement par le navigateur vers un autre site : un formulaire malveillant
ne peut pas l'ajouter. Le vecteur CSRF classique (cookie ambiant) **ne s'applique donc pas**.

En complément :

- La **politique CORS** (voir §7 et `internal/middleware/cors.go`) encadre quelles origines peuvent
  appeler l'API depuis un navigateur ; `Access-Control-Allow-Credentials` n'est activé que pour une
  origine **précise**, jamais avec `*`.
- Si l'on ajoutait un jour des cookies, il faudrait alors réintroduire une protection CSRF dédiée
  (jeton anti-CSRF, `SameSite`…).

---

## 4. Mass Assignment

**La menace.** Le *Mass Assignment* survient quand on désérialise directement le JSON du client
dans l'entité métier. Un attaquant peut alors « glisser » des champs qu'il ne devrait pas pouvoir  
fixer : `{"email":"…", "role":"admin", "actif":true}` le promeut administrateur.

**La parade dans ce projet : des structures d'entrée dédiées + rejet des champs inconnus.**

On ne désérialise **jamais** le JSON client dans une entité. On utilise des **structures d'entrée**
(DTO) qui décrivent **exactement** ce que le client a le droit d'envoyer (voir
`internal/models/*.go`). Par exemple, `InscriptionEntree` **ne contient pas** de champ `role` :

```go
// internal/models/utilisateur.go
type InscriptionEntree struct {
    Email      string `json:"email"`
    MotDePasse string `json:"mot_de_passe"`
    Nom        string `json:"nom"`
    Prenom     string `json:"prenom"`
    // Pas de « role » : impossible de s'auto-promouvoir. Le service force RoleMembre.
}
```

Renfort au décodage : **`DisallowUnknownFields`**. Tout champ non prévu provoque une erreur `400`  
explicite (voir `internal/handler/commun.go`), ce qui empêche d'injecter un champ inattendu **et**  
aide au diagnostic :

```go
decodeur := json.NewDecoder(r.Body)
decodeur.DisallowUnknownFields()   // « role » sur une inscription => 400 « Champ non autorisé »
```

Enfin, la logique métier applique des garde-fous explicites :

- à l'inscription, `Role` est **forcé** à `membre` (`internal/service/auth_service.go`) ;
- sur `PATCH /moi`, les champs `role` et `actif` sont **neutralisés** côté handler
  (`internal/handler/utilisateur_handler.go`) ;
- dans `UtilisateurService.Modifier`, `role`/`actif` ne sont pris en compte que si
  `appelantEstAdmin` (sinon `403`) — protection contre l'**escalade de privilèges**.

---

## 5. Force brute (Brute Force)

**La menace.** Un attaquant essaie des milliers de mots de passe sur un compte (ou un mot de passe
sur des milliers de comptes) jusqu'à en trouver un valide.

**Parade 1 — `bcrypt` avec un coût élevé.** Les mots de passe sont hachés avec **bcrypt, coût 12**
(voir `internal/auth/mot_de_passe.go`). Pourquoi bcrypt plutôt qu'un SHA-256 « simple » ?

- bcrypt intègre un **sel aléatoire unique** par mot de passe : deux comptes ayant le même mot de
  passe ont des hachés différents (pas d'attaque par **table arc-en-ciel**) ;
- bcrypt est **lent à dessein** (facteur de coût) : chaque essai coûte ~100–300 ms, ce qui rend une
  attaque par force brute massivement plus coûteuse. Chaque incrément du coût **double** le temps.

La vérification utilise `bcrypt.CompareHashAndPassword`, à **temps constant**, ce qui empêche les
**attaques temporelles** (on ne compare jamais deux hachés avec un simple `==`).

**Parade 2 — limitation de débit** (voir §7) : le nombre de tentatives par IP et par seconde est
plafonné.

**Parade 3 — pas d'énumération de comptes.** À la connexion, un e-mail inconnu **et** un mot de
passe erroné renvoient le **même** message générique « Identifiants invalides » (voir
`AuthService.Connexion`). L'attaquant ne peut pas déduire quels e-mails existent.

**Parade 4 — refresh tokens robustes.** Les refresh tokens sont tirés de `crypto/rand` (256 bits
d'entropie), **non devinables**, et stockés **hachés** (SHA-256) ; ils subissent une **rotation** à  
chaque usage (voir §12 et `internal/auth/jwt.go`).

---

## 6. Slowloris (clients lents)

**La menace.** L'attaque **Slowloris** ouvre de nombreuses connexions et envoie les données  
**très lentement** (un octet toutes les quelques secondes), gardant les connexions ouvertes le plus
longtemps possible pour **épuiser** la capacité du serveur, sans gros volume de trafic.

**La parade dans ce projet : des délais (timeouts) stricts sur le serveur HTTP.**

Le `http.Server` fixe des délais qui coupent une connexion qui traîne (voir `cmd/api/main.go`) :

```go
serveur := &http.Server{
    ReadHeaderTimeout: cfg.Serveur.DelaiLecture,   // 10s : lire les en-têtes
    ReadTimeout:       cfg.Serveur.DelaiLecture,   // 10s : lire toute la requête
    WriteTimeout:      cfg.Serveur.DelaiEcriture,  // 15s : écrire la réponse
    IdleTimeout:       cfg.Serveur.DelaiInactif,   // 60s : entre deux requêtes (keep-alive)
}
```

- `ReadHeaderTimeout` / `ReadTimeout` bornent le temps qu'un client a pour **envoyer** sa requête :
  un client qui n'envoie qu'un octet toutes les 10 s est déconnecté.
- `WriteTimeout` borne l'**écriture** de la réponse.
- `IdleTimeout` libère les connexions keep-alive inactives.

Ces valeurs sont **configurables** (`SERVEUR_DELAI_*`) pour s'adapter au contexte.

---

## 7. DoS / DDoS

**La menace.** Un **déni de service** (DoS), éventuellement **distribué** (DDoS), sature le serveur
pour le rendre indisponible : trop de requêtes, corps de requête gigantesques, traitements  
interminables…

**Parade 1 — limitation de débit (rate limiting).** Le middleware
`internal/middleware/rate_limiter.go` applique un algorithme **« seau à jetons »** (token bucket)
par **adresse IP** :

- chaque IP dispose d'un seau qui se remplit à `RATE_LIMIT_PAR_SECONDE` jetons/seconde (défaut 10) ;
- chaque requête consomme un jeton ; le seau tolère une **rafale** de `RATE_LIMIT_RAFALE` (défaut
  20) ;
- seau vide → **`429 Too Many Requests`** avec l'en-tête `Retry-After`.

L'IP est lue depuis `RemoteAddr` (l'IP de la connexion TCP), **pas** depuis `X-Forwarded-For`
(falsifiable), sauf à se trouver derrière un proxy de confiance (voir la note dans
`internal/middleware/middleware.go`). Une goroutine purge périodiquement les IP inactives pour
éviter une fuite mémoire, et s'arrête proprement à l'arrêt du serveur (via `context.Context`).

**Parade 2 — limite de la taille du corps.** `internal/middleware/body_limit.go` enveloppe le corps
avec `http.MaxBytesReader` : au-delà de `REQUETE_TAILLE_MAX_OCTETS` (défaut **1 Mio**), la lecture
échoue (`413`) **et** la connexion est interrompue si le client continue d'envoyer. Sans cette
limite, un corps de plusieurs gigaoctets pourrait saturer la mémoire.

**Parade 3 — délai de traitement.** `internal/middleware/timeout.go` attache un délai
(`SERVEUR_DELAI_TRAITEMENT`, défaut 10 s) au `context.Context` de la requête. Quand il expire, le
contexte est **annulé** : les requêtes SQL en cours (qui reçoivent ce contexte) **s'interrompent**,  
libérant les ressources. C'est complémentaire des délais réseau du §6.

**Parade 4 — timeouts serveur** (voir §6) et **pool de connexions borné** (voir
[PERFORMANCES.md](PERFORMANCES.md)) : le nombre de connexions à la base est plafonné, ce qui protège
PostgreSQL d'une avalanche.

> Une protection **complète** contre un DDoS volumétrique se joue aussi en amont (pare-feu, CDN,  
> anti-DDoS réseau). L'application fait sa part : elle ne s'écroule pas sous un abus applicatif.

---

## 8. En-têtes de sécurité HTTP

Le middleware `internal/middleware/securite.go` ajoute à **chaque réponse** des en-têtes qui  
durcissent le comportement des navigateurs. Chacun répond à une menace précise :

| En-tête                                                        | Protège contre…                                             |
|---------------------------------------------------------------|-------------------------------------------------------------|
| `X-Content-Type-Options: nosniff`                             | Le *MIME sniffing* (exécution d'un contenu mal typé, XSS)   |
| `X-Frame-Options: DENY`                                       | Le **clickjacking** (notre réponse dans une `<iframe>`)     |
| `Content-Security-Policy: default-src 'none'; frame-ancestors 'none'` | XSS et inclusion en iframe (API JSON : aucune ressource active) |
| `Referrer-Policy: no-referrer`                               | La fuite d'URL d'origine lors des navigations sortantes     |
| `Strict-Transport-Security: max-age=63072000; includeSubDomains` | Le *downgrade* HTTP (force HTTPS ; utile derrière un TLS)    |
| `Cache-Control: no-store`                                    | La mise en cache de réponses potentiellement sensibles      |

---

## 9. Gestion des secrets

**La menace.** Un secret (mot de passe de base, clé JWT) codé « en dur » ou committé dans Git fuite
tôt ou tard et compromet tout le système.

**La parade dans ce projet : configuration par l'environnement (12-Factor).**

- **Aucun secret dans le code.** Toute la configuration est lue depuis les **variables
  d'environnement** (voir `internal/config/config.go`). Le même binaire tourne en dev, test et prod.
- **`.env` non versionné.** `.gitignore` exclut `.env` (et `*.env`) mais **conserve** `.env.example`
  (modèle sans secret). On ne committe **jamais** de vrai mot de passe.
- **Validation au démarrage.** L'application **refuse de démarrer** si `BDD_MOT_DE_PASSE` ou
  `JWT_SECRET` sont vides, ou si le secret JWT fait **moins de 32 caractères** (un secret court
  affaiblit la signature HMAC-SHA256). « Échouer tôt et bruyamment » évite un démarrage silencieux
  dans un état non sécurisé.
- **Durcissement en production.** En `APP_ENVIRONNEMENT=production`, la configuration **interdit**
  `CORS_ORIGINES_AUTORISEES=*` (voir `config.valider`).
- **Le secret n'est pas dans le jeton.** Un JWT est **signé** mais **pas chiffré** : son contenu est
  lisible. On n'y met donc que l'UUID, l'e-mail et le rôle — **jamais** le mot de passe (voir
  `internal/auth/jwt.go`).

En production réelle, on remplacerait `.env` par un **gestionnaire de secrets** (Vault, secrets de  
l'orchestrateur…) ; le code n'a pas à changer puisqu'il lit l'environnement.

---

## 10. Conteneurs non-root et surface minimale

**La menace.** Un conteneur qui tourne en **`root`** aggrave l'impact d'une compromission :
l'attaquant hérite de droits étendus, potentiellement au-delà du conteneur.

**La parade dans ce projet (voir `Dockerfile`) :**

- **Build multi-stage.** Une étape compile le binaire avec tout le SDK Go ; l'**image finale** ne
  contient **que le binaire** (pas le compilateur, pas le code source), sur une base `alpine`
  minimale (~20 Mo). Moins de composants = **surface d'attaque réduite**.
- **Utilisateur non privilégié.** On crée `appuser` (UID 10001) et l'on bascule dessus avec
  `USER appuser` : le conteneur **ne tourne pas en root**.
- **Binaire statique** (`CGO_ENABLED=0`) : aucune dépendance à la libc du système, moins de vecteurs.
- **`.dockerignore`** exclut `.git`, `.env`, etc. du contexte de build : pas de secret embarqué par
  accident dans une couche d'image.
- **`HEALTHCHECK`** intégré : l'orchestrateur détecte un conteneur malade et réagit.

---

## 11. Journalisation sécurisée

**La menace.** Des logs trop bavards peuvent **fuiter** des données sensibles (mots de passe,
jetons, contenu de requêtes) ou aider un attaquant en révélant la structure interne (requêtes SQL).

**La parade dans ce projet :**

- **On ne journalise ni le corps ni la *query string*.** Le middleware `internal/middleware/logger.go`
  trace la **méthode** et le **chemin** (`r.URL.Path`), mais **pas** `r.URL.RawQuery` ni le corps :
  ils pourraient contenir des jetons ou des mots de passe. C'est une **règle d'or**.
- **Les mots de passe ne sortent jamais du serveur.** Le champ `MotDePasseHash` porte `json:"-"`
  (jamais sérialisé), et les triggers d'audit **excluent** explicitement le mot de passe des
  photos JSON (voir `sql/triggers/08_triggers.sql`).
- **Aucune fuite technique au client.** Toute erreur passe par `apperreur.Depuis` : une erreur
  inconnue devient un **`500` générique**. La **cause technique** (requête SQL, etc.) est
  journalisée **côté serveur uniquement**, avec l'`identifiant_requete`, jamais renvoyée au client
  (voir `internal/reponse/reponse.go` et `internal/apperreur/apperreur.go`).
- **Traçabilité sans injection.** Chaque requête reçoit un `X-Request-ID` (UUID). Si le client en
  fournit un, il n'est accepté **que** s'il est au format UUID valide (voir
  `internal/middleware/request_id.go`) — pour éviter l'**injection de contenu** dans les logs.
- **Les paniques sont capturées.** `internal/middleware/recovery.go` intercepte les paniques,
  journalise la pile d'appels **côté serveur** et renvoie un `500` neutre — jamais la trace au
  client.

---

## 12. Autres protections transverses

**Identifiants publics non énumérables (anti-IDOR).** On expose des **UUID** (`json:"id"`) et jamais
les clés séquentielles internes (`json:"-"`). Un attaquant ne peut pas deviner `/livres/2`,
`/livres/3`… pour parcourir les ressources (voir tous les modèles dans `internal/models/`).

**Cycle de vie des jetons.**

- Jeton d'accès JWT à **courte durée** (15 min) : une fuite a une fenêtre d'exploitation réduite.
- **Algorithme de signature imposé** (`HS256`) à la vérification : bloque l'attaque classique
  `alg: none` ou la confusion d'algorithme (voir `jwt.WithValidMethods` dans
  `internal/auth/jwt.go`).
- Refresh tokens **hachés** en base (SHA-256), **rotation** à chaque rafraîchissement (l'ancien est
  révoqué), purge automatique des expirés/révoqués par une **tâche `pg_cron`** horaire
  (`bib_purger_jetons`, voir `sql/cron/09_cron.sql`).
- **Vérification du compte actif** à chaque connexion et rafraîchissement : un compte désactivé ne
  peut plus obtenir de jeton.

**Suppression logique.** Les suppressions courantes sont **logiques** (`supprime_le`) : les données
restent pour l'audit et l'historique, et un compte supprimé est aussi passé `actif = FALSE`.

**Contraintes en base (défense en profondeur).** `CHECK`, `DOMAIN`, `UNIQUE`, `ENUM`, clés
étrangères et triggers (`RAISE EXCEPTION`) garantissent l'intégrité **même si** un bug applicatif
passait au travers (voir [DATABASE.md](../DATABASE.md)).

**Moindre privilège SQL.** L'application ne se connecte **jamais** avec le superutilisateur
(`postgres`). Elle utilise un rôle dédié `app_bibliotheque` (attribut `LOGIN`, **non**
superutilisateur), qui ne reçoit que le strict nécessaire : `CONNECT` sur la base, `USAGE` sur le  
schéma `public`, et le **CRUD** (`SELECT`/`INSERT`/`UPDATE`/`DELETE`) accordé automatiquement aux  
tables via `ALTER DEFAULT PRIVILEGES` (plus `EXECUTE` sur les fonctions et `USAGE`/`SELECT` sur les  
séquences). Il n'a **ni** `DROP`, **ni** `ALTER`, **ni** `TRUNCATE`, **ni** `CREATE` — la création  
d'objets dans `public` a d'ailleurs été **révoquée** de `PUBLIC` (`REVOKE CREATE ON SCHEMA public  
FROM PUBLIC`). Un garde-fou `statement_timeout = 30s` est même posé sur le rôle. Conséquence :
**même en cas d'injection réussie, l'attaquant ne peut pas détruire le schéma** ni faire tourner une
requête sans fin (voir `sql/schema/01_roles.sql`).

**Connexion à la base maîtrisée.** L'application dialogue avec PostgreSQL (port `5432`) via le pilote
pgx sur un **réseau Docker privé** ; la connexion utilise `sslmode=disable` (pas de TLS), acceptable
**tant que** les deux extrémités restent sur ce réseau isolé. Sur un réseau non maîtrisé (base
managée, lien traversant Internet), on passerait à `sslmode=require` — voire `verify-full` avec  
vérification du certificat — **sans changer une ligne** de code applicatif. La session est en outre
**forcée en UTC** (horodatages cohérents, pas d'ambiguïté de fuseau). Voir
`internal/database/database.go`.

**Arrêt gracieux.** À la réception de `SIGINT`/`SIGTERM`, le serveur laisse les requêtes en cours se
terminer avant de fermer (voir `cmd/api/main.go`) : pas de réponse tronquée ni de transaction  
laissée ouverte.

---

## Récapitulatif

| Menace                 | Protection principale                                | Où (fichier)                                  |
|------------------------|------------------------------------------------------|-----------------------------------------------|
| Injection SQL          | Requêtes préparées `$1, $2` + liste blanche `ORDER BY` | `repository/*`, `handler/commun.go`, `models/liste.go` |
| XSS                    | JSON pur + `nosniff` + CSP `default-src 'none'`      | `reponse/reponse.go`, `middleware/securite.go`|
| CSRF                   | Auth par Bearer (pas de cookie) + CORS encadré       | `middleware/auth.go`, `middleware/cors.go`    |
| Mass Assignment        | Structures d'entrée dédiées + `DisallowUnknownFields`| `models/*`, `handler/commun.go`, `service/*`  |
| Force brute            | bcrypt coût 12 + rate limit + pas d'énumération      | `auth/mot_de_passe.go`, `middleware/rate_limiter.go` |
| Slowloris              | Timeouts serveur HTTP                                | `cmd/api/main.go`                             |
| DoS / DDoS             | Rate limit + limite de corps + timeout de traitement | `middleware/rate_limiter.go`, `body_limit.go`, `timeout.go` |
| En-têtes navigateur    | 6 en-têtes de sécurité                               | `middleware/securite.go`                      |
| Secrets                | Variables d'environnement + validation au démarrage  | `config/config.go`, `.gitignore`              |
| Conteneur              | Multi-stage, non-root, image minimale                | `Dockerfile`                                  |
| Logs                   | Ni corps ni query string ; aucune fuite d'erreur     | `middleware/logger.go`, `reponse/reponse.go`  |
| IDOR                   | Identifiants publics UUID                             | `models/*`                                    |
| Jetons                 | Courte durée, `HS256` imposé, refresh haché + rotation | `auth/jwt.go`, `service/auth_service.go`    |
| Intégrité              | `CHECK`/`DOMAIN`/`UNIQUE`/FK/`RAISE` + moindre privilège SQL | `sql/schema/*`, `sql/triggers/*`         |

Pour aller plus loin : [OWASP API Security Top 10](https://owasp.org/API-Security/) et
[OWASP Cheat Sheet Series](https://cheatsheetseries.owasp.org/).
