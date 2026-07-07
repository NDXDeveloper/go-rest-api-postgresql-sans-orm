// Package config centralise TOUTE la configuration de l'application.
//
// # Principe : la configuration vient de l'environnement, jamais du code
//
// Aucune valeur sensible (mot de passe, secret JWT) ni aucune valeur susceptible
// de changer entre les environnements (hôte de la base, port...) ne doit être
// écrite « en dur » dans le code. On applique la méthodologie « 12-Factor App » :
// la configuration est lue depuis les variables d'environnement.
//
// Avantages :
//   - Le même binaire fonctionne en développement, en test et en production.
//   - Les secrets ne sont pas versionnés dans Git (voir .gitignore + .env.example).
//   - Docker/Docker Compose injecte naturellement des variables d'environnement.
//
// Pour le confort du développement local (hors Docker), on charge aussi un
// éventuel fichier .env. En production, on privilégie les vraies variables
// d'environnement injectées par l'orchestrateur.
package config

import (
	"bufio"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Config regroupe toute la configuration, organisée par domaine fonctionnel.
//
// On évite ainsi une longue liste plate de champs : chaque sous-structure
// (Serveur, BaseDeDonnees...) a une responsabilité claire.
type Config struct {
	Environnement string // "developpement" ou "production"
	Serveur       Serveur
	BaseDeDonnees BaseDeDonnees
	JWT           JWT
	Securite      Securite
	Log           Log
}

// Serveur contient les paramètres du serveur HTTP.
type Serveur struct {
	Adresse string // interface d'écoute, ex. "0.0.0.0"
	Port    int    // port d'écoute, ex. 8080

	// Les délais (timeouts) protègent contre les clients lents ou malveillants
	// (attaque « Slowloris »). Voir le middleware Timeout et la doc sécurité.
	DelaiLecture  time.Duration // temps max pour lire l'en-tête + le corps
	DelaiEcriture time.Duration // temps max pour écrire la réponse
	DelaiInactif  time.Duration // temps max entre deux requêtes (keep-alive)

	// DelaiTraitement borne la durée de traitement applicatif d'une requête
	// (middleware Timeout). Doit rester inférieur à DelaiEcriture pour que le
	// contexte expire AVANT que le serveur ne coupe l'écriture.
	DelaiTraitement time.Duration

	// DelaiArretGracieux : durée laissée aux requêtes en cours pour se terminer
	// lors de l'arrêt du serveur (SIGINT/SIGTERM).
	DelaiArretGracieux time.Duration
}

// AdresseComplete renvoie l'adresse « hôte:port » attendue par http.Server.
func (s Serveur) AdresseComplete() string {
	return fmt.Sprintf("%s:%d", s.Adresse, s.Port)
}

// BaseDeDonnees contient les paramètres de connexion à PostgreSQL et de gestion
// du pool de connexions.
type BaseDeDonnees struct {
	Hote        string
	Port        int
	Nom         string
	Utilisateur string
	MotDePasse  string

	// Paramètres du pool de connexions (voir package database pour les explications
	// détaillées sur pourquoi ces réglages sont importants).
	MaxConnexionsOuvertes  int           // nb max de connexions simultanées
	MaxConnexionsInactives int           // nb max de connexions gardées au repos
	DureeVieMaxConnexion   time.Duration // durée de vie max d'une connexion
	DelaiConnexion         time.Duration // délai max pour établir/valider la connexion
}

// JWT contient les paramètres de génération et de vérification des jetons.
type JWT struct {
	Secret                string        // clé secrète de signature HMAC (obligatoire)
	Emetteur              string        // champ "iss" du jeton
	DureeAcces            time.Duration // durée de validité d'un jeton d'accès (court)
	DureeRafraichissement time.Duration // durée de validité d'un refresh token (long)
}

// Securite regroupe les paramètres liés à la protection du serveur.
type Securite struct {
	OriginesAutorisees    []string // origines CORS autorisées
	TailleMaxCorpsOctets  int64    // taille max du corps d'une requête (anti-DoS)
	LimiteDebitParSeconde float64  // requêtes/seconde autorisées par client
	LimiteDebitRafale     int      // pic de requêtes toléré (burst)
}

// Log contient les paramètres de journalisation structurée.
type Log struct {
	Niveau string // "debug", "info", "warn", "error"
	Format string // "json" (production) ou "texte" (développement)
}

// Charger lit la configuration depuis l'environnement et renvoie une *Config
// validée, ou une erreur explicite si une valeur obligatoire manque.
//
// L'ordre de résolution d'une variable est :
//  1. variable d'environnement réelle (prioritaire) ;
//  2. valeur définie dans le fichier .env (si présent) ;
//  3. valeur par défaut fournie dans le code.
func Charger() (*Config, error) {
	// On tente de charger un fichier .env pour le confort du développement local.
	// L'absence de fichier n'est pas une erreur : en production les variables
	// sont injectées directement dans l'environnement.
	_ = chargerFichierEnv(".env")

	cfg := &Config{
		Environnement: lireChaine("APP_ENVIRONNEMENT", "developpement"),
		Serveur: Serveur{
			Adresse:            lireChaine("SERVEUR_ADRESSE", "0.0.0.0"),
			Port:               lireEntier("SERVEUR_PORT", 8080),
			DelaiLecture:       lireDuree("SERVEUR_DELAI_LECTURE", 10*time.Second),
			DelaiEcriture:      lireDuree("SERVEUR_DELAI_ECRITURE", 15*time.Second),
			DelaiInactif:       lireDuree("SERVEUR_DELAI_INACTIF", 60*time.Second),
			DelaiTraitement:    lireDuree("SERVEUR_DELAI_TRAITEMENT", 10*time.Second),
			DelaiArretGracieux: lireDuree("SERVEUR_DELAI_ARRET", 15*time.Second),
		},
		BaseDeDonnees: BaseDeDonnees{
			Hote:                   lireChaine("BDD_HOTE", "127.0.0.1"),
			Port:                   lireEntier("BDD_PORT", 5432),
			Nom:                    lireChaine("BDD_NOM", "bibliotheque"),
			Utilisateur:            lireChaine("BDD_UTILISATEUR", "app_bibliotheque"),
			MotDePasse:             lireChaine("BDD_MOT_DE_PASSE", ""),
			MaxConnexionsOuvertes:  lireEntier("BDD_MAX_CONNEXIONS_OUVERTES", 25),
			MaxConnexionsInactives: lireEntier("BDD_MAX_CONNEXIONS_INACTIVES", 25),
			DureeVieMaxConnexion:   lireDuree("BDD_DUREE_VIE_CONNEXION", 5*time.Minute),
			DelaiConnexion:         lireDuree("BDD_DELAI_CONNEXION", 5*time.Second),
		},
		JWT: JWT{
			Secret:                lireChaine("JWT_SECRET", ""),
			Emetteur:              lireChaine("JWT_EMETTEUR", "api-bibliotheque"),
			DureeAcces:            lireDuree("JWT_DUREE_ACCES", 15*time.Minute),
			DureeRafraichissement: lireDuree("JWT_DUREE_RAFRAICHISSEMENT", 168*time.Hour), // 7 jours
		},
		Securite: Securite{
			OriginesAutorisees:    lireListe("CORS_ORIGINES_AUTORISEES", []string{"*"}),
			TailleMaxCorpsOctets:  int64(lireEntier("REQUETE_TAILLE_MAX_OCTETS", 1<<20)), // 1 Mio
			LimiteDebitParSeconde: lireFlottant("RATE_LIMIT_PAR_SECONDE", 10),
			LimiteDebitRafale:     lireEntier("RATE_LIMIT_RAFALE", 20),
		},
		Log: Log{
			Niveau: lireChaine("LOG_NIVEAU", "info"),
			Format: lireChaine("LOG_FORMAT", "texte"),
		},
	}

	if err := cfg.valider(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// EstProduction indique si l'on tourne en environnement de production.
// On s'en sert pour durcir certains comportements (pas de message d'erreur
// détaillé, CORS restrictif...).
func (c *Config) EstProduction() bool {
	return c.Environnement == "production"
}

// valider vérifie la cohérence de la configuration et refuse de démarrer si une
// valeur critique est absente. « Échouer tôt et bruyamment » est préférable à
// un démarrage silencieux dans un état incorrect.
func (c *Config) valider() error {
	if c.BaseDeDonnees.MotDePasse == "" {
		return fmt.Errorf("configuration invalide : BDD_MOT_DE_PASSE est obligatoire")
	}
	if c.JWT.Secret == "" {
		return fmt.Errorf("configuration invalide : JWT_SECRET est obligatoire")
	}
	// Un secret JWT trop court affaiblit la signature HMAC-SHA256.
	if len(c.JWT.Secret) < 32 {
		return fmt.Errorf("configuration invalide : JWT_SECRET doit faire au moins 32 caractères (actuel : %d)", len(c.JWT.Secret))
	}
	if c.Serveur.Port < 1 || c.Serveur.Port > 65535 {
		return fmt.Errorf("configuration invalide : SERVEUR_PORT hors plage (%d)", c.Serveur.Port)
	}
	// En production, autoriser toutes les origines CORS ("*") est dangereux.
	if c.EstProduction() && slices.Contains(c.Securite.OriginesAutorisees, "*") {
		return fmt.Errorf("configuration invalide : CORS_ORIGINES_AUTORISEES ne doit pas contenir '*' en production")
	}
	return nil
}

// --- Lecteurs typés --------------------------------------------------------
//
// Ces fonctions convertissent une variable d'environnement (toujours une chaîne)
// vers le type attendu, en appliquant une valeur par défaut si la variable est
// absente ou vide.

// lireChaine renvoie la variable d'environnement ou la valeur par défaut.
func lireChaine(cle, defaut string) string {
	if v, ok := os.LookupEnv(cle); ok && v != "" {
		return v
	}
	return defaut
}

// lireEntier convertit la variable en int. En cas de valeur invalide, on
// retombe sur la valeur par défaut (comportement tolérant et prévisible).
func lireEntier(cle string, defaut int) int {
	if v, ok := os.LookupEnv(cle); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaut
}

// lireFlottant convertit la variable en float64.
func lireFlottant(cle string, defaut float64) float64 {
	if v, ok := os.LookupEnv(cle); ok && v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return defaut
}

// lireDuree convertit la variable en time.Duration.
// Format accepté : celui de time.ParseDuration, ex. "15s", "5m", "168h".
func lireDuree(cle string, defaut time.Duration) time.Duration {
	if v, ok := os.LookupEnv(cle); ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return defaut
}

// lireListe découpe une variable « a,b,c » en tranche de chaînes nettoyées.
func lireListe(cle string, defaut []string) []string {
	v, ok := os.LookupEnv(cle)
	if !ok || v == "" {
		return defaut
	}
	parts := strings.Split(v, ",")
	resultat := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			resultat = append(resultat, p)
		}
	}
	if len(resultat) == 0 {
		return defaut
	}
	return resultat
}

// chargerFichierEnv lit un fichier au format « CLE=valeur » et positionne les
// variables d'environnement correspondantes SI elles ne sont pas déjà définies.
//
// On écrit volontairement notre propre mini-analyseur plutôt que d'ajouter une
// dépendance : le format .env est trivial, et cela reste pédagogique.
//
// Règles supportées :
//   - lignes vides ignorées ;
//   - lignes commençant par '#' ignorées (commentaires) ;
//   - « export CLE=valeur » toléré ;
//   - guillemets simples ou doubles optionnels autour de la valeur.
func chargerFichierEnv(chemin string) error {
	//nolint:gosec // G304 : le chemin est fixe (« .env »), défini par l'application et non par une entrée utilisateur.
	fichier, err := os.Open(chemin)
	if err != nil {
		return err // fichier absent : ce n'est pas bloquant, l'appelant ignore l'erreur
	}
	defer fichier.Close()

	scanner := bufio.NewScanner(fichier)
	for scanner.Scan() {
		ligne := strings.TrimSpace(scanner.Text())
		if ligne == "" || strings.HasPrefix(ligne, "#") {
			continue
		}
		ligne = strings.TrimPrefix(ligne, "export ")

		cle, valeur, trouve := strings.Cut(ligne, "=")
		if !trouve {
			continue // ligne sans '=' : ignorée
		}
		cle = strings.TrimSpace(cle)
		valeur = strings.TrimSpace(valeur)
		valeur = strings.Trim(valeur, `"'`) // retire les guillemets éventuels

		// On ne remplace pas une variable déjà présente dans l'environnement :
		// les vraies variables (production, Docker) restent prioritaires.
		if _, existe := os.LookupEnv(cle); !existe {
			_ = os.Setenv(cle, valeur)
		}
	}
	return scanner.Err()
}
