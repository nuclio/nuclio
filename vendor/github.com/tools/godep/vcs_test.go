package main

import "testing"

func TestGitDetermineDefaultBranch(t *testing.T) {
	cases := []struct {
		r, o string
		v    string
		err  bool
	}{
		{"test",
			`* remote origin
  Fetch URL: https://gopkg.in/mgo.v2
  Push  URL: https://gopkg.in/mgo.v2
  HEAD branch: v2
  Remote branches:
    master      tracked
    v2          tracked
    v2-unstable tracked
  Local branches configured for 'git pull':
    master merges with remote master
    v2     merges with remote v2
  Local refs configured for 'git push':
    master pushes to master (up to date)
    v2     pushes to v2     (local out of date)
`, "v2", false},
		{"test",
			`* remote origin
  Fetch URL: https://gopkg.in/bluesuncorp/validator.v5
  Push  URL: https://gopkg.in/bluesuncorp/validator.v5
  HEAD branch (remote HEAD is ambiguous, may be one of the following):
    master
    v5
  Remote branches:
    krhubert       tracked
    master         tracked
    v4             tracked
    v5             tracked
    v5-development tracked
    v6             tracked
    v6-development tracked
    v7             tracked
    v7-development tracked
    v8             tracked
    v8-development tracked
  Local branch configured for 'git pull':
    master merges with remote master
  Local ref configured for 'git push':
    master pushes to master (up to date)
`, "master", false},
		{"test",
			`* remote origin
  Fetch URL: https://github.com/gin-gonic/gin
  Push  URL: https://github.com/gin-gonic/gin
  HEAD branch: develop
  Remote branches:
    benchmarks            tracked
    better-bind-errors    tracked
    develop               tracked
    fasthttp              tracked
    fix-binding           tracked
    fix-tests             tracked
    gh-pages              tracked
    honteng-bind_test     tracked
    master                tracked
    new-binding-validator tracked
    new-catch-all         tracked
    performance           tracked
    routes-list           tracked
  Local branch configured for 'git pull':
    develop merges with remote develop
  Local ref configured for 'git push':
    develop pushes to develop (local out of date)
`, "develop", false},
		{"test", "", "", true},
	}

	for i, test := range cases {
		v, e := gitDetermineDefaultBranch(test.r, test.o)
		if v != test.v {
			t.Errorf("%d Unexpected value returned: %s, wanted %s", i, v, test.v)
		}
		if e != nil {
			t.Log("Err", e.Error())
		}
		if test.err && e == nil {
			t.Errorf("%d Test should err, but didn't", i)
		}

		if !test.err && e != nil {
			t.Errorf("%d Test shouldn't err, but did with: %s", i, e)
		}
	}
}
