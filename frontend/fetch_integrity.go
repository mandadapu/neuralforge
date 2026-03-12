package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type Dist struct {
	Integrity string `json:"integrity"`
}

type Pkg struct {
	Dist Dist `json:"dist"`
}

func fetch(pkg, version string) string {
	url := fmt.Sprintf("https://registry.npmjs.org/%s/%s", pkg, version)
	resp, err := http.Get(url)
	if err != nil {
		return "ERROR: " + err.Error()
	}
	defer resp.Body.Close()
	var p Pkg
	json.NewDecoder(resp.Body).Decode(&p)
	return p.Dist.Integrity
}

func main() {
	packages := [][2]string{
		{"minimatch", "10.0.1"},
		{"brace-expansion", "2.0.1"},
		{"balanced-match", "1.0.2"},
	}
	for _, p := range packages {
		fmt.Printf("%s@%s: %s\n", p[0], p[1], fetch(p[0], p[1]))
	}
}
