package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestUpdate(t *testing.T) {
	var cases = []struct {
		cwd    string
		args   []string
		vendor bool
		start  []*node
		want   []*node
		wdep   Godeps
		werr   bool
	}{
		{ // 0 - simple case, update one dependency
			cwd:  "C",
			args: []string{"D"},
			start: []*node{
				{
					"D",
					"",
					[]*node{
						{"main.go", pkg("D") + decl("D1"), nil},
						{"+git", "D1", nil},
						{"main.go", pkg("D") + decl("D2"), nil},
						{"+git", "D2", nil},
					},
				},
				{
					"C",
					"",
					[]*node{
						{"main.go", pkg("main", "D"), nil},
						{"Godeps/Godeps.json", godeps("C", "D", "D1"), nil},
						{"Godeps/_workspace/src/D/main.go", pkg("D") + decl("D1"), nil},
						{"+git", "", nil},
					},
				},
			},
			want: []*node{
				{"C/Godeps/_workspace/src/D/main.go", pkg("D") + decl("D2"), nil},
			},
			wdep: Godeps{
				ImportPath: "C",
				Deps: []Dependency{
					{ImportPath: "D", Comment: "D2"},
				},
			},
		},
		{ // 1 - simple case, update one dependency, trailing slash
			cwd:  "C",
			args: []string{"D/"},
			start: []*node{
				{
					"D",
					"",
					[]*node{
						{"main.go", pkg("D") + decl("D1"), nil},
						{"+git", "D1", nil},
						{"main.go", pkg("D") + decl("D2"), nil},
						{"+git", "D2", nil},
					},
				},
				{
					"C",
					"",
					[]*node{
						{"main.go", pkg("main", "D"), nil},
						{"Godeps/Godeps.json", godeps("C", "D", "D1"), nil},
						{"Godeps/_workspace/src/D/main.go", pkg("D") + decl("D1"), nil},
						{"+git", "", nil},
					},
				},
			},
			want: []*node{
				{"C/Godeps/_workspace/src/D/main.go", pkg("D") + decl("D2"), nil},
			},
			wdep: Godeps{
				ImportPath: "C",
				Deps: []Dependency{
					{ImportPath: "D", Comment: "D2"},
				},
			},
		},
		{ // 2 - update one dependency, keep other one, no rewrite
			cwd:  "C",
			args: []string{"D"},
			start: []*node{
				{
					"D",
					"",
					[]*node{
						{"main.go", pkg("D", "E") + decl("D1"), nil},
						{"+git", "D1", nil},
						{"main.go", pkg("D", "E") + decl("D2"), nil},
						{"+git", "D2", nil},
					},
				},
				{
					"E",
					"",
					[]*node{
						{"main.go", pkg("E") + decl("E1"), nil},
						{"+git", "E1", nil},
						{"main.go", pkg("E") + decl("E2"), nil},
						{"+git", "E2", nil},
					},
				},
				{
					"C",
					"",
					[]*node{
						{"main.go", pkg("main", "D", "E"), nil},
						{"Godeps/Godeps.json", godeps("C", "D", "D1", "E", "E1"), nil},
						{"Godeps/_workspace/src/D/main.go", pkg("D", "E") + decl("D1"), nil},
						{"Godeps/_workspace/src/E/main.go", pkg("E") + decl("E1"), nil},
						{"+git", "", nil},
					},
				},
			},
			want: []*node{
				{"C/Godeps/_workspace/src/D/main.go", pkg("D", "E") + decl("D2"), nil},
				{"C/Godeps/_workspace/src/E/main.go", pkg("E") + decl("E1"), nil},
			},
			wdep: Godeps{
				ImportPath: "C",
				Deps: []Dependency{
					{ImportPath: "D", Comment: "D2"},
					{ImportPath: "E", Comment: "E1"},
				},
			},
		},
		{ // 3 - update one dependency, keep other one, with rewrite
			cwd:  "C",
			args: []string{"D"},
			start: []*node{
				{
					"D",
					"",
					[]*node{
						{"main.go", pkg("D", "E") + decl("D1"), nil},
						{"+git", "D1", nil},
						{"main.go", pkg("D", "E") + decl("D2"), nil},
						{"+git", "D2", nil},
					},
				},
				{
					"E",
					"",
					[]*node{
						{"main.go", pkg("E") + decl("E1"), nil},
						{"+git", "E1", nil},
						{"main.go", pkg("E") + decl("E2"), nil},
						{"+git", "E2", nil},
					},
				},
				{
					"C",
					"",
					[]*node{
						{"main.go", pkg("main", "C/Godeps/_workspace/src/D", "C/Godeps/_workspace/src/E"), nil},
						{"Godeps/Godeps.json", godeps("C", "D", "D1", "E", "E1"), nil},
						{"Godeps/_workspace/src/D/main.go", pkg("D", "C/Godeps/_workspace/src/E") + decl("D1"), nil},
						{"Godeps/_workspace/src/E/main.go", pkg("E") + decl("E1"), nil},
						{"+git", "", nil},
					},
				},
			},
			want: []*node{
				{"C/main.go", pkg("main", "C/Godeps/_workspace/src/D", "C/Godeps/_workspace/src/E"), nil},
				{"C/Godeps/_workspace/src/D/main.go", pkg("D", "C/Godeps/_workspace/src/E") + "\n" + decl("D2"), nil},
				{"C/Godeps/_workspace/src/E/main.go", pkg("E") + decl("E1"), nil},
			},
			wdep: Godeps{
				ImportPath: "C",
				Deps: []Dependency{
					{ImportPath: "D", Comment: "D2"},
					{ImportPath: "E", Comment: "E1"},
				},
			},
		},
		{ // 4 - update all dependencies
			cwd:  "C",
			args: []string{"..."},
			start: []*node{
				{
					"D",
					"",
					[]*node{
						{"main.go", pkg("D") + decl("D1"), nil},
						{"+git", "D1", nil},
						{"main.go", pkg("D") + decl("D2"), nil},
						{"+git", "D2", nil},
					},
				},
				{
					"E",
					"",
					[]*node{
						{"main.go", pkg("E") + decl("E1"), nil},
						{"+git", "E1", nil},
						{"main.go", pkg("E") + decl("E2"), nil},
						{"+git", "E2", nil},
					},
				},
				{
					"C",
					"",
					[]*node{
						{"main.go", pkg("main", "D", "E"), nil},
						{"Godeps/Godeps.json", godeps("C", "D", "D1", "E", "E1"), nil},
						{"Godeps/_workspace/src/D/main.go", pkg("D") + decl("D1"), nil},
						{"Godeps/_workspace/src/E/main.go", pkg("E") + decl("E1"), nil},
						{"+git", "", nil},
					},
				},
			},
			want: []*node{
				{"C/Godeps/_workspace/src/D/main.go", pkg("D") + decl("D2"), nil},
				{"C/Godeps/_workspace/src/E/main.go", pkg("E") + decl("E2"), nil},
			},
			wdep: Godeps{
				ImportPath: "C",
				Deps: []Dependency{
					{ImportPath: "D", Comment: "D2"},
					{ImportPath: "E", Comment: "E2"},
				},
			},
		},
		{ // 5 - one match of two patterns
			cwd:  "C",
			args: []string{"D", "X"},
			start: []*node{
				{
					"D",
					"",
					[]*node{
						{"main.go", pkg("D") + decl("D1"), nil},
						{"+git", "D1", nil},
						{"main.go", pkg("D") + decl("D2"), nil},
						{"+git", "D2", nil},
					},
				},
				{
					"C",
					"",
					[]*node{
						{"main.go", pkg("main", "D"), nil},
						{"Godeps/Godeps.json", godeps("C", "D", "D1"), nil},
						{"Godeps/_workspace/src/D/main.go", pkg("D") + decl("D1"), nil},
						{"+git", "", nil},
					},
				},
			},
			want: []*node{
				{"C/Godeps/_workspace/src/D/main.go", pkg("D") + decl("D2"), nil},
			},
			wdep: Godeps{
				ImportPath: "C",
				Deps: []Dependency{
					{ImportPath: "D", Comment: "D2"},
				},
			},
		},
		{ // 6 - no matches
			cwd:  "C",
			args: []string{"X"},
			start: []*node{
				{
					"D",
					"",
					[]*node{
						{"main.go", pkg("D") + decl("D1"), nil},
						{"+git", "D1", nil},
						{"main.go", pkg("D") + decl("D2"), nil},
						{"+git", "D2", nil},
					},
				},
				{
					"C",
					"",
					[]*node{
						{"main.go", pkg("main", "D"), nil},
						{"Godeps/Godeps.json", godeps("C", "D", "D1"), nil},
						{"Godeps/_workspace/src/D/main.go", pkg("D") + decl("D1"), nil},
						{"+git", "", nil},
					},
				},
			},
			want: []*node{
				{"C/Godeps/_workspace/src/D/main.go", pkg("D") + decl("D1"), nil},
			},
			wdep: Godeps{
				ImportPath: "C",
				Deps: []Dependency{
					{ImportPath: "D", Comment: "D1"},
				},
			},
			werr: true,
		},
		{ // 7 - update just one package of two in a repo skips it
			cwd:  "C",
			args: []string{"D/A", "E"},
			start: []*node{
				{
					"D",
					"",
					[]*node{
						{"A/main.go", pkg("A") + decl("D1"), nil},
						{"B/main.go", pkg("B") + decl("D1"), nil},
						{"+git", "D1", nil},
						{"A/main.go", pkg("A") + decl("D2"), nil},
						{"B/main.go", pkg("B") + decl("D2"), nil},
						{"+git", "D2", nil},
					},
				},
				{
					"E",
					"",
					[]*node{
						{"main.go", pkg("E") + decl("E1"), nil},
						{"+git", "E1", nil},
						{"main.go", pkg("E") + decl("E2"), nil},
						{"+git", "E2", nil},
					},
				},
				{
					"C",
					"",
					[]*node{
						{"main.go", pkg("main", "D/A", "D/B", "E"), nil},
						{"Godeps/Godeps.json", godeps("C", "D/A", "D1", "D/B", "D1", "E", "E1"), nil},
						{"Godeps/_workspace/src/D/A/main.go", pkg("A") + decl("D1"), nil},
						{"Godeps/_workspace/src/D/B/main.go", pkg("B") + decl("D1"), nil},
						{"Godeps/_workspace/src/E/main.go", pkg("E") + decl("E1"), nil},
						{"+git", "", nil},
					},
				},
			},
			want: []*node{
				{"C/Godeps/_workspace/src/D/A/main.go", pkg("A") + decl("D1"), nil},
				{"C/Godeps/_workspace/src/D/B/main.go", pkg("B") + decl("D1"), nil},
				{"C/Godeps/_workspace/src/E/main.go", pkg("E") + decl("E2"), nil},
			},
			wdep: Godeps{
				ImportPath: "C",
				Deps: []Dependency{
					{ImportPath: "D/A", Comment: "D1"},
					{ImportPath: "D/B", Comment: "D1"},
					{ImportPath: "E", Comment: "E2"},
				},
			},
		},
		{ // 8 - update just one package of two in a repo, none left
			cwd:  "C",
			args: []string{"D/A"},
			start: []*node{
				{
					"D",
					"",
					[]*node{
						{"A/main.go", pkg("A") + decl("D1"), nil},
						{"B/main.go", pkg("B") + decl("D1"), nil},
						{"+git", "D1", nil},
						{"A/main.go", pkg("A") + decl("D2"), nil},
						{"B/main.go", pkg("B") + decl("D2"), nil},
						{"+git", "D2", nil},
					},
				},
				{
					"C",
					"",
					[]*node{
						{"main.go", pkg("main", "D/A", "D/B"), nil},
						{"Godeps/Godeps.json", godeps("C", "D/A", "D1", "D/B", "D1"), nil},
						{"Godeps/_workspace/src/D/A/main.go", pkg("A") + decl("D1"), nil},
						{"Godeps/_workspace/src/D/B/main.go", pkg("B") + decl("D1"), nil},
						{"+git", "", nil},
					},
				},
			},
			want: []*node{
				{"C/Godeps/_workspace/src/D/A/main.go", pkg("A") + decl("D1"), nil},
				{"C/Godeps/_workspace/src/D/B/main.go", pkg("B") + decl("D1"), nil},
			},
			wdep: Godeps{
				ImportPath: "C",
				Deps: []Dependency{
					{ImportPath: "D/A", Comment: "D1"},
					{ImportPath: "D/B", Comment: "D1"},
				},
			},
			werr: true,
		},
		{ // 9 - package/..., just version bump
			vendor: true,
			cwd:    "C",
			args:   []string{"D/..."},
			start: []*node{
				{
					"D",
					"",
					[]*node{
						{"A/main.go", pkg("A") + decl("D1"), nil},
						{"B/main.go", pkg("B") + decl("D1"), nil},
						{"+git", "D1", nil},
						{"A/main.go", pkg("A") + decl("D2"), nil},
						{"B/main.go", pkg("B") + decl("D2"), nil},
						{"+git", "D2", nil},
					},
				},
				{
					"C",
					"",
					[]*node{
						{"main.go", pkg("main", "D/A", "D/B"), nil},
						{"Godeps/Godeps.json", godeps("C", "D/A", "D1", "D/B", "D1"), nil},
						{"vendor/D/A/main.go", pkg("A") + decl("D1"), nil},
						{"vendor/D/B/main.go", pkg("B") + decl("D1"), nil},
						{"+git", "", nil},
					},
				},
			},
			want: []*node{
				{"C/vendor/D/A/main.go", pkg("A") + decl("D2"), nil},
				{"C/vendor/D/B/main.go", pkg("B") + decl("D2"), nil},
			},
			wdep: Godeps{
				ImportPath: "C",
				Deps: []Dependency{
					{ImportPath: "D/A", Comment: "D2"},
					{ImportPath: "D/B", Comment: "D2"},
				},
			},
		},
		{ // 10 - package/..., new unrelated package that's not imported
			vendor: true,
			cwd:    "C",
			args:   []string{"D/..."},
			start: []*node{
				{
					"D",
					"",
					[]*node{
						{"A/main.go", pkg("A") + decl("D1"), nil},
						{"B/main.go", pkg("B") + decl("D1"), nil},
						{"+git", "D1", nil},
						{"A/main.go", pkg("A") + decl("D2"), nil},
						{"B/main.go", pkg("B") + decl("D2"), nil},
						{"E/main.go", pkg("E") + decl("D2"), nil},
						{"+git", "D2", nil},
					},
				},
				{
					"C",
					"",
					[]*node{
						{"main.go", pkg("main", "D/A", "D/B"), nil},
						{"Godeps/Godeps.json", godeps("C", "D/A", "D1", "D/B", "D1"), nil},
						{"vendor/D/A/main.go", pkg("A") + decl("D1"), nil},
						{"vendor/D/B/main.go", pkg("B") + decl("D1"), nil},
						{"+git", "", nil},
					},
				},
			},
			want: []*node{
				{"C/vendor/D/A/main.go", pkg("A") + decl("D2"), nil},
				{"C/vendor/D/B/main.go", pkg("B") + decl("D2"), nil},
				{"C/vendor/D/E/main.go", "(absent)", nil},
			},
			wdep: Godeps{
				ImportPath: "C",
				Deps: []Dependency{
					{ImportPath: "D/A", Comment: "D2"},
					{ImportPath: "D/B", Comment: "D2"},
				},
			},
		},
		{ // 11 - package/..., new transitive package, same repo
			vendor: true,
			cwd:    "C",
			args:   []string{"D/..."},
			start: []*node{
				{
					"D",
					"",
					[]*node{
						{"A/main.go", pkg("A") + decl("D1"), nil},
						{"B/main.go", pkg("B") + decl("D1"), nil},
						{"+git", "D1", nil},
						{"A/main.go", pkg("A") + decl("D2"), nil},
						{"B/main.go", pkg("B", "D/E") + decl("D2"), nil},
						{"E/main.go", pkg("E") + decl("D2"), nil},
						{"+git", "D2", nil},
					},
				},
				{
					"C",
					"",
					[]*node{
						{"main.go", pkg("main", "D/A", "D/B"), nil},
						{"Godeps/Godeps.json", godeps("C", "D/A", "D1", "D/B", "D1"), nil},
						{"vendor/D/A/main.go", pkg("A") + decl("D1"), nil},
						{"vendor/D/B/main.go", pkg("B") + decl("D1"), nil},
						{"+git", "", nil},
					},
				},
			},
			want: []*node{
				{"C/vendor/D/A/main.go", pkg("A") + decl("D2"), nil},
				{"C/vendor/D/B/main.go", pkg("B", "D/E") + decl("D2"), nil},
				{"C/vendor/D/E/main.go", pkg("E") + decl("D2"), nil},
			},
			wdep: Godeps{
				ImportPath: "C",
				Deps: []Dependency{
					{ImportPath: "D/A", Comment: "D2"},
					{ImportPath: "D/B", Comment: "D2"},
					{ImportPath: "D/E", Comment: "D2"},
				},
			},
		},
		{ // 12 - package/..., new transitive package, different repo
			vendor: true,
			cwd:    "C",
			args:   []string{"D/..."},
			start: []*node{
				{
					"D",
					"",
					[]*node{
						{"A/main.go", pkg("A") + decl("D1"), nil},
						{"B/main.go", pkg("B") + decl("D1"), nil},
						{"+git", "D1", nil},
						{"A/main.go", pkg("A") + decl("D2"), nil},
						{"B/main.go", pkg("B", "E") + decl("D2"), nil},
						{"+git", "D2", nil},
					},
				},
				{
					"E",
					"",
					[]*node{
						{"main.go", pkg("E") + decl("E1"), nil},
						{"+git", "E1", nil},
					},
				},
				{
					"C",
					"",
					[]*node{
						{"main.go", pkg("main", "D/A", "D/B"), nil},
						{"Godeps/Godeps.json", godeps("C", "D/A", "D1", "D/B", "D1"), nil},
						{"vendor/D/A/main.go", pkg("A") + decl("D1"), nil},
						{"vendor/D/B/main.go", pkg("B") + decl("D1"), nil},
						{"+git", "", nil},
					},
				},
			},
			want: []*node{
				{"C/vendor/D/A/main.go", pkg("A") + decl("D2"), nil},
				{"C/vendor/D/B/main.go", pkg("B", "E") + decl("D2"), nil},
				{"C/vendor/E/main.go", pkg("E") + decl("E1"), nil},
			},
			wdep: Godeps{
				ImportPath: "C",
				Deps: []Dependency{
					{ImportPath: "D/A", Comment: "D2"},
					{ImportPath: "D/B", Comment: "D2"},
					{ImportPath: "E", Comment: "E1"},
				},
			},
		},
		{ // 13 - package/..., missing packages
			vendor: true,
			cwd:    "C",
			args:   []string{"D/..."},
			start: []*node{
				{
					"D",
					"",
					[]*node{
						{"A/main.go", pkg("A") + decl("D1"), nil},
						{"B/main.go", pkg("B") + decl("D1"), nil},
						{"+git", "D1", nil},
					},
				},
				{
					"C",
					"",
					[]*node{
						{"main.go", pkg("main", "D/A"), nil},
						{"Godeps/Godeps.json", godeps("C", "D/A", "D1", "D/B", "D1"), nil},
						{"vendor/D/A/main.go", pkg("A") + decl("D1"), nil},
						{"vendor/D/B/main.go", pkg("B") + decl("D1"), nil},
						{"+git", "", nil},
					},
				},
				{"D",
					"",
					[]*node{
						{"A/main.go", pkg("A") + decl("D2"), nil},
						{"B", "(rm)", nil},
						{"+git", "D2", nil},
					},
				},
			},
			want: []*node{
				{"C/vendor/D/A/main.go", pkg("A") + decl("D2"), nil},
				{"C/vendor/D/B/main.go", "(absent)", nil},
				{"C/vendor/D/E/main.go", "(absent)", nil},
			},
			wdep: Godeps{
				ImportPath: "C",
				Deps: []Dependency{
					{ImportPath: "D/A", Comment: "D2"},
				},
			},
		},
		{ // 14 - Update package A, but not package B, which is missing from $GOPATH
			vendor: true,
			cwd:    "C",
			args:   []string{"A"},
			start: []*node{
				{
					"A",
					"",
					[]*node{
						{"main.go", pkg("A") + decl("A1"), nil},
						{"+git", "A1", nil},
						{"main.go", pkg("A") + decl("A2"), nil},
						{"+git", "A2", nil},
					},
				},
				{ // Create B so makeTree can resolve the rev for Godeps.json
					"B",
					"",
					[]*node{
						{"main.go", pkg("B") + decl("B1"), nil},
						{"+git", "B1", nil},
					},
				},
				{
					"C",
					"",
					[]*node{
						{"main.go", pkg("main", "A", "B"), nil},
						{"Godeps/Godeps.json", godeps("C", "A", "A1", "B", "B1"), nil},
						{"vendor/A/main.go", pkg("A") + decl("A1"), nil},
						{"vendor/B/main.go", pkg("B") + decl("B1"), nil},
						{"+git", "", nil},
					},
				},
				{ // Remove B so it's not in the $GOPATH
					"",
					"",
					[]*node{
						{"B", "(rm)", nil},
					},
				},
			},
			want: []*node{
				{"C/vendor/A/main.go", pkg("A") + decl("A2"), nil},
				{"C/vendor/B/main.go", pkg("B") + decl("B1"), nil},
			},
			wdep: Godeps{
				ImportPath: "C",
				Deps: []Dependency{
					{ImportPath: "A", Comment: "A2"},
					{ImportPath: "B", Comment: "B1"},
				},
			},
		},
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	const gopath = "godeptest"
	defer os.RemoveAll(gopath)
	for pos, test := range cases {
		setGlobals(test.vendor)
		err = os.RemoveAll(gopath)
		if err != nil {
			t.Fatal(err)
		}
		src := filepath.Join(gopath, "src")
		makeTree(t, &node{src, "", test.start}, "")

		dir := filepath.Join(wd, src, test.cwd)
		err = os.Chdir(dir)
		if err != nil {
			panic(err)
		}
		setGOPATH(filepath.Join(wd, gopath))
		log.SetOutput(ioutil.Discard)
		err = update(test.args)
		log.SetOutput(os.Stderr)
		if err != nil {
			t.Log(pos, "Err:", err)
		}
		if g := err != nil; g != test.werr {
			t.Errorf("update err = %v (%v) want %v", g, err, test.werr)
		}
		err = os.Chdir(wd)
		if err != nil {
			panic(err)
		}

		checkTree(t, pos, &node{src, "", test.want})

		f, err := os.Open(filepath.Join(dir, "Godeps/Godeps.json"))
		if err != nil {
			t.Error(err)
		}
		g := new(Godeps)
		err = json.NewDecoder(f).Decode(g)
		if err != nil {
			t.Error(err)
		}
		f.Close()

		if g.ImportPath != test.wdep.ImportPath {
			t.Errorf("ImportPath = %s want %s", g.ImportPath, test.wdep.ImportPath)
		}
		for i := range g.Deps {
			g.Deps[i].Rev = ""
		}
		if !reflect.DeepEqual(g.Deps, test.wdep.Deps) {
			t.Errorf("Deps = %v want %v", g.Deps, test.wdep.Deps)
		}
	}
}
