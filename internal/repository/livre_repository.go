package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/exemple/api-bibliotheque/internal/apperreur"
	"github.com/exemple/api-bibliotheque/internal/database"
	"github.com/exemple/api-bibliotheque/internal/models"
)

// LivreRepository gère les accès SQL à la table livres.
//
// Particularité pédagogique : les LECTURES s'appuient sur la VUE
// vue_livres_details (qui joint auteurs et categories et calcule la
// disponibilité), tandis que les ÉCRITURES ciblent la table livres directement.
//
// # Le type NUMERIC et le prix
//
// Le prix est stocké en NUMERIC (précision décimale exacte, sans erreur d'arrondi
// binaire — le bon choix pour de la monnaie). Comme le modèle Go expose un float64
// pour l'API, on convertit explicitement le NUMERIC en « double precision »
// (::float8) dans les SELECT : le pilote pgx sait alors le lire sans ambiguïté
// dans un float64. Le stockage, lui, reste en NUMERIC.
type LivreRepository struct {
	db *sql.DB
}

// NouveauLivreRepository construit le repository avec sa dépendance.
func NouveauLivreRepository(db *sql.DB) *LivreRepository {
	return &LivreRepository{db: db}
}

// colonnesVueLivre suit l'ordre exact des colonnes de vue_livres_details.
// (La vue expose déjà prix en double precision et disponible en booléen.)
const colonnesVueLivre = `id, uuid, titre, isbn, annee_publication, nombre_exemplaires,
	exemplaires_disponibles, disponible, prix, langue, resume,
	auteur_uuid, auteur_nom_complet, categorie_uuid, categorie_nom, cree_le, modifie_le`

// scannerLivreVue lit une ligne de la vue vue_livres_details (données d'affichage).
func scannerLivreVue(ligne ligneScannable) (*models.Livre, error) {
	var l models.Livre
	var resume sql.NullString
	err := ligne.Scan(
		&l.ID, &l.UUID, &l.Titre, &l.ISBN, &l.AnneePublication, &l.NombreExemplaires,
		&l.ExemplairesDisponibles, &l.Disponible, &l.Prix, &l.Langue, &resume,
		&l.AuteurUUID, &l.AuteurNomComplet, &l.CategorieUUID, &l.CategorieNom, &l.CreeLe, &l.ModifieLe,
	)
	if err != nil {
		return nil, err
	}
	l.Resume = resume.String
	return &l, nil
}

// colonnesTableLivre suit l'ordre exact des colonnes lues dans la table livres.
// On caste prix en double precision pour le scan Go (voir la remarque de type).
const colonnesTableLivre = `id, uuid, titre, isbn, auteur_id, categorie_id, annee_publication,
	nombre_exemplaires, exemplaires_disponibles, resume, prix::float8, langue, cree_le, modifie_le, supprime_le`

// scannerLivreTable lit une ligne de la table livres (avec les clés internes
// auteur_id/categorie_id), nécessaire pour les mises à jour.
func scannerLivreTable(ligne ligneScannable) (*models.Livre, error) {
	var l models.Livre
	var resume sql.NullString
	err := ligne.Scan(
		&l.ID, &l.UUID, &l.Titre, &l.ISBN, &l.AuteurID, &l.CategorieID, &l.AnneePublication,
		&l.NombreExemplaires, &l.ExemplairesDisponibles, &resume, &l.Prix, &l.Langue,
		&l.CreeLe, &l.ModifieLe, &l.SupprimeLe,
	)
	if err != nil {
		return nil, err
	}
	l.Resume = resume.String
	l.Disponible = l.ExemplairesDisponibles > 0
	return &l, nil
}

// Creer insère un livre. Les clés auteur_id/categorie_id ont été résolues et
// validées par le service à partir des UUID fournis par le client.
func (r *LivreRepository) Creer(ctx context.Context, l *models.Livre) error {
	const requete = `INSERT INTO livres
		(uuid, titre, isbn, auteur_id, categorie_id, annee_publication,
		 nombre_exemplaires, exemplaires_disponibles, resume, prix, langue)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id`
	err := r.db.QueryRowContext(ctx, requete,
		l.UUID, l.Titre, l.ISBN, l.AuteurID, l.CategorieID, l.AnneePublication,
		l.NombreExemplaires, l.ExemplairesDisponibles, l.Resume, l.Prix, l.Langue).Scan(&l.ID)
	if err != nil {
		if database.EstErreurDoublon(err) {
			return apperreur.Conflit("Un livre avec cet ISBN existe déjà.")
		}
		return apperreur.Interne("création du livre").AvecCause(err)
	}
	return nil
}

// ParUUID récupère un livre pour AFFICHAGE depuis la vue (avec auteur/catégorie).
func (r *LivreRepository) ParUUID(ctx context.Context, uuid string) (*models.Livre, error) {
	const requete = `SELECT ` + colonnesVueLivre + ` FROM vue_livres_details WHERE uuid = $1`
	l, err := scannerLivreVue(r.db.QueryRowContext(ctx, requete, uuid))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperreur.NonTrouve("Livre introuvable.")
		}
		return nil, apperreur.Interne("lecture du livre").AvecCause(err)
	}
	return l, nil
}

// ParUUIDInterne récupère un livre depuis la TABLE (avec les clés internes),
// pour préparer une mise à jour.
func (r *LivreRepository) ParUUIDInterne(ctx context.Context, uuid string) (*models.Livre, error) {
	const requete = `SELECT ` + colonnesTableLivre + ` FROM livres WHERE uuid = $1 AND supprime_le IS NULL`
	l, err := scannerLivreTable(r.db.QueryRowContext(ctx, requete, uuid))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apperreur.NonTrouve("Livre introuvable.")
		}
		return nil, apperreur.Interne("lecture interne du livre").AvecCause(err)
	}
	return l, nil
}

// Lister renvoie une page de livres (depuis la vue) et le total. Prend en charge
// la recherche par titre et les filtres par catégorie, auteur et disponibilité.
func (r *LivreRepository) Lister(ctx context.Context, params models.ParametresListe) ([]models.Livre, int, error) {
	var conditions constructeurConditions
	if params.Recherche != "" {
		conditions.ajouter("titre ILIKE ?", "%"+params.Recherche+"%")
	}
	if cat := params.Filtres["categorie"]; cat != "" {
		conditions.ajouter("categorie_uuid = ?", cat)
	}
	if aut := params.Filtres["auteur"]; aut != "" {
		conditions.ajouter("auteur_uuid = ?", aut)
	}
	if params.Filtres["disponible"] == "true" {
		conditions.ajouter("disponible = TRUE")
	}
	where := conditions.clauseWHERE()

	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM vue_livres_details `+where, conditions.args...).Scan(&total); err != nil {
		return nil, 0, apperreur.Interne("comptage des livres").AvecCause(err)
	}
	if total == 0 {
		return []models.Livre{}, 0, nil
	}

	triPagination, argsPagination := clauseTriEtPagination(params, "titre", &conditions)
	//nolint:gosec // G202 : concaténation sûre — 'where' n'utilise que des paramètres '$N' et 'triPagination' une colonne validée par liste blanche.
	requete := `SELECT ` + colonnesVueLivre + ` FROM vue_livres_details ` + where + triPagination
	lignes, err := r.db.QueryContext(ctx, requete, append(conditions.args, argsPagination...)...)
	if err != nil {
		return nil, 0, apperreur.Interne("liste des livres").AvecCause(err)
	}
	defer lignes.Close()

	livres := make([]models.Livre, 0, params.Taille)
	for lignes.Next() {
		l, err := scannerLivreVue(lignes)
		if err != nil {
			return nil, 0, apperreur.Interne("lecture d'une ligne livre").AvecCause(err)
		}
		livres = append(livres, *l)
	}
	if err := lignes.Err(); err != nil {
		return nil, 0, apperreur.Interne("parcours des livres").AvecCause(err)
	}
	return livres, total, nil
}

// MettreAJour réécrit les champs d'un livre. Un ISBN en doublon renvoie 409 ;
// une incohérence de stock détectée par le trigger (RAISE EXCEPTION) renvoie 409
// avec le message métier fourni par la base.
func (r *LivreRepository) MettreAJour(ctx context.Context, l *models.Livre) error {
	const requete = `UPDATE livres
		SET titre = $1, isbn = $2, auteur_id = $3, categorie_id = $4, annee_publication = $5,
		    nombre_exemplaires = $6, exemplaires_disponibles = $7, resume = $8, prix = $9, langue = $10
		WHERE uuid = $11 AND supprime_le IS NULL`
	resultat, err := r.db.ExecContext(ctx, requete,
		l.Titre, l.ISBN, l.AuteurID, l.CategorieID, l.AnneePublication,
		l.NombreExemplaires, l.ExemplairesDisponibles, l.Resume, l.Prix, l.Langue, l.UUID)
	if err != nil {
		if database.EstErreurDoublon(err) {
			return apperreur.Conflit("Un livre avec cet ISBN existe déjà.")
		}
		// Le trigger trg_livres_avant_update peut lever une exception (RAISE) :
		// son message est en français et destiné à l'utilisateur, donc exposable.
		if message, ok := database.MessageSignal(err); ok {
			return apperreur.Conflit(message)
		}
		return apperreur.Interne("mise à jour du livre").AvecCause(err)
	}
	return verifierLigneAffectee(resultat, "Livre introuvable.")
}

// SupprimerLogique horodate supprime_le : le livre disparaît du catalogue mais
// reste en base (l'historique des emprunts est préservé).
func (r *LivreRepository) SupprimerLogique(ctx context.Context, uuid string) error {
	const requete = `UPDATE livres SET supprime_le = NOW() WHERE uuid = $1 AND supprime_le IS NULL`
	resultat, err := r.db.ExecContext(ctx, requete, uuid)
	if err != nil {
		return apperreur.Interne("suppression logique du livre").AvecCause(err)
	}
	return verifierLigneAffectee(resultat, "Livre introuvable.")
}

// SupprimerPhysique efface définitivement le livre (bloqué par la FK si des
// emprunts le référencent : on renvoie alors un 409 explicite).
func (r *LivreRepository) SupprimerPhysique(ctx context.Context, uuid string) error {
	const requete = `DELETE FROM livres WHERE uuid = $1`
	resultat, err := r.db.ExecContext(ctx, requete, uuid)
	if err != nil {
		if database.EstErreurCleEtrangere(err) {
			return apperreur.Conflit("Impossible de supprimer ce livre : des emprunts y sont rattachés.")
		}
		return apperreur.Interne("suppression physique du livre").AvecCause(err)
	}
	return verifierLigneAffectee(resultat, "Livre introuvable.")
}
