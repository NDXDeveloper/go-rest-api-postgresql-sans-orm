package models

import "time"

// Livre représente un ouvrage du catalogue.
//
// # Gestion du stock
//
//   - NombreExemplaires      : nombre total d'exemplaires possédés.
//   - ExemplairesDisponibles : nombre d'exemplaires actuellement empruntables.
//
// À chaque emprunt, ExemplairesDisponibles diminue ; à chaque retour, il
// augmente. Une contrainte CHECK en base garantit l'invariant
// 0 <= ExemplairesDisponibles <= NombreExemplaires (défense en profondeur).
//
// # Champs dénormalisés (Auteur..., Categorie...)
//
// Pour éviter au client de faire plusieurs appels, le repository joint les
// tables auteurs et categories et remplit les champs « AuteurNomComplet »,
// « CategorieNom », etc. Ces champs sont en LECTURE SEULE : ils ne sont pas
// modifiés directement mais reflètent l'état des tables liées.
type Livre struct {
	ID                     int64  `json:"-"`
	UUID                   string `json:"id"`
	Titre                  string `json:"titre"`
	ISBN                   string `json:"isbn"`
	AnneePublication       int    `json:"annee_publication"`
	NombreExemplaires      int    `json:"nombre_exemplaires"`
	ExemplairesDisponibles int    `json:"exemplaires_disponibles"`
	Resume                 string `json:"resume,omitempty"`
	// Prix en euros. ATTENTION : le type float64 n'est PAS idéal pour de la
	// monnaie (erreurs d'arrondi binaire). Pour un vrai système financier, on
	// stockerait des centimes en entier ou on utiliserait un type décimal.
	// On l'accepte ici pour rester simple, la précision n'étant pas critique.
	Prix   float64 `json:"prix"`
	Langue string  `json:"langue,omitempty"`

	CreeLe     time.Time  `json:"cree_le"`
	ModifieLe  time.Time  `json:"modifie_le"`
	SupprimeLe *time.Time `json:"supprime_le,omitempty"`

	// Clés étrangères internes (jamais exposées telles quelles).
	AuteurID    int64 `json:"-"`
	CategorieID int64 `json:"-"`

	// Données jointes, exposées au client (lecture seule).
	AuteurUUID       string `json:"auteur_id,omitempty"`
	AuteurNomComplet string `json:"auteur,omitempty"`
	CategorieUUID    string `json:"categorie_id,omitempty"`
	CategorieNom     string `json:"categorie,omitempty"`

	// Disponible est calculé (ExemplairesDisponibles > 0), pratique pour le client.
	Disponible bool `json:"disponible"`
}

// CreerLivreEntree décrit les champs acceptés à la création d'un livre.
//
// AuteurID et CategorieID sont les IDENTIFIANTS PUBLICS (UUID) : le service les
// traduit en clés internes après avoir vérifié que l'auteur et la catégorie
// existent (intégrité référentielle validée côté application EN PLUS des clés
// étrangères SQL).
type CreerLivreEntree struct {
	Titre             string  `json:"titre"`
	ISBN              string  `json:"isbn"`
	AuteurID          string  `json:"auteur_id"`
	CategorieID       string  `json:"categorie_id"`
	AnneePublication  int     `json:"annee_publication"`
	NombreExemplaires int     `json:"nombre_exemplaires"`
	Resume            string  `json:"resume"`
	Prix              float64 `json:"prix"`
	Langue            string  `json:"langue"`
}

// MettreAJourLivreEntree décrit une mise à jour complète (PUT).
type MettreAJourLivreEntree struct {
	Titre             string  `json:"titre"`
	ISBN              string  `json:"isbn"`
	AuteurID          string  `json:"auteur_id"`
	CategorieID       string  `json:"categorie_id"`
	AnneePublication  int     `json:"annee_publication"`
	NombreExemplaires int     `json:"nombre_exemplaires"`
	Resume            string  `json:"resume"`
	Prix              float64 `json:"prix"`
	Langue            string  `json:"langue"`
}

// ModifierLivreEntree décrit une mise à jour partielle (PATCH).
// Chaque champ est un pointeur : nil signifie « ne pas modifier ».
type ModifierLivreEntree struct {
	Titre             *string  `json:"titre"`
	ISBN              *string  `json:"isbn"`
	AuteurID          *string  `json:"auteur_id"`
	CategorieID       *string  `json:"categorie_id"`
	AnneePublication  *int     `json:"annee_publication"`
	NombreExemplaires *int     `json:"nombre_exemplaires"`
	Resume            *string  `json:"resume"`
	Prix              *float64 `json:"prix"`
	Langue            *string  `json:"langue"`
}
