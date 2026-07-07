// Package repository contient la couche d'accès aux données. C'est la SEULE
// couche autorisée à écrire du SQL. Les handlers et services n'en contiennent
// jamais : cette séparation garde le SQL centralisé, testable et auditable.
//
// # Principes appliqués dans TOUS les repositories
//
//  1. Requêtes PRÉPARÉES paramétrées : les valeurs passent par des paramètres
//     positionnels « $1, $2... » (syntaxe PostgreSQL) et sont envoyées séparément
//     de la requête au serveur. C'est la parade n°1 contre l'injection SQL. On ne
//     concatène JAMAIS une valeur utilisateur dans une requête.
//
//     DIFFÉRENCE AVEC MariaDB : MariaDB utilise des points d'interrogation « ? »
//     anonymes, tandis que PostgreSQL utilise des paramètres NUMÉROTÉS « $1, $2 ».
//     C'est l'une des adaptations de dialecte les plus visibles entre les deux.
//
//  2. context.Context sur chaque appel : permet d'annuler une requête si le
//     client se déconnecte ou si un délai est dépassé (voir le middleware Timeout).
//
//  3. Traduction des erreurs SQL en erreurs métier (apperreur) : aucune erreur
//     technique brute ne remonte vers le client (pas de fuite d'information).
package repository

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/exemple/api-bibliotheque/internal/models"
)

// ligneScannable est satisfaite à la fois par *sql.Row (résultat d'une ligne) et
// *sql.Rows (curseur multi-lignes). Grâce à elle, on écrit UNE seule fonction de
// scan par entité, réutilisée pour « lire un » comme pour « lire plusieurs ».
type ligneScannable interface {
	Scan(destinations ...any) error
}

// constructeurConditions assemble progressivement la clause WHERE d'une requête
// de liste (filtres, recherche...) tout en collectant les arguments et en leur
// attribuant un numéro de placeholder « $N » dans l'ordre.
//
// Il garantit que les valeurs restent des paramètres préparés : on n'insère
// jamais une valeur directement dans la chaîne SQL.
type constructeurConditions struct {
	conditions []string
	args       []any
}

// ajouter enregistre une condition SQL et ses arguments. On écrit la condition
// avec des « ? » (comme repère), et ajouter les remplace par les placeholders
// PostgreSQL numérotés « $N » correspondants, dans l'ordre.
//
//	c.ajouter("email = ?", email)                 -> "email = $1"
//	c.ajouter("role IN (?, ?)", r1, r2)           -> "role IN ($2, $3)"
func (c *constructeurConditions) ajouter(condition string, args ...any) {
	for _, arg := range args {
		numero := len(c.args) + 1
		condition = strings.Replace(condition, "?", "$"+strconv.Itoa(numero), 1)
		c.args = append(c.args, arg)
	}
	c.conditions = append(c.conditions, condition)
}

// clauseWHERE renvoie « WHERE a AND b AND ... » ou une chaîne vide s'il n'y a
// aucune condition.
func (c *constructeurConditions) clauseWHERE() string {
	if len(c.conditions) == 0 {
		return ""
	}
	return "WHERE " + strings.Join(c.conditions, " AND ")
}

// clauseTriEtPagination construit la fin d'une requête de liste :
// « ORDER BY <colonne> <sens> LIMIT $N OFFSET $M ».
//
// SÉCURITÉ : params.ColonneTri est un nom de colonne DÉJÀ VALIDÉ contre une liste
// blanche par le handler (voir models.ParametresListe et le parsing HTTP). Il est
// donc sûr de l'interpoler ici. En dernier recours, si la colonne est vide, on
// applique un tri par défaut : jamais de valeur non maîtrisée dans « ORDER BY ».
//
// Les numéros de placeholder de LIMIT et OFFSET CONTINUENT la numérotation des
// conditions déjà présentes (d'où le paramètre `cond`) : si le WHERE utilise déjà
// $1..$k, alors LIMIT devient $(k+1) et OFFSET $(k+2).
func clauseTriEtPagination(params models.ParametresListe, colonneParDefaut string, cond *constructeurConditions) (string, []any) {
	colonne := params.ColonneTri
	if colonne == "" {
		colonne = colonneParDefaut
	}
	sens := params.Ordre
	if sens != models.OrdreAsc && sens != models.OrdreDesc {
		sens = models.OrdreAsc
	}
	numeroLimit := len(cond.args) + 1
	numeroOffset := len(cond.args) + 2
	clause := fmt.Sprintf(" ORDER BY %s %s LIMIT $%d OFFSET $%d", colonne, sens, numeroLimit, numeroOffset)
	return clause, []any{params.Taille, params.Offset()}
}

// dateVersChaine convertit une colonne DATE nullable (lue en sql.NullTime) vers
// un *string au format « AAAA-MM-JJ », ou nil si la valeur est NULL.
//
// Pourquoi ? Une colonne DATE seule (sans heure) est plus lisible en JSON sous la
// forme "1802-02-26" que "1802-02-26T00:00:00Z" (ce que donnerait un time.Time).
func dateVersChaine(valeur sql.NullTime) *string {
	if !valeur.Valid {
		return nil
	}
	s := valeur.Time.Format("2006-01-02")
	return &s
}

// argDate prépare une date facultative (*string) pour un paramètre SQL : renvoie
// la chaîne si elle est renseignée, sinon nil (qui devient NULL en base). Le
// pilote convertit automatiquement "AAAA-MM-JJ" en type DATE.
func argDate(valeur *string) any {
	if valeur == nil || *valeur == "" {
		return nil
	}
	return *valeur
}
