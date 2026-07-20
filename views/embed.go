package views

import "embed"

// FS contiene todas las plantillas HTML embebidas directamente en el binario compilado de Go
//go:embed *
var FS embed.FS
