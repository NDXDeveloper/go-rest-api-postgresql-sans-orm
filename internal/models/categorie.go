package models

import "time"

// Categorie regroupe les livres par thème (Roman, Science-fiction, Histoire...).
//
// Relation : un livre appartient à une catégorie (livres.categorie_id →
// categories.id). Une catégorie peut contenir plusieurs livres.
type Categorie struct {
	ID          int64     `json:"-"`
	UUID        string    `json:"id"`
	Nom         string    `json:"nom"`
	Description string    `json:"description,omitempty"`
	CreeLe      time.Time `json:"cree_le"`
	ModifieLe   time.Time `json:"modifie_le"`
}

// CreerCategorieEntree décrit les champs acceptés à la création.
type CreerCategorieEntree struct {
	Nom         string `json:"nom"`
	Description string `json:"description"`
}

// MettreAJourCategorieEntree décrit une mise à jour complète (PUT).
type MettreAJourCategorieEntree struct {
	Nom         string `json:"nom"`
	Description string `json:"description"`
}

// ModifierCategorieEntree décrit une mise à jour partielle (PATCH).
type ModifierCategorieEntree struct {
	Nom         *string `json:"nom"`
	Description *string `json:"description"`
}
