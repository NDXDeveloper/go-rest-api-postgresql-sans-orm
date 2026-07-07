// Package service contient la LOGIQUE MÉTIER de l'application. C'est la couche
// intermédiaire entre les handlers (HTTP) et les repositories (SQL).
//
// Responsabilités d'un service :
//   - valider les données d'entrée (règles métier) ;
//   - orchestrer un ou plusieurs repositories ;
//   - appliquer les règles (quotas, autorisations métier, protection contre le
//     Mass Assignment via des structures d'entrée dédiées) ;
//   - ne JAMAIS contenir de SQL (c'est le rôle des repositories) ni de détail HTTP.
//
// # Pourquoi des interfaces de repository ici ?
//
// Chaque service dépend d'une INTERFACE (ex. UtilisateurRepo), pas de la
// structure concrète du package repository. Deux bénéfices :
//   - DÉCOUPLAGE : on pourrait changer d'implémentation de stockage sans toucher
//     aux services ;
//   - TESTABILITÉ : dans les tests, on injecte un « faux » repository (mock) qui
//     implémente l'interface, sans base de données réelle.
//
// On définit les interfaces du côté CONSOMMATEUR (le service), comme le
// recommande la communauté Go (« accept interfaces, return structs »).
package service

import (
	"context"
	"time"

	"github.com/exemple/api-bibliotheque/internal/models"
)

// UtilisateurRepo décrit les opérations de persistance des utilisateurs dont les
// services ont besoin.
type UtilisateurRepo interface {
	Creer(ctx context.Context, u *models.Utilisateur) error
	ParUUID(ctx context.Context, uuid string) (*models.Utilisateur, error)
	ParEmail(ctx context.Context, email string) (*models.Utilisateur, error)
	ParID(ctx context.Context, id int64) (*models.Utilisateur, error)
	Lister(ctx context.Context, params models.ParametresListe) ([]models.Utilisateur, int, error)
	MettreAJour(ctx context.Context, u *models.Utilisateur) error
	SupprimerLogique(ctx context.Context, uuid string) error
	SupprimerPhysique(ctx context.Context, uuid string) error
}

// JetonRepo décrit la persistance des refresh tokens.
type JetonRepo interface {
	Enregistrer(ctx context.Context, utilisateurID int64, jetonHache string, expireLe time.Time) error
	TrouverUtilisateurValide(ctx context.Context, jetonHache string) (int64, error)
	Revoquer(ctx context.Context, jetonHache string) error
	RevoquerTousPourUtilisateur(ctx context.Context, utilisateurID int64) error
}

// AuteurRepo décrit la persistance des auteurs.
type AuteurRepo interface {
	Creer(ctx context.Context, a *models.Auteur) error
	ParUUID(ctx context.Context, uuid string) (*models.Auteur, error)
	Lister(ctx context.Context, params models.ParametresListe) ([]models.Auteur, int, error)
	MettreAJour(ctx context.Context, a *models.Auteur) error
	Supprimer(ctx context.Context, uuid string) error
}

// CategorieRepo décrit la persistance des catégories.
type CategorieRepo interface {
	Creer(ctx context.Context, c *models.Categorie) error
	ParUUID(ctx context.Context, uuid string) (*models.Categorie, error)
	Lister(ctx context.Context, params models.ParametresListe) ([]models.Categorie, int, error)
	MettreAJour(ctx context.Context, c *models.Categorie) error
	Supprimer(ctx context.Context, uuid string) error
}

// LivreRepo décrit la persistance des livres.
type LivreRepo interface {
	Creer(ctx context.Context, l *models.Livre) error
	ParUUID(ctx context.Context, uuid string) (*models.Livre, error)
	ParUUIDInterne(ctx context.Context, uuid string) (*models.Livre, error)
	Lister(ctx context.Context, params models.ParametresListe) ([]models.Livre, int, error)
	MettreAJour(ctx context.Context, l *models.Livre) error
	SupprimerLogique(ctx context.Context, uuid string) error
	SupprimerPhysique(ctx context.Context, uuid string) error
}

// EmpruntRepo décrit la persistance des emprunts (dont les opérations
// transactionnelles et les appels de procédures).
type EmpruntRepo interface {
	Emprunter(ctx context.Context, utilisateurUUID, livreUUID string, dureeJours int) (string, error)
	Rendre(ctx context.Context, empruntUUID string) (float64, error)
	ParUUID(ctx context.Context, uuid string) (*models.Emprunt, error)
	Lister(ctx context.Context, utilisateurUUID string, params models.ParametresListe) ([]models.Emprunt, int, error)
	StatistiquesUtilisateur(ctx context.Context, utilisateurUUID string) (models.StatistiquesUtilisateur, error)
}
