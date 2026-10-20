package main

const (
	npmRegistry      = "https://registry.npmjs.org"
	unpkgBase        = "https://unpkg.com"
	htmxVersion      = "4.0.0"
	htmxSHA256       = "e484d9171a9db30a39c8f16e3d709d4137f3211c659f8e6125816635033d593f"
	cytoscapeVersion = "3.34.4"
	cytoscapeSHA256  = "081591ab735b2b6a3b31a3c60f8f3a3e2fee8c92939e0c7820ad1208571b6cd8"
	plantumlVersion  = "1.2026.8"
	plantumlSHA256   = "5e1ecfa8ecd32c90b03bbf3b1eb6f020943f98ab0fcf4032be31a0002ee2c462"
	plantumlURL      = "https://github.com/plantuml/plantuml/releases/download/v" + plantumlVersion + "/plantuml-" + plantumlVersion + ".jar"
	plantumlJarPath  = "data/tools/plantuml-" + plantumlVersion + ".jar"
)

type vendoredAsset struct {
	Pkg      string
	Version  string
	SHA256   string
	File     string
	DistURL  string
	VerConst string
	SHAConst string
}

func vendoredAssets() []vendoredAsset {
	return []vendoredAsset{
		{Pkg: "htmx.org", Version: htmxVersion, SHA256: htmxSHA256, File: "htmx.min.js", DistURL: unpkgBase + "/htmx.org@" + htmxVersion + "/dist/htmx.min.js", VerConst: "htmxVersion", SHAConst: "htmxSHA256"},
		{Pkg: "cytoscape", Version: cytoscapeVersion, SHA256: cytoscapeSHA256, File: "cytoscape.min.js", DistURL: unpkgBase + "/cytoscape@" + cytoscapeVersion + "/dist/cytoscape.min.js", VerConst: "cytoscapeVersion", SHAConst: "cytoscapeSHA256"},
	}
}
