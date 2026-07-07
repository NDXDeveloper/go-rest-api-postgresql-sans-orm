package models

import "time"

// StatutEmprunt énumère les états possibles d'un emprunt.
type StatutEmprunt string

const (
	// StatutEnCours : le livre est emprunté et pas encore rendu, dans les délais.
	StatutEnCours StatutEmprunt = "en_cours"
	// StatutRendu : le livre a été restitué.
	StatutRendu StatutEmprunt = "rendu"
	// StatutEnRetard : le livre n'est pas rendu et la date prévue est dépassée.
	// Ce passage « en_cours » → « en_retard » est effectué automatiquement par
	// une tâche pg_cron quotidienne (voir sql/cron).
	StatutEnRetard StatutEmprunt = "en_retard"
)

// Emprunt représente le prêt d'un livre à un utilisateur.
//
// C'est l'entité centrale qui illustre le mieux les fonctionnalités avancées du
// projet :
//   - sa création passe par une PROCÉDURE STOCKÉE transactionnelle
//     (pr_emprunter_livre) qui vérifie la disponibilité et le quota ;
//   - son retour passe par une TRANSACTION écrite en Go (verrouillage FOR UPDATE,
//     mise à jour de l'emprunt ET du stock, commit/rollback) ;
//   - des TRIGGERS journalisent chaque changement dans journal_audit ;
//   - des EVENTS le font passer « en_retard » et l'archivent après un an.
type Emprunt struct {
	ID                  int64         `json:"-"`
	UUID                string        `json:"id"`
	DateEmprunt         time.Time     `json:"date_emprunt"`
	DateRetourPrevue    time.Time     `json:"date_retour_prevue"`
	DateRetourEffective *time.Time    `json:"date_retour_effective,omitempty"`
	Statut              StatutEmprunt `json:"statut"`
	// Penalite en euros due en cas de retard (0 si rendu à temps). Voir la
	// remarque sur float64 et la monnaie dans le modèle Livre.
	Penalite  float64   `json:"penalite"`
	CreeLe    time.Time `json:"cree_le"`
	ModifieLe time.Time `json:"modifie_le"`

	// Clés étrangères internes.
	UtilisateurID int64 `json:"-"`
	LivreID       int64 `json:"-"`

	// Données jointes exposées au client (lecture seule).
	UtilisateurUUID       string `json:"utilisateur_id,omitempty"`
	UtilisateurNomComplet string `json:"utilisateur,omitempty"`
	LivreUUID             string `json:"livre_id,omitempty"`
	LivreTitre            string `json:"livre,omitempty"`
}

// StatistiquesUtilisateur agrège les indicateurs d'emprunt d'un utilisateur.
// Ce type est alimenté par la procédure stockée pr_statistiques_utilisateur
// (qui renvoie ses résultats via des paramètres OUT).
type StatistiquesUtilisateur struct {
	NbTotal        int     `json:"nb_total"`
	NbEnCours      int     `json:"nb_en_cours"`
	NbEnRetard     int     `json:"nb_en_retard"`
	TotalPenalites float64 `json:"total_penalites"`
}

// EmprunterEntree décrit le corps d'une demande d'emprunt.
//
// L'identité de l'emprunteur n'est PAS dans le corps : elle provient du jeton
// JWT (un membre emprunte pour lui-même). Un bibliothécaire peut, lui, préciser
// un utilisateur via UtilisateurID.
type EmprunterEntree struct {
	LivreID    string `json:"livre_id"`    // UUID du livre à emprunter
	DureeJours int    `json:"duree_jours"` // durée du prêt (défaut : 14 jours)
	// UtilisateurID est optionnel et réservé aux bibliothécaires/admins qui
	// enregistrent un emprunt pour le compte d'un membre. Ignoré pour un membre.
	UtilisateurID string `json:"utilisateur_id,omitempty"`
}
