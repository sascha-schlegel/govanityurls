// Copyright 2017 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
)

func TestHandler(t *testing.T) {
	tests := []struct {
		name   string
		config string
		path   string

		goImport string
		goSource string
	}{
		{
			name: "explicit display",
			config: "host: example.com\n" +
				"paths:\n" +
				"  /portmidi:\n" +
				"    repo: https://github.com/rakyll/portmidi\n" +
				"    display: https://github.com/rakyll/portmidi _ _\n",
			path:     "/portmidi",
			goImport: "example.com/portmidi git https://github.com/rakyll/portmidi",
			goSource: "example.com/portmidi https://github.com/rakyll/portmidi _ _",
		},
		{
			name: "display GitHub inference",
			config: "host: example.com\n" +
				"paths:\n" +
				"  /portmidi:\n" +
				"    repo: https://github.com/rakyll/portmidi\n",
			path:     "/portmidi",
			goImport: "example.com/portmidi git https://github.com/rakyll/portmidi",
			goSource: "example.com/portmidi https://github.com/rakyll/portmidi https://github.com/rakyll/portmidi/tree/master{/dir} https://github.com/rakyll/portmidi/blob/master{/dir}/{file}#L{line}",
		},
		{
			name: "Bitbucket Mercurial",
			config: "host: example.com\n" +
				"paths:\n" +
				"  /gopdf:\n" +
				"    repo: https://bitbucket.org/zombiezen/gopdf\n" +
				"    vcs: hg\n",
			path:     "/gopdf",
			goImport: "example.com/gopdf hg https://bitbucket.org/zombiezen/gopdf",
			goSource: "example.com/gopdf https://bitbucket.org/zombiezen/gopdf https://bitbucket.org/zombiezen/gopdf/src/default{/dir} https://bitbucket.org/zombiezen/gopdf/src/default{/dir}/{file}#{file}-{line}",
		},
		{
			name: "Bitbucket Git",
			config: "host: example.com\n" +
				"paths:\n" +
				"  /mygit:\n" +
				"    repo: https://bitbucket.org/zombiezen/mygit\n" +
				"    vcs: git\n",
			path:     "/mygit",
			goImport: "example.com/mygit git https://bitbucket.org/zombiezen/mygit",
			goSource: "example.com/mygit https://bitbucket.org/zombiezen/mygit https://bitbucket.org/zombiezen/mygit/src/default{/dir} https://bitbucket.org/zombiezen/mygit/src/default{/dir}/{file}#{file}-{line}",
		},
		{
			name: "subpath",
			config: "host: example.com\n" +
				"paths:\n" +
				"  /portmidi:\n" +
				"    repo: https://github.com/rakyll/portmidi\n" +
				"    display: https://github.com/rakyll/portmidi _ _\n",
			path:     "/portmidi/foo",
			goImport: "example.com/portmidi git https://github.com/rakyll/portmidi",
			goSource: "example.com/portmidi https://github.com/rakyll/portmidi _ _",
		},
		{
			name: "subpath with trailing config slash",
			config: "host: example.com\n" +
				"paths:\n" +
				"  /portmidi/:\n" +
				"    repo: https://github.com/rakyll/portmidi\n" +
				"    display: https://github.com/rakyll/portmidi _ _\n",
			path:     "/portmidi/foo",
			goImport: "example.com/portmidi git https://github.com/rakyll/portmidi",
			goSource: "example.com/portmidi https://github.com/rakyll/portmidi _ _",
		},
		{
			name: "path rules",
			config: "host: example.com\n" +
				"pathrules:\n" +
				"  /{name}:\n" +
				"    repo: https://github.com/rakyll/{name}\n" +
				"    display: https://github.com/rakyll/{name} _ _\n",
			path:     "/portmidi",
			goImport: "example.com/portmidi git https://github.com/rakyll/portmidi",
			goSource: "example.com/portmidi https://github.com/rakyll/portmidi _ _",
		},
		{
			name: "path rules come last",
			config: "host: example.com\n" +
				"paths:\n" +
				"  /portmidi/:\n" +
				"    repo: https://github.com/rakyll/modo\n" +
				"    display: https://github.com/rakyll/modo_ _\n" +
				"pathrules:\n" +
				"  /{name}:\n" +
				"    repo: https://github.com/rakyll/{name}\n" +
				"    display: https://github.com/rakyll/{name} _ _\n",
			path:     "/portmidi",
			goImport: "example.com/portmidi git https://github.com/rakyll/modo",
			goSource: "example.com/portmidi https://github.com/rakyll/modo_ _",
		},
	}
	for _, test := range tests {
		h, err := newHandler([]byte(test.config))
		if err != nil {
			t.Errorf("%s: newHandler: %v", test.name, err)
			continue
		}
		s := httptest.NewServer(h)
		resp, err := http.Get(s.URL + test.path)
		if err != nil {
			s.Close()
			t.Errorf("%s: http.Get: %v", test.name, err)
			continue
		}
		data, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		s.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s: status code = %s; want 200 OK", test.name, resp.Status)
		}
		if err != nil {
			t.Errorf("%s: ioutil.ReadAll: %v", test.name, err)
			continue
		}
		if got := findMeta(data, "go-import"); got != test.goImport {
			t.Errorf("%s: meta go-import = %q; want %q", test.name, got, test.goImport)
		}
		if got := findMeta(data, "go-source"); got != test.goSource {
			t.Errorf("%s: meta go-source = %q; want %q", test.name, got, test.goSource)
		}
	}
}

func TestBadConfigs(t *testing.T) {
	badConfigs := []string{
		"paths:\n" +
			"  /missingvcs:\n" +
			"    repo: https://bitbucket.org/zombiezen/gopdf\n",
		"paths:\n" +
			"  /unknownvcs:\n" +
			"    repo: https://bitbucket.org/zombiezen/gopdf\n" +
			"    vcs: xyzzy\n",
	}
	for _, config := range badConfigs {
		_, err := newHandler([]byte(config))
		if err == nil {
			t.Errorf("expected config to produce an error, but did not:\n%s", config)
		}
	}
}

func findMeta(data []byte, name string) string {
	var sep []byte
	sep = append(sep, `<meta name="`...)
	sep = append(sep, name...)
	sep = append(sep, `" content="`...)
	i := bytes.Index(data, sep)
	if i == -1 {
		return ""
	}
	content := data[i+len(sep):]
	j := bytes.IndexByte(content, '"')
	if j == -1 {
		return ""
	}
	return string(content[:j])
}

func TestPathConfigSetFind(t *testing.T) {
	tests := []struct {
		paths   []string
		query   string
		want    string
		subpath string
	}{
		{
			paths: []string{"/portmidi"},
			query: "/portmidi",
			want:  "/portmidi",
		},
		{
			paths: []string{"/portmidi"},
			query: "/portmidi/",
			want:  "/portmidi",
		},
		{
			paths: []string{"/portmidi"},
			query: "/foo",
			want:  "",
		},
		{
			paths: []string{"/portmidi"},
			query: "/zzz",
			want:  "",
		},
		{
			paths: []string{"/abc", "/portmidi", "/xyz"},
			query: "/portmidi",
			want:  "/portmidi",
		},
		{
			paths:   []string{"/abc", "/portmidi", "/xyz"},
			query:   "/portmidi/foo",
			want:    "/portmidi",
			subpath: "foo",
		},
	}
	emptyToNil := func(s string) string {
		if s == "" {
			return "<nil>"
		}
		return s
	}
	for _, test := range tests {
		pset := make(pathConfigSet, len(test.paths))
		for i := range test.paths {
			pset[i].path = test.paths[i]
		}
		sort.Sort(pset)
		pc, subpath := pset.find(test.query)
		var got string
		if pc != nil {
			got = pc.path
		}
		if got != test.want || subpath != test.subpath {
			t.Errorf("pathConfigSet(%v).find(%q) = %v, %v; want %v, %v",
				test.paths, test.query, emptyToNil(got), subpath, emptyToNil(test.want), test.subpath)
		}
	}
}

func TestFindStructure(t *testing.T) {
	tests := []struct {
		name            string
		match           string
		wantPrefix      string
		wantPlaceholder string
		wantSuffix      string
		wantErr         bool
	}{
		{
			name:    "empty",
			wantErr: true,
		},
		{
			name:            "empty prefix and suffix",
			match:           "{name}",
			wantPlaceholder: "{name}",
		},
		{
			name:    "empty placeholder",
			match:   "{}",
			wantErr: true,
		},
		{
			name:    "no placeholder, but text",
			match:   "nothing",
			wantErr: true,
		},
		{
			name:    "not terminated placeholder",
			match:   "{open",
			wantErr: true,
		},
		{
			name:    "multiple placeholders",
			match:   "{one}two{three}",
			wantErr: true,
		},
		{
			name:            "have prefix and no suffix",
			match:           "/prefix/{name}",
			wantPrefix:      "/prefix/",
			wantPlaceholder: "{name}",
		},
		{
			name:            "have prefix and suffix",
			match:           "/prefix/{name}.suffix",
			wantPrefix:      "/prefix/",
			wantPlaceholder: "{name}",
			wantSuffix:      ".suffix",
		},
		{
			name:            "have single character prefix, placeholder and suffix",
			match:           "p{n}s",
			wantPrefix:      "p",
			wantPlaceholder: "{n}",
			wantSuffix:      "s",
		},
	}
	for _, tt := range tests {
		gotPrefix, gotPlaceholder, gotSuffix, err := findStructure(tt.match)
		if (err != nil) != tt.wantErr {
			t.Errorf("%s: findStructure(%v) error = %v, wantErr %v", tt.name, tt.match, err, tt.wantErr)
			return
		}
		if gotPrefix != tt.wantPrefix {
			t.Errorf("%s: findStructure(%v) gotPrefix = %v, want %v", tt.name, tt.match, gotPrefix, tt.wantPrefix)
		}
		if gotPlaceholder != tt.wantPlaceholder {
			t.Errorf("%s: findStructure(%v) gotPlaceholder = %v, want %v", tt.name, tt.match, gotPlaceholder, tt.wantPlaceholder)
		}
		if gotSuffix != tt.wantSuffix {
			t.Errorf("%s: findStructure(%v) gotSuffix = %v, want %v", tt.name, tt.match, gotSuffix, tt.wantSuffix)
		}
	}
}
