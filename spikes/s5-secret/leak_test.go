package secret

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	htmltemplate "html/template"
	"io"
	"log"
	"log/slog"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
	texttemplate "text/template"
	"unsafe"

	"github.com/davecgh/go-spew/spew"
	"github.com/google/go-cmp/cmp"
)

const canary = "hunter2-LEAK-CANARY"

type exportedHolder struct {
	Secret Secret
	Name   string
}

type unexportedHolder struct {
	secret Secret
}

type nestedHolder struct {
	Exported   exportedHolder
	Unexported unexportedHolder
	Ptr        *Secret
	Slice      []Secret
	Map        map[string]Secret
	Any        any
}

func assertNoLeak(t *testing.T, context, out string) {
	t.Helper()
	if strings.Contains(out, canary) {
		t.Fatalf("%s leaked canary in output: %q", context, out)
	}
	if strings.Contains(out, "hunter2") || strings.Contains(out, "LEAK-CANARY") {
		t.Fatalf("%s leaked partial canary in output: %q", context, out)
	}
	// Also check hex-encoded canary (lowercase and uppercase) in case %x/%X encoded raw bytes.
	hexLower := fmt.Sprintf("%x", canary)
	hexUpper := fmt.Sprintf("%X", canary)
	if strings.Contains(out, hexLower) || strings.Contains(out, hexUpper) {
		t.Fatalf("%s leaked hex-encoded canary in output: %q", context, out)
	}
}

func TestSecret_BasicSemantics(t *testing.T) {
	t.Parallel()

	var zero Secret
	if zero.IsSet() {
		t.Fatal("zero Secret should not be set")
	}
	if got := zero.Reveal(); got != "" {
		t.Fatalf("zero.Reveal() = %q, want empty", got)
	}
	if got := zero.Bytes(); got != nil {
		t.Fatalf("zero.Bytes() = %v, want nil", got)
	}
	if zero.Equal("") {
		t.Fatal("zero.Equal(\"\") = true, want false when unset")
	}
	if zero.Equal(canary) {
		t.Fatal("zero.Equal(canary) = true, want false")
	}

	empty := New("")
	if empty.IsSet() {
		t.Fatal("New(\"\") should be unset")
	}
	if empty.Equal("") {
		t.Fatal("New(\"\").Equal(\"\") = true, want false when unset")
	}
	if got := empty.Bytes(); got != nil {
		t.Fatalf("New(\"\").Bytes() = %v, want nil", got)
	}

	s := New(canary)
	if !s.IsSet() {
		t.Fatal("New(canary).IsSet() = false, want true")
	}
	if got := s.Reveal(); got != canary {
		t.Fatalf("s.Reveal() = %q, want %q", got, canary)
	}
	if !s.Equal(canary) {
		t.Fatal("s.Equal(canary) = false, want true")
	}
	if s.Equal("wrong") {
		t.Fatal("s.Equal(\"wrong\") = true, want false")
	}
	if s.Equal("") {
		t.Fatal("s.Equal(\"\") = true, want false")
	}

	// Bytes() must return a defensive copy.
	b1 := s.Bytes()
	if string(b1) != canary {
		t.Fatalf("s.Bytes() = %q, want %q", string(b1), canary)
	}
	for i := range b1 {
		b1[i] = 'X'
	}
	if got := s.Reveal(); got != canary {
		t.Fatalf("mutating Bytes() mutated Reveal(): got %q", got)
	}
	if got := string(s.Bytes()); got != canary {
		t.Fatalf("mutating Bytes() mutated subsequent Bytes(): got %q", got)
	}
}

func TestLeak_FmtVerbsAndFunctions(t *testing.T) {
	// Not parallel because subtest captures os.Stdout for fmt.Print/Println.
	s := New(canary)
	var unset Secret

	verbs := []string{
		"%v", "%+v", "%#v", "%s", "%q", "%x", "%X", "%T", "%d",
		"%p", "%10s", "%.3s", "%-10v",
	}

	for _, secretVal := range []struct {
		name string
		val  Secret
	}{
		{"set", s},
		{"unset", unset},
	} {
		v := secretVal.val
		vp := &v

		for _, verb := range verbs {
			outVal := fmt.Sprintf(verb, v)
			assertNoLeak(t, fmt.Sprintf("Sprintf(%s, %s value)", verb, secretVal.name), outVal)
			if verb == "%T" {
				if outVal != "secret.Secret" {
					t.Fatalf("Sprintf(%%T, value) = %q, want \"secret.Secret\"", outVal)
				}
			} else if verb == "%p" {
				// fmt handles %p before fmt.Formatter; for a non-pointer value badVerb prints
				// "%!p(secret.Secret={0x...})" or "%!p(secret.Secret={<nil>})" without calling methods.
				if !strings.HasPrefix(outVal, "%!p(secret.Secret={") {
					t.Fatalf("Sprintf(%%p, value) = %q, want %%!p(secret.Secret={...})", outVal)
				}
			} else if outVal != mask {
				t.Fatalf("Sprintf(%s, %s value) = %q, want %q", verb, secretVal.name, outVal, mask)
			}

			outPtr := fmt.Sprintf(verb, vp)
			assertNoLeak(t, fmt.Sprintf("Sprintf(%s, %s ptr)", verb, secretVal.name), outPtr)
			if verb == "%T" {
				if outPtr != "*secret.Secret" {
					t.Fatalf("Sprintf(%%T, ptr) = %q, want \"*secret.Secret\"", outPtr)
				}
			} else if verb == "%p" {
				if !strings.HasPrefix(outPtr, "0x") {
					t.Fatalf("Sprintf(%%p, ptr) = %q, want 0x... pointer address", outPtr)
				}
			} else if outPtr != mask {
				t.Fatalf("Sprintf(%s, %s ptr) = %q, want %q", verb, secretVal.name, outPtr, mask)
			}
		}

		// Sprint / Sprintln
		if got := fmt.Sprint(v); got != mask {
			t.Fatalf("Sprint(value) = %q, want %q", got, mask)
		}
		if got := fmt.Sprint(vp); got != mask {
			t.Fatalf("Sprint(ptr) = %q, want %q", got, mask)
		}
		if got := fmt.Sprintln(v); got != mask+"\n" {
			t.Fatalf("Sprintln(value) = %q, want %q", got, mask+"\n")
		}
		if got := fmt.Sprintln(vp); got != mask+"\n" {
			t.Fatalf("Sprintln(ptr) = %q, want %q", got, mask+"\n")
		}

		// Fprint / Fprintln / Fprintf
		var buf bytes.Buffer
		_, _ = fmt.Fprint(&buf, v, " ", vp)
		assertNoLeak(t, "Fprint", buf.String())
		if buf.String() != mask+" "+mask {
			t.Fatalf("Fprint = %q, want %q", buf.String(), mask+" "+mask)
		}

		buf.Reset()
		_, _ = fmt.Fprintln(&buf, v, vp)
		assertNoLeak(t, "Fprintln", buf.String())
		if buf.String() != mask+" "+mask+"\n" {
			t.Fatalf("Fprintln = %q, want %q", buf.String(), mask+" "+mask+"\n")
		}

		buf.Reset()
		_, _ = fmt.Fprintf(&buf, "%v %+v %#v %s %q", v, v, v, vp, vp)
		assertNoLeak(t, "Fprintf", buf.String())

		// Errorf
		err := fmt.Errorf("bad secret: %v %+v %#v %s %q %x %X %T %d", v, vp, v, vp, v, v, vp, v, v)
		assertNoLeak(t, "Errorf", err.Error())

		// Direct String() and GoString()
		if got := v.String(); got != mask {
			t.Fatalf("String() = %q, want %q", got, mask)
		}
		if got := v.GoString(); got != mask {
			t.Fatalf("GoString() = %q, want %q", got, mask)
		}
	}

	// Print and Println via captured os.Stdout
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	_, _ = fmt.Print(s, " ", &s, "\n")
	_, _ = fmt.Println(s, &s)
	_ = w.Close()
	os.Stdout = origStdout

	outBytes, err := io.ReadAll(r)
	_ = r.Close()
	if err != nil {
		t.Fatalf("ReadAll pipe: %v", err)
	}
	out := string(outBytes)
	assertNoLeak(t, "Print/Println", out)
	wantStdout := mask + " " + mask + "\n" + mask + " " + mask + "\n"
	if out != wantStdout {
		t.Fatalf("Print/Println output = %q, want %q", out, wantStdout)
	}
}

func TestLeak_ParentStructsAndContainers(t *testing.T) {
	t.Parallel()

	s := New(canary)
	exp := exportedHolder{Secret: s, Name: "db"}
	unexp := unexportedHolder{secret: s}
	var unexpUnset unexportedHolder

	// 1. Exported field by value and by pointer.
	for _, verb := range []string{"%v", "%+v", "%#v"} {
		outVal := fmt.Sprintf(verb, exp)
		assertNoLeak(t, "exportedHolder value "+verb, outVal)
		if !strings.Contains(outVal, mask) {
			t.Fatalf("exportedHolder value %s = %q, expected mask %q", verb, outVal, mask)
		}

		outPtr := fmt.Sprintf(verb, &exp)
		assertNoLeak(t, "exportedHolder ptr "+verb, outPtr)
		if !strings.Contains(outPtr, mask) {
			t.Fatalf("exportedHolder ptr %s = %q, expected mask %q", verb, outPtr, mask)
		}
	}

	// 2. Unexported field by value and by pointer: confirm exact %v, %+v, %#v output.
	// When Secret sits in an unexported field, fmt cannot call its methods and formats
	// the inner struct { reveal func() string } via reflection.
	reValV := regexp.MustCompile(`^\{\{0x[0-9a-f]+\}\}$`)
	reValPlusV := regexp.MustCompile(`^\{secret:\{reveal:0x[0-9a-f]+\}\}$`)
	reValSharpV := regexp.MustCompile(`^secret\.unexportedHolder\{secret:secret\.Secret\{reveal:\(func\(\) string\)\(0x[0-9a-f]+\)\}\}$`)

	gotV := fmt.Sprintf("%v", unexp)
	assertNoLeak(t, "unexportedHolder value %v", gotV)
	if !reValV.MatchString(gotV) {
		t.Fatalf("Sprintf(%%v, unexportedHolder) = %q, want match %s", gotV, reValV)
	}

	gotPlusV := fmt.Sprintf("%+v", unexp)
	assertNoLeak(t, "unexportedHolder value %+v", gotPlusV)
	if !reValPlusV.MatchString(gotPlusV) {
		t.Fatalf("Sprintf(%%+v, unexportedHolder) = %q, want match %s", gotPlusV, reValPlusV)
	}

	gotSharpV := fmt.Sprintf("%#v", unexp)
	assertNoLeak(t, "unexportedHolder value %#v", gotSharpV)
	if !reValSharpV.MatchString(gotSharpV) {
		t.Fatalf("Sprintf(%%#v, unexportedHolder) = %q, want match %s", gotSharpV, reValSharpV)
	}

	// Pointer to unexportedHolder
	gotPtrV := fmt.Sprintf("%v", &unexp)
	assertNoLeak(t, "unexportedHolder ptr %v", gotPtrV)
	if gotPtrV != "&"+gotV {
		t.Fatalf("Sprintf(%%v, &unexportedHolder) = %q, want %q", gotPtrV, "&"+gotV)
	}

	gotPtrPlusV := fmt.Sprintf("%+v", &unexp)
	assertNoLeak(t, "unexportedHolder ptr %+v", gotPtrPlusV)
	if gotPtrPlusV != "&"+gotPlusV {
		t.Fatalf("Sprintf(%%+v, &unexportedHolder) = %q, want %q", gotPtrPlusV, "&"+gotPlusV)
	}

	gotPtrSharpV := fmt.Sprintf("%#v", &unexp)
	assertNoLeak(t, "unexportedHolder ptr %#v", gotPtrSharpV)
	if gotPtrSharpV != "&"+gotSharpV {
		t.Fatalf("Sprintf(%%#v, &unexportedHolder) = %q, want %q", gotPtrSharpV, "&"+gotSharpV)
	}

	// Unset Secret in an unexported field prints <nil> for the func pointer.
	if got := fmt.Sprintf("%v", unexpUnset); got != "{{<nil>}}" {
		t.Fatalf("Sprintf(%%v, unexpUnset) = %q, want \"{{<nil>}}\"", got)
	}
	if got := fmt.Sprintf("%+v", unexpUnset); got != "{secret:{reveal:<nil>}}" {
		t.Fatalf("Sprintf(%%+v, unexpUnset) = %q, want \"{secret:{reveal:<nil>}}\"", got)
	}

	// 3. Slices, arrays, maps, nested structs, and interfaces holding a Secret.
	sliceVal := []Secret{s, New("")}
	arrayVal := [2]Secret{s, New("")}
	mapVal := map[string]Secret{"primary": s, "empty": New("")}
	var ifaceVal any = s
	var ifacePtr any = &s
	var stringerVal fmt.Stringer = s
	nested := nestedHolder{
		Exported:   exp,
		Unexported: unexp,
		Ptr:        &s,
		Slice:      sliceVal,
		Map:        mapVal,
		Any:        s,
	}

	containers := []struct {
		name string
		val  any
	}{
		{"slice", sliceVal},
		{"array", arrayVal},
		{"map", mapVal},
		{"any_val", ifaceVal},
		{"any_ptr", ifacePtr},
		{"stringer", stringerVal},
		{"nested_val", nested},
		{"nested_ptr", &nested},
	}

	for _, tc := range containers {
		for _, verb := range []string{"%v", "%+v", "%#v"} {
			out := fmt.Sprintf(verb, tc.val)
			assertNoLeak(t, tc.name+" "+verb, out)
		}
	}

	// 4. Confirm Secret is not comparable and cannot be a map key.
	secretType := reflect.TypeFor[Secret]()
	if secretType.Comparable() {
		t.Fatal("reflect.TypeFor[Secret]().Comparable() = true, want false")
	}
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("reflect.MapOf(Secret, string) did not panic, expected panic for incomparable key")
			}
		}()
		_ = reflect.MapOf(secretType, reflect.TypeFor[string]())
	}()
}

func TestLeak_EncodingsTemplatesLogsErrors(t *testing.T) {
	t.Parallel()

	s := New(canary)
	exp := exportedHolder{Secret: s, Name: "db"}
	unexp := unexportedHolder{secret: s}

	// 1. encoding/json: value, ptr, struct fields (exported & unexported), maps, slices.
	jsonCases := []struct {
		name string
		val  any
		want string
	}{
		{"value", s, `"******"`},
		{"ptr", &s, `"******"`},
		{"unset", Secret{}, `"******"`},
		{"exported_struct", exp, `{"Secret":"******","Name":"db"}`},
		{"exported_struct_ptr", &exp, `{"Secret":"******","Name":"db"}`},
		{"unexported_struct", unexp, `{}`},
		{"map", map[string]Secret{"k": s}, `{"k":"******"}`},
		{"slice", []Secret{s}, `["******"]`},
	}
	for _, tc := range jsonCases {
		b, err := json.Marshal(tc.val)
		if err != nil {
			t.Fatalf("json.Marshal(%s): %v", tc.name, err)
		}
		out := string(b)
		assertNoLeak(t, "json.Marshal("+tc.name+")", out)
		if out != tc.want {
			t.Fatalf("json.Marshal(%s) = %s, want %s", tc.name, out, tc.want)
		}
	}

	// 2. MarshalText and MarshalYAML.
	textBytes, err := s.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText: %v", err)
	}
	assertNoLeak(t, "MarshalText", string(textBytes))
	if string(textBytes) != mask {
		t.Fatalf("MarshalText = %q, want %q", string(textBytes), mask)
	}

	yamlVal, err := s.MarshalYAML()
	if err != nil {
		t.Fatalf("MarshalYAML: %v", err)
	}
	yamlStr, ok := yamlVal.(string)
	if !ok || yamlStr != mask {
		t.Fatalf("MarshalYAML = %#v, want %q", yamlVal, mask)
	}
	assertNoLeak(t, "MarshalYAML", yamlStr)

	// 3. encoding/xml.
	xmlCases := []struct {
		name string
		val  any
		want string
	}{
		{"value", s, "<Secret>******</Secret>"},
		{"ptr", &s, "<Secret>******</Secret>"},
		{"exported_struct", exp, "<exportedHolder><Secret>******</Secret><Name>db</Name></exportedHolder>"},
		{"unexported_struct", unexp, "<unexportedHolder></unexportedHolder>"},
	}
	for _, tc := range xmlCases {
		b, err := xml.Marshal(tc.val)
		if err != nil {
			t.Fatalf("xml.Marshal(%s): %v", tc.name, err)
		}
		out := string(b)
		assertNoLeak(t, "xml.Marshal("+tc.name+")", out)
		if out != tc.want {
			t.Fatalf("xml.Marshal(%s) = %s, want %s", tc.name, out, tc.want)
		}
	}

	// 4. text/template and html/template: {{.}} and {{printf "%v" .}} (plus %+v, %#v).
	tmplSrcs := []string{
		`{{.}}`,
		`{{printf "%v" .}}`,
		`{{printf "%+v" .}}`,
		`{{printf "%#v" .}}`,
	}
	tmplInputs := []struct {
		name string
		val  any
	}{
		{"secret_val", s},
		{"secret_ptr", &s},
		{"exported_struct", exp},
		{"unexported_struct", unexp},
	}
	for _, src := range tmplSrcs {
		tt := texttemplate.Must(texttemplate.New("t").Parse(src))
		ht := htmltemplate.Must(htmltemplate.New("h").Parse(src))
		for _, inp := range tmplInputs {
			var tbuf bytes.Buffer
			if err := tt.Execute(&tbuf, inp.val); err != nil {
				t.Fatalf("text/template %s on %s: %v", src, inp.name, err)
			}
			assertNoLeak(t, "text/template "+src+" "+inp.name, tbuf.String())

			var hbuf bytes.Buffer
			if err := ht.Execute(&hbuf, inp.val); err != nil {
				t.Fatalf("html/template %s on %s: %v", src, inp.name, err)
			}
			assertNoLeak(t, "html/template "+src+" "+inp.name, hbuf.String())
		}
	}

	// 5. log.Printf / log.Print / log.Println.
	var logBuf bytes.Buffer
	stdLogger := log.New(&logBuf, "", 0)
	stdLogger.Printf("val=%v ptr=%v exp=%+v unexp=%+v sharp=%#v", s, &s, exp, unexp, unexp)
	stdLogger.Print(s, &s, exp, unexp)
	stdLogger.Println(s, &s, exp, unexp)
	assertNoLeak(t, "log.Printf/Print/Println", logBuf.String())

	// 6. slog TextHandler and JSONHandler: top-level attr, inside a group, inside a struct passed with slog.Any.
	for _, handlerKind := range []string{"text", "json"} {
		var sbuf bytes.Buffer
		var h slog.Handler
		if handlerKind == "text" {
			h = slog.NewTextHandler(&sbuf, nil)
		} else {
			h = slog.NewJSONHandler(&sbuf, nil)
		}
		logger := slog.New(h)
		logger.Info("config loaded",
			"top_val", s,
			"top_ptr", &s,
			slog.Any("any_val", s),
			slog.Any("any_ptr", &s),
			slog.Group("group",
				"g_val", s,
				"g_ptr", &s,
				slog.Any("g_exp", exp),
				slog.Any("g_unexp", unexp),
			),
			slog.Any("struct_exp", exp),
			slog.Any("struct_exp_ptr", &exp),
			slog.Any("struct_unexp", unexp),
			slog.Any("struct_unexp_ptr", &unexp),
			slog.Any("nested", nestedHolder{
				Exported:   exp,
				Unexported: unexp,
				Ptr:        &s,
				Slice:      []Secret{s},
				Map:        map[string]Secret{"k": s},
				Any:        s,
			}),
		)
		out := sbuf.String()
		assertNoLeak(t, "slog."+handlerKind, out)
		if !strings.Contains(out, mask) {
			t.Fatalf("slog.%s output did not contain mask %q: %s", handlerKind, mask, out)
		}
	}

	// 7. errors.New(fmt.Sprint(...)) and fmt.Errorf with %w wrapping.
	err1 := errors.New(fmt.Sprint(s, " ", &s, " ", exp, " ", unexp))
	assertNoLeak(t, "errors.New(fmt.Sprint)", err1.Error())

	err2 := fmt.Errorf("wrapping %v %+v %#v: %w", s, exp, unexp, err1)
	assertNoLeak(t, "fmt.Errorf %w", err2.Error())
	if !errors.Is(err2, err1) {
		t.Fatal("errors.Is(err2, err1) = false, want true")
	}
}

func TestLeak_ReflectionDumpersAndGob(t *testing.T) {
	t.Parallel()

	s1 := New(canary)
	s2 := New("second-secret-LEAK-CANARY-2")
	exp1 := exportedHolder{Secret: s1, Name: "db"}
	exp2 := exportedHolder{Secret: s2, Name: "db"}
	unexp1 := unexportedHolder{secret: s1}
	unexp2 := unexportedHolder{secret: s2}

	// 1. github.com/davecgh/go-spew/spew (default config, plus DisableMethods: true which forces unsafe field traversal).
	spewDefaultOut := spew.Sdump(s1, &s1, exp1, &exp1, unexp1, &unexp1)
	assertNoLeak(t, "spew.Sdump default", spewDefaultOut)
	if !strings.Contains(spewDefaultOut, mask) {
		t.Fatalf("spew.Sdump default did not contain mask %q: %s", mask, spewDefaultOut)
	}

	spewRawCfg := spew.ConfigState{
		DisableMethods:        true,
		DisablePointerMethods: true,
		ContinueOnMethod:      true,
	}
	spewRawOut := spewRawCfg.Sdump(s1, &s1, exp1, &exp1, unexp1, &unexp1)
	assertNoLeak(t, "spew.Sdump DisableMethods", spewRawOut)
	if !strings.Contains(spewRawOut, "reveal: (func() string) 0x") {
		t.Fatalf("spew.Sdump DisableMethods output = %q, expected func address", spewRawOut)
	}

	// 2. github.com/google/go-cmp/cmp.
	// Without AllowUnexported, cmp.Diff panics on unexported fields.
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("cmp.Diff without AllowUnexported did not panic")
			} else {
				assertNoLeak(t, "cmp.Diff panic message", fmt.Sprint(r))
			}
		}()
		_ = cmp.Diff(exp1, exp2)
	}()

	// With cmp.AllowUnexported, cmp compares the func fields by pointer and formats their addresses, never the secret string.
	diffExp := cmp.Diff(exp1, exp2, cmp.AllowUnexported(Secret{}))
	assertNoLeak(t, "cmp.Diff exportedHolder with AllowUnexported", diffExp)
	if strings.Contains(diffExp, "second-secret-LEAK-CANARY-2") {
		t.Fatalf("cmp.Diff leaked second secret: %q", diffExp)
	}
	if !strings.Contains(diffExp, "reveal: ⟪0x") {
		t.Fatalf("cmp.Diff exportedHolder output = %q, expected ⟪0x...⟫ func address", diffExp)
	}

	diffUnexp := cmp.Diff(unexp1, unexp2, cmp.AllowUnexported(unexportedHolder{}, Secret{}))
	assertNoLeak(t, "cmp.Diff unexportedHolder with AllowUnexported", diffUnexp)
	if strings.Contains(diffUnexp, "second-secret-LEAK-CANARY-2") {
		t.Fatalf("cmp.Diff unexportedHolder leaked second secret: %q", diffUnexp)
	}
	if !strings.Contains(diffUnexp, "reveal: ⟪0x") {
		t.Fatalf("cmp.Diff unexportedHolder output = %q, expected ⟪0x...⟫ func address", diffUnexp)
	}

	// 3. encoding/gob: record what happens for direct Secret, exportedHolder, unexportedHolder,
	// and a mixed struct with an exported non-secret field plus an unexported Secret field.
	// gob checks GobEncoder and encoding.BinaryMarshaler (not TextMarshaler), then falls back to
	// exported struct fields. Because Secret has only an unexported func field, gob rejects it with:
	// "gob: type secret.Secret has no exported fields".
	var gobBuf bytes.Buffer
	errGobDirect := gob.NewEncoder(&gobBuf).Encode(s1)
	if errGobDirect == nil {
		t.Fatal("gob.Encode(s1) succeeded, expected error for struct with no exported fields")
	}
	assertNoLeak(t, "gob.Encode(s1) error", errGobDirect.Error())
	assertNoLeak(t, "gob.Encode(s1) buffer", gobBuf.String())

	gobBuf.Reset()
	errGobExp := gob.NewEncoder(&gobBuf).Encode(exp1)
	if errGobExp == nil {
		t.Fatal("gob.Encode(exp1) succeeded, expected error for field Secret with no exported fields")
	}
	assertNoLeak(t, "gob.Encode(exp1) error", errGobExp.Error())
	assertNoLeak(t, "gob.Encode(exp1) buffer", gobBuf.String())

	gobBuf.Reset()
	errGobUnexp := gob.NewEncoder(&gobBuf).Encode(unexp1)
	if errGobUnexp == nil {
		t.Fatal("gob.Encode(unexp1) succeeded, expected error for struct with no exported fields")
	}
	assertNoLeak(t, "gob.Encode(unexp1) error", errGobUnexp.Error())
	assertNoLeak(t, "gob.Encode(unexp1) buffer", gobBuf.String())

	// Mixed struct with an exported field and an unexported Secret field: gob encodes the exported
	// field and silently ignores the unexported Secret field.
	gobBuf.Reset()
	mixed := struct {
		Name   string
		secret Secret
	}{Name: "db", secret: s1}
	if err := gob.NewEncoder(&gobBuf).Encode(mixed); err != nil {
		t.Fatalf("gob.Encode(mixed) unexpected error: %v", err)
	}
	assertNoLeak(t, "gob.Encode(mixed) buffer", gobBuf.String())
}

func TestLeak_ReflectWithoutUnsafeCannotReachString(t *testing.T) {
	t.Parallel()

	s := New(canary)
	rv := reflect.ValueOf(s)
	field := rv.Field(0) // reveal func() string

	if field.Kind() != reflect.Func {
		t.Fatalf("field.Kind() = %v, want reflect.Func", field.Kind())
	}
	if field.CanInterface() {
		t.Fatal("unexported reveal field should not allow Interface()")
	}
	assertNoLeak(t, "reflect.Value.String()", field.String())

	// Attempting to call the unexported func field via reflect without unsafe panics.
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatal("field.Call(nil) on unexported func field did not panic")
			}
			assertNoLeak(t, "field.Call panic", fmt.Sprint(r))
		}()
		_ = field.Call(nil)
	}()

	// Confirm what unsafe CAN do (documenting the boundary in requirement 3):
	// unsafe can turn the unexported field into a callable func or read the closure struct directly.
	sCopy := s
	fnPtr := (*func() string)(unsafe.Pointer(&sCopy))
	if got := (*fnPtr)(); got != canary {
		t.Fatalf("unsafe call got %q, want %q", got, canary)
	}
}
