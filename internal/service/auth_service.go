package service

import (
	"context"
	"strings"

	"github.com/exemple/api-bibliotheque/internal/apperreur"
	"github.com/exemple/api-bibliotheque/internal/auth"
	"github.com/exemple/api-bibliotheque/internal/models"
	"github.com/exemple/api-bibliotheque/internal/validation"
	"github.com/google/uuid"
)

// AuthService porte la logique d'inscription, de connexion, de rafraîchissement
// et de déconnexion. Toutes ses dépendances sont injectées (aucune globale).
type AuthService struct {
	utilisateurs UtilisateurRepo
	jetons       JetonRepo
	jwt          *auth.GestionnaireJWT
}

// NouveauAuthService assemble le service avec ses dépendances.
func NouveauAuthService(utilisateurs UtilisateurRepo, jetons JetonRepo, gestionnaireJWT *auth.GestionnaireJWT) *AuthService {
	return &AuthService{utilisateurs: utilisateurs, jetons: jetons, jwt: gestionnaireJWT}
}

// Inscription crée un nouveau compte MEMBRE.
//
// Protection contre le Mass Assignment : le rôle est FORCÉ à « membre ». Même si
// un client malicieux envoie « role: admin » dans le JSON, ce champ n'existe pas
// dans InscriptionEntree : il est donc ignoré à la désérialisation.
func (s *AuthService) Inscription(ctx context.Context, entree models.InscriptionEntree) (*models.Utilisateur, error) {
	email := strings.ToLower(strings.TrimSpace(entree.Email))

	v := validation.Nouveau()
	v.ChampRequis("email", email)
	v.Email("email", email)
	v.LongueurMax("email", email, 254)
	v.ChampRequis("mot_de_passe", entree.MotDePasse)
	v.LongueurMin("mot_de_passe", entree.MotDePasse, 8)
	// bcrypt ignore les octets au-delà de 72 : on refuse donc les mots de passe
	// plus longs pour éviter une fausse impression de sécurité.
	v.LongueurMax("mot_de_passe", entree.MotDePasse, 72)
	v.ChampRequis("nom", entree.Nom)
	v.LongueurMax("nom", entree.Nom, 100)
	v.ChampRequis("prenom", entree.Prenom)
	v.LongueurMax("prenom", entree.Prenom, 100)
	if err := v.Erreur(); err != nil {
		return nil, err
	}

	hache, err := auth.HacherMotDePasse(entree.MotDePasse)
	if err != nil {
		return nil, apperreur.Interne("hachage du mot de passe").AvecCause(err)
	}

	u := &models.Utilisateur{
		UUID:           uuid.NewString(),
		Email:          email,
		MotDePasseHash: hache,
		Nom:            strings.TrimSpace(entree.Nom),
		Prenom:         strings.TrimSpace(entree.Prenom),
		Role:           models.RoleMembre, // rôle non négociable à l'inscription
		Actif:          true,
	}
	if err := s.utilisateurs.Creer(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// Connexion vérifie les identifiants et renvoie une paire de jetons + le profil.
//
// SÉCURITÉ : en cas d'e-mail inconnu OU de mot de passe erroné, on renvoie le
// MÊME message générique (« Identifiants invalides »). Révéler « e-mail inconnu »
// permettrait d'énumérer les comptes existants.
func (s *AuthService) Connexion(ctx context.Context, entree models.ConnexionEntree) (*models.ReponseConnexion, error) {
	v := validation.Nouveau()
	v.ChampRequis("email", entree.Email)
	v.ChampRequis("mot_de_passe", entree.MotDePasse)
	if err := v.Erreur(); err != nil {
		return nil, err
	}

	email := strings.ToLower(strings.TrimSpace(entree.Email))
	u, err := s.utilisateurs.ParEmail(ctx, email)
	if err != nil {
		if apperreur.EstCode(err, apperreur.CodeNonTrouve) {
			return nil, apperreur.NonAuthentifie("Identifiants invalides.")
		}
		return nil, err
	}

	if !auth.VerifierMotDePasse(u.MotDePasseHash, entree.MotDePasse) {
		return nil, apperreur.NonAuthentifie("Identifiants invalides.")
	}
	if !u.Actif {
		return nil, apperreur.Interdit("Ce compte est désactivé.")
	}

	paire, err := s.genererPaireDeJetons(ctx, u)
	if err != nil {
		return nil, err
	}
	return &models.ReponseConnexion{Utilisateur: u, Jetons: paire}, nil
}

// Rafraichir échange un refresh token valide contre une NOUVELLE paire de jetons.
//
// On applique la ROTATION des refresh tokens : l'ancien est immédiatement révoqué
// et remplacé. Ainsi, si un refresh token fuite, sa réutilisation par un attaquant
// après un usage légitime échouera (il aura déjà été consommé).
func (s *AuthService) Rafraichir(ctx context.Context, jetonRafraichissementClair string) (*models.PaireDeJetons, error) {
	if strings.TrimSpace(jetonRafraichissementClair) == "" {
		return nil, apperreur.RequeteInvalide("Jeton de rafraîchissement manquant.")
	}

	hache := auth.HacherJeton(jetonRafraichissementClair)
	utilisateurID, err := s.jetons.TrouverUtilisateurValide(ctx, hache)
	if err != nil {
		return nil, err // 401 si invalide/expiré/révoqué
	}

	u, err := s.utilisateurs.ParID(ctx, utilisateurID)
	if err != nil {
		return nil, err
	}
	if !u.Actif {
		return nil, apperreur.Interdit("Ce compte est désactivé.")
	}

	// Rotation : on invalide l'ancien jeton avant d'en émettre un nouveau.
	if err := s.jetons.Revoquer(ctx, hache); err != nil {
		return nil, err
	}

	paire, err := s.genererPaireDeJetons(ctx, u)
	if err != nil {
		return nil, err
	}
	return &paire, nil
}

// Deconnexion révoque le refresh token fourni (l'utilisateur devra se
// reconnecter pour obtenir un nouveau jeton). Le jeton d'accès JWT, lui, reste
// techniquement valide jusqu'à sa courte expiration : c'est une limite connue
// des JWT sans liste de révocation, acceptable vu leur durée de vie réduite.
func (s *AuthService) Deconnexion(ctx context.Context, jetonRafraichissementClair string) error {
	if strings.TrimSpace(jetonRafraichissementClair) == "" {
		return apperreur.RequeteInvalide("Jeton de rafraîchissement manquant.")
	}
	return s.jetons.Revoquer(ctx, auth.HacherJeton(jetonRafraichissementClair))
}

// genererPaireDeJetons crée le jeton d'accès, le refresh token, stocke le haché
// du refresh en base et renvoie la paire prête à être transmise au client.
func (s *AuthService) genererPaireDeJetons(ctx context.Context, u *models.Utilisateur) (models.PaireDeJetons, error) {
	acces, expirationAcces, err := s.jwt.GenererJetonAcces(u)
	if err != nil {
		return models.PaireDeJetons{}, err
	}
	clair, hache, expirationRefresh, err := s.jwt.GenererJetonRafraichissement()
	if err != nil {
		return models.PaireDeJetons{}, err
	}
	if err := s.jetons.Enregistrer(ctx, u.ID, hache, expirationRefresh); err != nil {
		return models.PaireDeJetons{}, err
	}
	return models.PaireDeJetons{
		JetonAcces:            acces,
		JetonRafraichissement: clair,
		TypeJeton:             "Bearer",
		ExpireLe:              expirationAcces,
	}, nil
}
