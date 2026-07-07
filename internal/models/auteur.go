package models

import "time"

// Auteur représente l'auteur d'un ou plusieurs livres.
//
// Un auteur est une entité indépendante : plusieurs livres peuvent partager le
// même auteur (relation « un auteur → plusieurs livres »). En base, la table
// livres porte une clé étrangère auteur_id vers auteurs.id.
type Auteur struct {
	ID            int64     `json:"-"`  // clé technique interne
	UUID          string    `json:"id"` // identifiant public
	Nom           string    `json:"nom"`
	Prenom        string    `json:"prenom"`
	Nationalite   string    `json:"nationalite,omitempty"`
	DateNaissance *string   `json:"date_naissance,omitempty"` // format "AAAA-MM-JJ", facultatif
	Biographie    string    `json:"biographie,omitempty"`
	CreeLe        time.Time `json:"cree_le"`
	ModifieLe     time.Time `json:"modifie_le"`
}

// CreerAuteurEntree décrit les champs acceptés à la création d'un auteur.
type CreerAuteurEntree struct {
	Nom           string  `json:"nom"`
	Prenom        string  `json:"prenom"`
	Nationalite   string  `json:"nationalite"`
	DateNaissance *string `json:"date_naissance"` // facultatif, format "AAAA-MM-JJ"
	Biographie    string  `json:"biographie"`
}

// MettreAJourAuteurEntree décrit une mise à jour complète (PUT) d'un auteur.
type MettreAJourAuteurEntree struct {
	Nom           string  `json:"nom"`
	Prenom        string  `json:"prenom"`
	Nationalite   string  `json:"nationalite"`
	DateNaissance *string `json:"date_naissance"`
	Biographie    string  `json:"biographie"`
}

// ModifierAuteurEntree décrit une mise à jour partielle (PATCH).
// Champs en pointeurs : nil => champ non fourni => inchangé.
type ModifierAuteurEntree struct {
	Nom           *string `json:"nom"`
	Prenom        *string `json:"prenom"`
	Nationalite   *string `json:"nationalite"`
	DateNaissance *string `json:"date_naissance"`
	Biographie    *string `json:"biographie"`
}
