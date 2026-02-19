package handler

import (
	"bytes"
	"html/template"
	"testing"

	"github.com/seuusuario/factorydev/web"
)

func TestDoctorTemplateRendersFullPage(t *testing.T) {
	tpl := template.New("root")
	_, err := tpl.ParseFS(web.FS,
		"templates/layout.html",
		"templates/partials/sidebar.html",
		"templates/partials/drawer.html",
		"templates/doctor.html",
	)
	if err != nil {
		t.Fatalf("parse templates: %v", err)
	}

	data := PageData{
		Title:      "FactoryDev",
		ActiveTool: "ssh",
		ContentTpl: "doctor.html",
		Data:       map[string]any{"Checks": []map[string]any{{"Name": "x", "OK": true, "Message": "OK"}}},
	}

	var out bytes.Buffer
	if err := tpl.ExecuteTemplate(&out, "layout.html", data); err != nil {
		t.Fatalf("execute layout: %v", err)
	}
}

func TestDoctorTemplateRendersPartial(t *testing.T) {
	tpl := template.New("root")
	_, err := tpl.ParseFS(web.FS, "templates/doctor.html")
	if err != nil {
		t.Fatalf("parse templates: %v", err)
	}

	var out bytes.Buffer
	if err := tpl.ExecuteTemplate(&out, "doctor.html", map[string]any{"Checks": []map[string]any{{"Name": "x", "OK": true, "Message": "OK"}}}); err != nil {
		t.Fatalf("execute partial: %v", err)
	}
}

func TestReposTemplateRendersFullPage(t *testing.T) {
	tpl := template.New("root")
	_, err := tpl.ParseFS(web.FS,
		"templates/layout.html",
		"templates/partials/sidebar.html",
		"templates/partials/drawer.html",
		"templates/repos/list.html",
	)
	if err != nil {
		t.Fatalf("parse templates: %v", err)
	}

	data := PageData{
		Title:      "FactoryDev",
		ActiveTool: "repos",
		ContentTpl: "repos/list.html",
		Data: map[string]any{
			"Accounts": []map[string]any{
				{"ID": "1", "Name": "work", "HostAlias": "github-work", "HasKey": true},
			},
			"DefaultDest": "/tmp",
		},
	}

	var out bytes.Buffer
	if err := tpl.ExecuteTemplate(&out, "layout.html", data); err != nil {
		t.Fatalf("execute layout: %v", err)
	}
}

func TestReposTemplateRendersPartial(t *testing.T) {
	tpl := template.New("root")
	_, err := tpl.ParseFS(web.FS, "templates/repos/list.html")
	if err != nil {
		t.Fatalf("parse templates: %v", err)
	}

	var out bytes.Buffer
	if err := tpl.ExecuteTemplate(&out, "repos/list.html", map[string]any{
		"Accounts":    []map[string]any{{"ID": "1", "Name": "work", "HostAlias": "github-work", "HasKey": true}},
		"DefaultDest": "/tmp",
	}); err != nil {
		t.Fatalf("execute partial: %v", err)
	}
}
