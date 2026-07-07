package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/exemple/api-bibliotheque/internal/apperreur"
	"github.com/exemple/api-bibliotheque/internal/database"
)

// JetonRepository gère la table jetons_rafraichissement (« refresh tokens »).
//
// SÉCURITÉ : on ne stocke JAMAIS le jeton en clair, uniquement son haché SHA-256.
// Ainsi, une fuite de la base ne permet pas de réutiliser les jetons. Le service
// d'authentification hache le jeton avant de le confier au repository.
type JetonRepository struct {
	db *sql.DB
}

// NouveauJetonRepository construit le repository avec sa dépendance.
func NouveauJetonRepository(db *sql.DB) *JetonRepository {
	return &JetonRepository{db: db}
}

// Enregistrer stocke un nouveau refresh token (haché) pour un utilisateur.
func (r *JetonRepository) Enregistrer(ctx context.Context, utilisateurID int64, jetonHache string, expireLe time.Time) error {
	const requete = `INSERT INTO jetons_rafraichissement (utilisateur_id, jeton_hash, expire_le)
		VALUES ($1, $2, $3)`
	if _, err := r.db.ExecContext(ctx, requete, utilisateurID, jetonHache, expireLe); err != nil {
		if database.EstErreurDoublon(err) {
			// Collision de hachés extrêmement improbable ; on la traite proprement.
			return apperreur.Conflit("Jeton déjà enregistré.")
		}
		return apperreur.Interne("enregistrement du jeton de rafraîchissement").AvecCause(err)
	}
	return nil
}

// TrouverUtilisateurValide renvoie l'identifiant de l'utilisateur associé à un
// refresh token VALIDE : non révoqué et non expiré. Sinon, renvoie une erreur
// 401 (le client doit se reconnecter).
func (r *JetonRepository) TrouverUtilisateurValide(ctx context.Context, jetonHache string) (int64, error) {
	const requete = `SELECT utilisateur_id FROM jetons_rafraichissement
		WHERE jeton_hash = $1 AND revoque = FALSE AND expire_le > NOW()`
	var utilisateurID int64
	if err := r.db.QueryRowContext(ctx, requete, jetonHache).Scan(&utilisateurID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, apperreur.NonAuthentifie("Jeton de rafraîchissement invalide ou expiré.")
		}
		return 0, apperreur.Interne("lecture du jeton de rafraîchissement").AvecCause(err)
	}
	return utilisateurID, nil
}

// Revoquer invalide un refresh token précis (utilisé à la déconnexion, et lors
// de la ROTATION du jeton à chaque rafraîchissement).
func (r *JetonRepository) Revoquer(ctx context.Context, jetonHache string) error {
	const requete = `UPDATE jetons_rafraichissement SET revoque = TRUE WHERE jeton_hash = $1`
	if _, err := r.db.ExecContext(ctx, requete, jetonHache); err != nil {
		return apperreur.Interne("révocation du jeton").AvecCause(err)
	}
	return nil
}

// RevoquerTousPourUtilisateur invalide TOUS les refresh tokens d'un utilisateur
// (déconnexion de tous les appareils, ou réaction à une compromission).
func (r *JetonRepository) RevoquerTousPourUtilisateur(ctx context.Context, utilisateurID int64) error {
	const requete = `UPDATE jetons_rafraichissement SET revoque = TRUE WHERE utilisateur_id = $1`
	if _, err := r.db.ExecContext(ctx, requete, utilisateurID); err != nil {
		return apperreur.Interne("révocation des jetons de l'utilisateur").AvecCause(err)
	}
	return nil
}
