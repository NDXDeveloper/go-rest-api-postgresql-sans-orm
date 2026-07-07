package service

import (
	"context"
	"strings"

	"github.com/exemple/api-bibliotheque/internal/apperreur"
	"github.com/exemple/api-bibliotheque/internal/models"
	"github.com/exemple/api-bibliotheque/internal/validation"
	"github.com/google/uuid"
)

// LivreService porte la logique métier des livres. Il dépend de TROIS
// repositories : celui des livres, mais aussi ceux des auteurs et catégories pour
// VÉRIFIER que les références fournies par le client existent réellement (en plus
// des clés étrangères SQL, on valide tôt et on renvoie un message clair).
type LivreService struct {
	livres     LivreRepo
	auteurs    AuteurRepo
	categories CategorieRepo
}

// NouveauLivreService assemble le service avec ses dépendances.
func NouveauLivreService(livres LivreRepo, auteurs AuteurRepo, categories CategorieRepo) *LivreService {
	return &LivreService{livres: livres, auteurs: auteurs, categories: categories}
}

// Lister renvoie une page de livres.
func (s *LivreService) Lister(ctx context.Context, params models.ParametresListe) ([]models.Livre, int, error) {
	return s.livres.Lister(ctx, params)
}

// Obtenir renvoie un livre (avec auteur/catégorie) par identifiant public.
func (s *LivreService) Obtenir(ctx context.Context, uuidCible string) (*models.Livre, error) {
	if !validation.EstUUIDValide(uuidCible) {
		return nil, apperreur.RequeteInvalide("Identifiant de livre invalide.")
	}
	return s.livres.ParUUID(ctx, uuidCible)
}

// Creer crée un livre après validation et résolution des références.
func (s *LivreService) Creer(ctx context.Context, entree models.CreerLivreEntree) (*models.Livre, error) {
	v := validation.Nouveau()
	v.ChampRequis("titre", entree.Titre)
	v.LongueurMax("titre", entree.Titre, 255)
	v.ChampRequis("isbn", entree.ISBN)
	v.ISBN13("isbn", entree.ISBN)
	v.ChampRequis("auteur_id", entree.AuteurID)
	v.ChampRequis("categorie_id", entree.CategorieID)
	v.EntierDans("annee_publication", entree.AnneePublication, 1400, 2200)
	v.EntierDans("nombre_exemplaires", entree.NombreExemplaires, 1, 100000)
	v.Verifier(entree.Prix >= 0, "prix", "le prix doit être positif ou nul")
	v.LongueurMax("langue", entree.Langue, 50)
	if err := v.Erreur(); err != nil {
		return nil, err
	}

	auteurID, err := s.resoudreAuteur(ctx, entree.AuteurID)
	if err != nil {
		return nil, err
	}
	categorieID, err := s.resoudreCategorie(ctx, entree.CategorieID)
	if err != nil {
		return nil, err
	}

	livre := &models.Livre{
		UUID:                   uuid.NewString(),
		Titre:                  strings.TrimSpace(entree.Titre),
		ISBN:                   validation.NormaliserISBN(entree.ISBN),
		AuteurID:               auteurID,
		CategorieID:            categorieID,
		AnneePublication:       entree.AnneePublication,
		NombreExemplaires:      entree.NombreExemplaires,
		ExemplairesDisponibles: entree.NombreExemplaires, // tous disponibles à la création
		Resume:                 entree.Resume,
		Prix:                   entree.Prix,
		Langue:                 langueOuDefaut(entree.Langue),
	}
	if err := s.livres.Creer(ctx, livre); err != nil {
		return nil, err
	}
	// On relit via la vue pour renvoyer les noms d'auteur/catégorie au client.
	return s.livres.ParUUID(ctx, livre.UUID)
}

// MettreAJour remplace les champs d'un livre (PUT).
func (s *LivreService) MettreAJour(ctx context.Context, uuidCible string, entree models.MettreAJourLivreEntree) (*models.Livre, error) {
	livre, err := s.livres.ParUUIDInterne(ctx, uuidCible)
	if err != nil {
		return nil, err
	}

	v := validation.Nouveau()
	v.ChampRequis("titre", entree.Titre)
	v.LongueurMax("titre", entree.Titre, 255)
	v.ChampRequis("isbn", entree.ISBN)
	v.ISBN13("isbn", entree.ISBN)
	v.ChampRequis("auteur_id", entree.AuteurID)
	v.ChampRequis("categorie_id", entree.CategorieID)
	v.EntierDans("annee_publication", entree.AnneePublication, 1400, 2200)
	v.EntierDans("nombre_exemplaires", entree.NombreExemplaires, 1, 100000)
	v.Verifier(entree.Prix >= 0, "prix", "le prix doit être positif ou nul")
	v.LongueurMax("langue", entree.Langue, 50)
	if err := v.Erreur(); err != nil {
		return nil, err
	}

	auteurID, err := s.resoudreAuteur(ctx, entree.AuteurID)
	if err != nil {
		return nil, err
	}
	categorieID, err := s.resoudreCategorie(ctx, entree.CategorieID)
	if err != nil {
		return nil, err
	}

	// Recalcul cohérent du stock disponible (voir ajusterStock).
	nouvelleDispo, err := ajusterStock(livre, entree.NombreExemplaires)
	if err != nil {
		return nil, err
	}

	livre.Titre = strings.TrimSpace(entree.Titre)
	livre.ISBN = validation.NormaliserISBN(entree.ISBN)
	livre.AuteurID = auteurID
	livre.CategorieID = categorieID
	livre.AnneePublication = entree.AnneePublication
	livre.NombreExemplaires = entree.NombreExemplaires
	livre.ExemplairesDisponibles = nouvelleDispo
	livre.Resume = entree.Resume
	livre.Prix = entree.Prix
	livre.Langue = langueOuDefaut(entree.Langue)

	if err := s.livres.MettreAJour(ctx, livre); err != nil {
		return nil, err
	}
	return s.livres.ParUUID(ctx, uuidCible)
}

// Modifier applique une mise à jour partielle (PATCH).
func (s *LivreService) Modifier(ctx context.Context, uuidCible string, entree models.ModifierLivreEntree) (*models.Livre, error) {
	livre, err := s.livres.ParUUIDInterne(ctx, uuidCible)
	if err != nil {
		return nil, err
	}

	v := validation.Nouveau()
	if entree.Titre != nil {
		v.ChampRequis("titre", *entree.Titre)
		v.LongueurMax("titre", *entree.Titre, 255)
		livre.Titre = strings.TrimSpace(*entree.Titre)
	}
	if entree.ISBN != nil {
		v.ISBN13("isbn", *entree.ISBN)
		livre.ISBN = validation.NormaliserISBN(*entree.ISBN)
	}
	if entree.AnneePublication != nil {
		v.EntierDans("annee_publication", *entree.AnneePublication, 1400, 2200)
		livre.AnneePublication = *entree.AnneePublication
	}
	if entree.Prix != nil {
		v.Verifier(*entree.Prix >= 0, "prix", "le prix doit être positif ou nul")
		livre.Prix = *entree.Prix
	}
	if entree.Langue != nil {
		v.LongueurMax("langue", *entree.Langue, 50)
		livre.Langue = langueOuDefaut(*entree.Langue)
	}
	if entree.Resume != nil {
		livre.Resume = *entree.Resume
	}
	if entree.NombreExemplaires != nil {
		v.EntierDans("nombre_exemplaires", *entree.NombreExemplaires, 1, 100000)
		if v.EstValide() {
			nouvelleDispo, err := ajusterStock(livre, *entree.NombreExemplaires)
			if err != nil {
				return nil, err
			}
			livre.NombreExemplaires = *entree.NombreExemplaires
			livre.ExemplairesDisponibles = nouvelleDispo
		}
	}
	if err := v.Erreur(); err != nil {
		return nil, err
	}

	// Résolution des références seulement si elles changent.
	if entree.AuteurID != nil {
		auteurID, err := s.resoudreAuteur(ctx, *entree.AuteurID)
		if err != nil {
			return nil, err
		}
		livre.AuteurID = auteurID
	}
	if entree.CategorieID != nil {
		categorieID, err := s.resoudreCategorie(ctx, *entree.CategorieID)
		if err != nil {
			return nil, err
		}
		livre.CategorieID = categorieID
	}

	if err := s.livres.MettreAJour(ctx, livre); err != nil {
		return nil, err
	}
	return s.livres.ParUUID(ctx, uuidCible)
}

// SupprimerLogique masque un livre du catalogue (réversible).
func (s *LivreService) SupprimerLogique(ctx context.Context, uuidCible string) error {
	return s.livres.SupprimerLogique(ctx, uuidCible)
}

// SupprimerPhysique efface définitivement un livre (réservé aux admins).
func (s *LivreService) SupprimerPhysique(ctx context.Context, uuidCible string) error {
	return s.livres.SupprimerPhysique(ctx, uuidCible)
}

// --- Helpers internes ------------------------------------------------------

// resoudreAuteur vérifie qu'un auteur existe et renvoie sa clé interne. Une
// référence inexistante devient une erreur de VALIDATION ciblée sur le champ.
func (s *LivreService) resoudreAuteur(ctx context.Context, auteurUUID string) (int64, error) {
	if !validation.EstUUIDValide(auteurUUID) {
		return 0, apperreur.Validation("Référence invalide.", map[string]string{"auteur_id": "identifiant d'auteur invalide"})
	}
	a, err := s.auteurs.ParUUID(ctx, auteurUUID)
	if err != nil {
		if apperreur.EstCode(err, apperreur.CodeNonTrouve) {
			return 0, apperreur.Validation("Référence invalide.", map[string]string{"auteur_id": "cet auteur n'existe pas"})
		}
		return 0, err
	}
	return a.ID, nil
}

// resoudreCategorie vérifie qu'une catégorie existe et renvoie sa clé interne.
func (s *LivreService) resoudreCategorie(ctx context.Context, categorieUUID string) (int64, error) {
	if !validation.EstUUIDValide(categorieUUID) {
		return 0, apperreur.Validation("Référence invalide.", map[string]string{"categorie_id": "identifiant de catégorie invalide"})
	}
	c, err := s.categories.ParUUID(ctx, categorieUUID)
	if err != nil {
		if apperreur.EstCode(err, apperreur.CodeNonTrouve) {
			return 0, apperreur.Validation("Référence invalide.", map[string]string{"categorie_id": "cette catégorie n'existe pas"})
		}
		return 0, err
	}
	return c.ID, nil
}

// ajusterStock recalcule le nombre d'exemplaires disponibles quand le total
// change, en préservant le nombre d'exemplaires actuellement empruntés.
//
// Exemple : total 5, disponibles 3 => 2 empruntés. Si le nouveau total est 7,
// les disponibles passent à 5 (toujours 2 empruntés). On REFUSE de descendre le
// total sous le nombre d'exemplaires déjà empruntés (incohérent).
func ajusterStock(livre *models.Livre, nouveauTotal int) (int, error) {
	empruntes := livre.NombreExemplaires - livre.ExemplairesDisponibles
	if nouveauTotal < empruntes {
		return 0, apperreur.Conflit("Impossible : le nombre d'exemplaires ne peut être inférieur aux exemplaires actuellement empruntés.")
	}
	return nouveauTotal - empruntes, nil
}

// langueOuDefaut renvoie la langue fournie ou « français » par défaut.
func langueOuDefaut(langue string) string {
	if strings.TrimSpace(langue) == "" {
		return "français"
	}
	return strings.TrimSpace(langue)
}
