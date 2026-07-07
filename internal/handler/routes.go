package handler

import (
	"log/slog"
	"net/http"

	"github.com/exemple/api-bibliotheque/internal/auth"
	"github.com/exemple/api-bibliotheque/internal/middleware"
	"github.com/exemple/api-bibliotheque/internal/models"
	"github.com/exemple/api-bibliotheque/internal/observabilite"
)

// Dependances regroupe tout ce dont le routeur a besoin. On les injecte depuis
// main.go (injection de dépendances manuelle). Aucun état global.
type Dependances struct {
	Auth            *AuthHandler
	Utilisateur     *UtilisateurHandler
	Auteur          *AuteurHandler
	Categorie       *CategorieHandler
	Livre           *LivreHandler
	Emprunt         *EmpruntHandler
	Sante           *SanteHandler
	GestionnaireJWT *auth.GestionnaireJWT
	Metriques       *observabilite.Metriques
	Logger          *slog.Logger
}

// ConstruireRouteur crée le multiplexeur (ServeMux) et enregistre toutes les
// routes avec leur niveau de protection. Il renvoie un http.Handler prêt à être
// enveloppé par les middlewares GLOBAUX (voir main.go).
//
// On s'appuie sur le routeur standard net/http.ServeMux (Go 1.22+), qui gère
// nativement les patrons « MÉTHODE /chemin/{parametre} » — pas besoin de
// dépendance externe. Les paramètres se lisent avec r.PathValue("parametre").
func ConstruireRouteur(d Dependances) http.Handler {
	mux := http.NewServeMux()

	// Middlewares d'accès réutilisables.
	authentifier := middleware.Authentification(d.GestionnaireJWT, d.Logger)
	// exiger(roles...) = être authentifié ET posséder l'un des rôles.
	exiger := func(roles ...models.Role) middleware.Middleware {
		return middleware.Chainer(authentifier, middleware.ExigerRole(d.Logger, roles...))
	}

	const admin = models.RoleAdmin
	const biblio = models.RoleBibliothecaire

	// ---------------------------------------------------------------------
	// Observabilité (public, pas d'authentification).
	// ---------------------------------------------------------------------
	mux.HandleFunc("GET /health", d.Sante.Vivant)
	mux.HandleFunc("GET /ready", d.Sante.Pret)
	mux.Handle("GET /metrics", d.Metriques.Handler())

	// ---------------------------------------------------------------------
	// Authentification (public).
	// ---------------------------------------------------------------------
	mux.HandleFunc("POST /api/v1/auth/inscription", d.Auth.Inscription)
	mux.HandleFunc("POST /api/v1/auth/connexion", d.Auth.Connexion)
	mux.HandleFunc("POST /api/v1/auth/rafraichir", d.Auth.Rafraichir)
	mux.HandleFunc("POST /api/v1/auth/deconnexion", d.Auth.Deconnexion)

	// ---------------------------------------------------------------------
	// Profil personnel (authentifié, tout rôle).
	// ---------------------------------------------------------------------
	mux.Handle("GET /api/v1/moi", authentifier(http.HandlerFunc(d.Utilisateur.MonProfil)))
	mux.Handle("PATCH /api/v1/moi", authentifier(http.HandlerFunc(d.Utilisateur.ModifierMonProfil)))
	mux.Handle("GET /api/v1/moi/emprunts", authentifier(http.HandlerFunc(d.Emprunt.MesEmprunts)))
	mux.Handle("GET /api/v1/moi/statistiques", authentifier(http.HandlerFunc(d.Emprunt.MesStatistiques)))

	// ---------------------------------------------------------------------
	// Catalogue — LECTURE publique.
	// ---------------------------------------------------------------------
	mux.HandleFunc("GET /api/v1/livres", d.Livre.Lister)
	mux.HandleFunc("GET /api/v1/livres/{id}", d.Livre.Obtenir)
	mux.HandleFunc("GET /api/v1/auteurs", d.Auteur.Lister)
	mux.HandleFunc("GET /api/v1/auteurs/{id}", d.Auteur.Obtenir)
	mux.HandleFunc("GET /api/v1/categories", d.Categorie.Lister)
	mux.HandleFunc("GET /api/v1/categories/{id}", d.Categorie.Obtenir)

	// ---------------------------------------------------------------------
	// Catalogue — ÉCRITURE (bibliothécaire ou admin).
	// ---------------------------------------------------------------------
	ecritureCatalogue := exiger(admin, biblio)
	mux.Handle("POST /api/v1/livres", ecritureCatalogue(http.HandlerFunc(d.Livre.Creer)))
	mux.Handle("PUT /api/v1/livres/{id}", ecritureCatalogue(http.HandlerFunc(d.Livre.Remplacer)))
	mux.Handle("PATCH /api/v1/livres/{id}", ecritureCatalogue(http.HandlerFunc(d.Livre.Modifier)))
	mux.Handle("DELETE /api/v1/livres/{id}", ecritureCatalogue(http.HandlerFunc(d.Livre.Supprimer)))

	mux.Handle("POST /api/v1/auteurs", ecritureCatalogue(http.HandlerFunc(d.Auteur.Creer)))
	mux.Handle("PUT /api/v1/auteurs/{id}", ecritureCatalogue(http.HandlerFunc(d.Auteur.Remplacer)))
	mux.Handle("PATCH /api/v1/auteurs/{id}", ecritureCatalogue(http.HandlerFunc(d.Auteur.Modifier)))
	mux.Handle("DELETE /api/v1/auteurs/{id}", ecritureCatalogue(http.HandlerFunc(d.Auteur.Supprimer)))

	mux.Handle("POST /api/v1/categories", ecritureCatalogue(http.HandlerFunc(d.Categorie.Creer)))
	mux.Handle("PUT /api/v1/categories/{id}", ecritureCatalogue(http.HandlerFunc(d.Categorie.Remplacer)))
	mux.Handle("PATCH /api/v1/categories/{id}", ecritureCatalogue(http.HandlerFunc(d.Categorie.Modifier)))
	mux.Handle("DELETE /api/v1/categories/{id}", ecritureCatalogue(http.HandlerFunc(d.Categorie.Supprimer)))

	// ---------------------------------------------------------------------
	// Emprunts.
	//   - Emprunter / Rendre : tout utilisateur authentifié (pour lui-même).
	//   - Consultation globale : bibliothécaire ou admin.
	// ---------------------------------------------------------------------
	mux.Handle("POST /api/v1/emprunts", authentifier(http.HandlerFunc(d.Emprunt.Emprunter)))
	mux.Handle("POST /api/v1/emprunts/{id}/retour", authentifier(http.HandlerFunc(d.Emprunt.Rendre)))
	mux.Handle("GET /api/v1/emprunts", exiger(admin, biblio)(http.HandlerFunc(d.Emprunt.Lister)))
	mux.Handle("GET /api/v1/emprunts/{id}", exiger(admin, biblio)(http.HandlerFunc(d.Emprunt.Obtenir)))

	// ---------------------------------------------------------------------
	// Gestion des utilisateurs (administrateur uniquement).
	// ---------------------------------------------------------------------
	adminSeul := exiger(admin)
	mux.Handle("GET /api/v1/utilisateurs", adminSeul(http.HandlerFunc(d.Utilisateur.Lister)))
	mux.Handle("POST /api/v1/utilisateurs", adminSeul(http.HandlerFunc(d.Utilisateur.Creer)))
	mux.Handle("GET /api/v1/utilisateurs/{id}", adminSeul(http.HandlerFunc(d.Utilisateur.Obtenir)))
	mux.Handle("PUT /api/v1/utilisateurs/{id}", adminSeul(http.HandlerFunc(d.Utilisateur.Remplacer)))
	mux.Handle("PATCH /api/v1/utilisateurs/{id}", adminSeul(http.HandlerFunc(d.Utilisateur.Modifier)))
	mux.Handle("DELETE /api/v1/utilisateurs/{id}", adminSeul(http.HandlerFunc(d.Utilisateur.Supprimer)))

	return mux
}
