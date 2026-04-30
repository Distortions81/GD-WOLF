package wl6

import "embed"

//go:embed shareware/*.WL1
var embeddedShareware embed.FS

func OpenEmbeddedShareware() (*Files, error) {
	return openFromFS(embeddedShareware, "shareware", variantWL1)
}
