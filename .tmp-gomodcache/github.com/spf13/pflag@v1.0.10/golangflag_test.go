// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pflag

import (
	goflag "flag"
	"testing"
	"time"
)

func TestGoflags(t *testing.T) {
	goflag.String("stringFlag", "stringFlag", "stringFlag")
	goflag.Bool("boolFlag", false, "boolFlag")
	var testxxxValue string
	goflag.StringVar(&testxxxValue, "test.xxx", "test.xxx", "it is a test flag")

	f := NewFlagSet("test", ContinueOnError)

	f.AddGoFlagSet(goflag.CommandLine)
	args := []string{"--stringFlag=bob", "--boolFlag", "-test.xxx=testvalue"}
	err := f.Parse(args)
	if err != nil {
		t.Fatal("expected no error; get", err)
	}

	getString, err := f.GetString("stringFlag")
	if err != nil {
		t.Fatal("expected no error; get", err)
	}
	if getString != "bob" {
		t.Fatalf("expected getString=bob but got getString=%s", getString)
	}

	getBool, err := f.GetBool("boolFlag")
	if err != nil {
		t.Fatal("expected no error; get", err)
	}
	if getBool != true {
		t.Fatalf("expected getBool=true but got getBool=%v", getBool)
	}
	if !f.Parsed() {
		t.Fatal("f.Parsed() return false after f.Parse() called")
	}

	if testxxxValue != "test.xxx" {
		t.Fatalf("expected testxxxValue to be test.xxx but got %v", testxxxValue)
	}
	err = ParseSkippedFlags(args, goflag.CommandLine)
	if err != nil {
		t.Fatal("expected no error; ParseSkippedFlags", err)
	}
	if testxxxValue != "testvalue" {
		t.Fatalf("expected testxxxValue to be testvalue but got %v", testxxxValue)
	}

	// in fact it is useless. because `go test` called flag.Parse()
	if !goflag.CommandLine.Parsed() {
		t.Fatal("goflag.CommandLine.Parsed() return false after f.Parse() called")
	}
}

func TestToGoflags(t *testing.T) {
	pfs := FlagSet{}
	gfs := goflag.FlagSet{}
	pfs.String("StringFlag", "String value", "String flag usage")
	pfs.Int("IntFlag", 1, "Int flag usage")
	pfs.Uint("UintFlag", 2, "Uint flag usage")
	pfs.Int64("Int64Flag", 3, "Int64 flag usage")
	pfs.Uint64("Uint64Flag", 4, "Uint64 flag usage")
	pfs.Int8("Int8Flag", 5, "Int8 flag usage")
	pfs.Float64("Float64Flag", 6.0, "Float64 flag usage")
	pfs.Duration("DurationFlag", time.Second, "Duration flag usage")
	pfs.Bool("BoolFlag", true, "Bool flag usage")
	pfs.String("deprecated", "Deprecated value", "Deprecated flag usage")
	pfs.MarkDeprecated("deprecated", "obsolete")

	pfs.CopyToGoFlagSet(&gfs)

	// Modify via pfs. Should be visible via gfs because both share the
	// same values.
	for name, value := range map[string]string{
		"StringFlag":  "Modified String value",
		"IntFlag":     "11",
		"UintFlag":    "12",
		"Int64Flag":   "13",
		"Uint64Flag":  "14",
		"Int8Flag":    "15",
		"Float64Flag": "16.0",
		"BoolFlag":    "false",
	} {
		pf := pfs.Lookup(name)
		if pf == nil {
			t.Errorf("%s: not found in pflag flag set", name)
			continue
		}
		if err := pf.Value.Set(value); err != nil {
			t.Errorf("error setting %s = %s: %v", name, value, err)
		}
	}

	// Check that all flags were added and share the same value.
	pfs.VisitAll(func(pf *Flag) {
		gf := gfs.Lookup(pf.Name)
		if gf == nil {
			t.Errorf("%s: not found in Go flag set", pf.Name)
			return
		}
		if gf.Value.String() != pf.Value.String() {
			t.Errorf("%s: expected value %v from Go flag set, got %v",
				pf.Name, pf.Value, gf.Value)
			return
		}
	})

	// Check for unexpected additional flags.
	gfs.VisitAll(func(gf *goflag.Flag) {
		pf := gfs.Lookup(gf.Name)
		if pf == nil {
			t.Errorf("%s: not found in pflag flag set", gf.Name)
			return
		}
	})

	deprecated := gfs.Lookup("deprecated")
	if deprecated == nil {
		t.Error("deprecated: not found in Go flag set")
	} else {
		expectedUsage := "Deprecated flag usage (DEPRECATED: obsolete)"
		if deprecated.Usage != expectedUsage {
			t.Errorf("deprecation remark not added, expected usage %q, got %q", expectedUsage, deprecated.Usage)
		}
	}
}
