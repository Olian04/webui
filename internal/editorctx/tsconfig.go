package editorctx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
)

// The numbers below are TypeScript's own enum values, as of the 5.9 that Monaco bundles. The
// service is given the numbers, not Monaco's enums, because those stop at ES2020 and at Node
// resolution while the TypeScript underneath knows more.
var (
	targets = map[string]int{
		"es3": 0, "es5": 1, "es6": 2, "es2015": 2, "es2016": 3, "es2017": 4, "es2018": 5,
		"es2019": 6, "es2020": 7, "es2021": 8, "es2022": 9, "es2023": 10, "es2024": 11, "esnext": 99,
	}
	modules = map[string]int{
		"none": 0, "commonjs": 1, "amd": 2, "umd": 3, "system": 4, "es6": 5, "es2015": 5,
		"es2020": 6, "es2022": 7, "esnext": 99, "node16": 100, "node18": 101, "node20": 102,
		"nodenext": 199, "preserve": 200,
	}
	resolutions = map[string]int{"classic": 1, "node": 2, "node10": 2, "node16": 3, "nodenext": 99, "bundler": 100}
	jsxModes    = map[string]int{"preserve": 1, "react": 2, "react-native": 3, "react-jsx": 4, "react-jsxdev": 5}
)

// libs are the library files Monaco bundles, by the name tsconfig.json gives them ("es2022",
// "dom.iterable"). A name outside it would be a file the service cannot find.
var libs = func() map[string]bool {
	m := map[string]bool{}
	for _, name := range strings.Fields(`
		decorators decorators.legacy dom dom.asynciterable dom.iterable
		es5 es6 es7 es2015 es2015.collection es2015.core es2015.generator es2015.iterable es2015.promise
		es2015.proxy es2015.reflect es2015.symbol es2015.symbol.wellknown
		es2016 es2016.array.include es2016.intl
		es2017 es2017.arraybuffer es2017.date es2017.intl es2017.object es2017.sharedmemory es2017.string es2017.typedarrays
		es2018 es2018.asyncgenerator es2018.asynciterable es2018.intl es2018.promise es2018.regexp
		es2019 es2019.array es2019.intl es2019.object es2019.string es2019.symbol
		es2020 es2020.bigint es2020.date es2020.intl es2020.number es2020.promise es2020.sharedmemory es2020.string es2020.symbol.wellknown
		es2021 es2021.intl es2021.promise es2021.string es2021.weakref
		es2022 es2022.array es2022.error es2022.intl es2022.object es2022.regexp es2022.string
		es2023 es2023.array es2023.collection es2023.intl
		es2024 es2024.arraybuffer es2024.collection es2024.object es2024.promise es2024.regexp es2024.sharedmemory es2024.string
		esnext esnext.array esnext.collection esnext.decorators esnext.disposable esnext.error esnext.float16
		esnext.intl esnext.iterator esnext.promise esnext.sharedmemory
		scripthost webworker webworker.asynciterable webworker.importscripts webworker.iterable`) {
		m[name] = true
	}
	return m
}()

// libFile is the file a lib name stands for.
func libFile(name string) string {
	switch name {
	case "es6":
		name = "es2015"
	case "es7":
		name = "es2016"
	}
	return "lib." + name + ".d.ts"
}

// flags are the options that are true or false. They are the ones that change what is an
// error, which is what a language service is for; the options that only shape output are not
// here, since nothing is emitted.
var flags = []string{
	"allowJs", "allowSyntheticDefaultImports", "allowUmdGlobalAccess", "allowUnreachableCode",
	"allowUnusedLabels", "alwaysStrict", "checkJs", "esModuleInterop", "exactOptionalPropertyTypes",
	"experimentalDecorators", "isolatedModules", "noFallthroughCasesInSwitch", "noImplicitAny",
	"noImplicitOverride", "noImplicitReturns", "noImplicitThis", "noPropertyAccessFromIndexSignature",
	"noUncheckedIndexedAccess", "noUnusedLocals", "noUnusedParameters", "resolveJsonModule",
	"skipDefaultLibCheck", "skipLibCheck", "strict", "strictBindCallApply", "strictFunctionTypes",
	"strictNullChecks", "strictPropertyInitialization", "useUnknownInCatchVariables",
}

var enums = map[string]map[string]int{
	"target": targets, "module": modules, "moduleResolution": resolutions, "jsx": jsxModes,
}

// projectKeys are the tsconfig.json keys that describe a project: which files are in it,
// and what it builds on. The editor has one text, so none of them has a meaning here.
var projectKeys = []string{"extends", "include", "exclude", "files", "references"}

// supported lists the option names a tsconfig.json may use, for the message that rejects another.
func supported() string {
	names := append([]string{"target", "module", "moduleResolution", "jsx", "lib", "baseUrl", "paths", "types", "typeRoots"}, flags...)
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// readTSConfig reads the compilerOptions of a tsconfig.json into the object the editor's
// script gives the service: booleans, strings and lists as written, and the options that
// name a value of an enumeration as TypeScript's own number for it.
func readTSConfig(src []byte) (map[string]any, []Problem) {
	var top map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(stripJSONC(src)))
	if err := dec.Decode(&top); err != nil {
		return nil, []Problem{{
			Detail: "tsconfig.json is not valid JSON: " + err.Error(),
			Fix:    "Write it as tsc reads it: a JSON object, in which comments and trailing commas are allowed.",
		}}
	}
	var problems []Problem
	bad := func(detail, fix string) { problems = append(problems, Problem{Detail: detail, Fix: fix}) }

	for key := range top {
		switch {
		case key == "compilerOptions":
		case slices.Contains(projectKeys, key):
			bad(fmt.Sprintf("tsconfig.json has %q, which describes a project, and an editor has one text", key),
				"Remove it. Put the declaration files an editor should know in the same directory, and the options in compilerOptions.")
		default:
			bad(fmt.Sprintf("tsconfig.json has %q, which the editor does not read", key), "Keep only compilerOptions.")
		}
	}
	var raw map[string]json.RawMessage
	if b, ok := top["compilerOptions"]; ok {
		if err := json.Unmarshal(b, &raw); err != nil {
			bad("tsconfig.json compilerOptions is not an object", "Write compilerOptions as an object of options.")
			return nil, problems
		}
	}

	out := map[string]any{}
	names := make([]string, 0, len(raw))
	for name := range raw {
		names = append(names, name)
	}
	sort.Strings(names) // a stable order of problems
	for _, name := range names {
		value, err := option(name, raw[name])
		if err != "" {
			bad(fmt.Sprintf("compilerOptions.%s %s", name, err), "")
			continue
		}
		out[name] = value
	}
	for i := range problems {
		if problems[i].Fix == "" {
			problems[i].Fix = "Use an option the editor's language service reads: " + supported() + "."
		}
	}
	return out, problems
}

// option converts one option's value, or says what is wrong with it.
func option(name string, raw json.RawMessage) (any, string) {
	switch {
	case slices.Contains(flags, name):
		var b bool
		if json.Unmarshal(raw, &b) != nil {
			return nil, "must be true or false"
		}
		return b, ""
	case enums[name] != nil:
		var s string
		if json.Unmarshal(raw, &s) != nil {
			return nil, "must be a string"
		}
		n, ok := enums[name][strings.ToLower(s)]
		if !ok {
			return nil, fmt.Sprintf("is %q, which is not one of %s", s, keys(enums[name]))
		}
		return n, ""
	case name == "lib":
		var list []string
		if json.Unmarshal(raw, &list) != nil {
			return nil, "must be a list of strings"
		}
		files := make([]string, len(list))
		for i, l := range list {
			l = strings.ToLower(l)
			if !libs[l] {
				return nil, fmt.Sprintf("names %q, which is not a library that the editor carries", list[i])
			}
			files[i] = libFile(l)
		}
		return files, ""
	case name == "types" || name == "typeRoots":
		var list []string
		if json.Unmarshal(raw, &list) != nil {
			return nil, "must be a list of strings"
		}
		return list, ""
	case name == "baseUrl":
		var s string
		if json.Unmarshal(raw, &s) != nil {
			return nil, "must be a string"
		}
		return s, ""
	case name == "paths":
		var m map[string][]string
		if json.Unmarshal(raw, &m) != nil {
			return nil, "must be an object of lists of strings"
		}
		return m, ""
	}
	return nil, "is not an option the editor reads"
}

func keys(m map[string]int) string {
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// decodeJSON reads one JSON value and nothing after it.
func decodeJSON(src []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(src))
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.More() {
		return fmt.Errorf("there is more after the first value")
	}
	return nil
}
