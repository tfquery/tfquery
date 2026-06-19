package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Root        Root         `yaml:"root"`
	Subcommands []Subcommand `yaml:"subcommands"`
	Common      Common       `yaml:"common"`
}

type Common struct {
	Flags []Flag `yaml:"flags"`
}

type Root struct {
	Name        string        `yaml:"name"`
	Short       string        `yaml:"short"`
	Usage       string        `yaml:"usage"`
	Commands    []RootCommand `yaml:"commands"`
	GlobalFlags []Flag        `yaml:"global_flags"`
}

type Subcommand struct {
	ID          string    `yaml:"id"`
	Alias       string    `yaml:"alias,omitempty"`
	Short       string    `yaml:"short"`
	Description string    `yaml:"description"`
	Usage       string    `yaml:"usage"`
	Flags       []Flag    `yaml:"flags"`
	Examples    []Example `yaml:"examples"`
	Notes       []string  `yaml:"notes,omitempty"`
}

type Flag struct {
	ID          string `yaml:"id"`
	Syntax      string `yaml:"syntax"`
	Description string `yaml:"description"`
	Default     string `yaml:"default,omitempty"`
	More        string `yaml:"more,omitempty"`
}

type Example struct {
	Command     string `yaml:"command"`
	Description string `yaml:"description"`
}

type RootCommand struct {
	Names       string `yaml:"names"`
	Alias       string `yaml:"alias,omitempty"`
	Description string `yaml:"description"`
}

type TemplateData struct {
	Subcommand
	Date       string
	Version    string
	IDUpper    string
	UsageAlias string
}

type RootTemplateData struct {
	Root
	Date    string
	Version string
}

type Outputs struct {
	Template string
	Folder   string
	Prefix   string
	Suffix   string
}

func main() {
	if len(os.Args) < 2 {
		panic("usage: docsgen <docs-dir> [version]")
	}

	docs := os.Args[1]
	version := getVersion()
	if len(os.Args) > 2 && os.Args[2] != "" {
		version = os.Args[2]
	}

	file := filepath.Join(docs, "templates", "docs-tfq.yaml")
	data, _ := os.ReadFile(filepath.Clean(file))
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		panic(err)
	}

	date := time.Now().Format("January 2, 2006")

	if err := renderOutput(
		Outputs{Template: docs + "/templates/tfq.root.man.tmpl", Folder: docs + "/./man/share/man1/", Prefix: "", Suffix: ".1"},
		config.Root.Name,
		RootTemplateData{Root: config.Root, Date: date, Version: version},
	); err != nil {
		panic(err)
	}

	for _, sub := range config.Subcommands {
		mergedFlags := config.Common.Flags

		if len(sub.Flags) > 0 {
			mergedFlags = append(mergedFlags, sub.Flags...)
		}

		sort.Slice(mergedFlags, func(i, j int) bool {
			return mergedFlags[i].ID < mergedFlags[j].ID
		})
		sub.Flags = mergedFlags

		// Prepare template data
		metadata := TemplateData{
			Subcommand: sub,
			Date:       date,
			Version:    version,
			UsageAlias: usageWithAlias(sub.Usage, sub.ID, sub.Alias),
		}

		types := []Outputs{
			{Template: docs + "/templates/tfq.md.tmpl", Folder: docs + "/commands/", Suffix: ".md"},
			{Template: docs + "/templates/tfq.man.tmpl", Folder: docs + "/./man/share/man1/", Prefix: "tfq-", Suffix: ".1"},
			{Template: docs + "/templates/tfq.tldr.tmpl", Folder: docs + "/./tldr/", Prefix: "tfq-", Suffix: ".md"},
		}

		for _, t := range types {
			if err := renderOutput(t, sub.ID, metadata); err != nil {
				panic(err)
			}
		}
	}
}

func renderOutput(output Outputs, name string, metadata any) error {
	if err := os.MkdirAll(output.Folder, 0o750); err != nil {
		return err
	}

	path := output.Folder + output.Prefix + name + output.Suffix
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	fmt.Println("Generating", path)
	tmpl, err := template.ParseFiles(output.Template)
	if err != nil {
		return err
	}

	if err := tmpl.Execute(file, metadata); err != nil {
		return err
	}

	return nil
}

// getVersion returns the version string from git tags, stripping the leading
// "v" prefix. Falls back to "dev" if git describe fails.
func getVersion() string {
	out, err := exec.Command("git", "describe", "--tags", "--abbrev=0").Output()
	if err != nil {
		return "dev"
	}

	version := strings.TrimSpace(string(out))
	return strings.TrimPrefix(version, "v")
}

// usageWithAlias rewrites the first "tfq <id>" segment in a usage string to
// "tfq <alias>". It returns an empty string when alias is blank or when the
// usage string does not contain the required format.
func usageWithAlias(usage string, id string, alias string) string {
	if alias == "" {
		return ""
	}

	short := "tfq " + id
	aliased := "tfq " + alias

	if !strings.Contains(usage, short) {
		return ""
	}

	return strings.Replace(usage, short, aliased, 1)
}
