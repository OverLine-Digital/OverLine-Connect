// Package webui embarque et sert le dashboard web statique (une seule page
// HTML/JS/CSS, sans étape de build). Embarqué directement dans le binaire
// Go via go:embed — pas de dossier à copier au déploiement.
package webui

import _ "embed"

//go:embed dashboard.html
var DashboardHTML []byte
