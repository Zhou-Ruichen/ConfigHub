package web

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"path"
)

//go:embed assets/*
var assets embed.FS

// PageData is the common shape passed to server-rendered pages.
type PageData struct {
	Title string
	Nav   []NavItem
	Data  any
}

type NavItem struct {
	Href  string
	Label string
}

func Render(w io.Writer, page string, data PageData) error {
	if data.Nav == nil {
		data.Nav = []NavItem{{Href: "/", Label: "Status"}, {Href: "/profiles", Label: "Profiles"}, {Href: "/warnings", Label: "Warnings"}}
	}
	name := path.Base(page)
	t, err := template.ParseFS(assets, "assets/layout.html.tmpl", "assets/"+name+".html.tmpl")
	if err != nil {
		return fmt.Errorf("parse web template %q: %w", name, err)
	}
	return t.ExecuteTemplate(w, "layout", data)
}

func Asset(name string) []byte {
	data, _ := assets.ReadFile("assets/" + path.Base(name))
	return data
}
