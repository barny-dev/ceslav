package main

import (
	"testing"
)

func TestCountRows(t *testing.T) {
	cmd := WithCommand("./out/ceslav").
		WithArgs("row-count", "-k").
		WithStdin("\"Smith\",\"Adam\",\"5000 EUR\",\"2001-12-01\"\n\"Goldman\",\"Anise\",\"8000 EUR\",\"1991-01-23\"\n\"Stevenson\",\"Steve\",\"7500 EUR\",\"1974-07-20\"")

	output, err := cmd.Run()

	if err != nil {
		t.Fatalf("Error running ceslav: %v", err)
	}

	expected := "row count\n3\n"
	if output != expected {
		t.Fatalf("Expected output %q but got %q", expected, output)
	}
}

func TestCountRowsFromFile(t *testing.T) {
	cmd := WithCommand("./out/ceslav").
		WithArgs("row-count", "-i", "testdata/count.csv", "-k")

	output, err := cmd.Run()

	if err != nil {
		t.Fatalf("Error running ceslav: %v", err)
	}

	expected := "row count\n3\n"
	if output != expected {
		t.Fatalf("Expected output %q but got %q", expected, output)
	}
}

func TestSortRowsWithHeaderRowOrderingByColumnSpecifiedByNameWithStringValueAscending(t *testing.T) {
	cmd := WithCommand("./out/ceslav").
		WithArgs("sort", "-j", "-k", "-b", "+s%name").
		WithStdin("name,value\nJessie,1\nChris,2\nAlbert,3\nDonald,4")

	output, err := cmd.Run()

	if err != nil {
		t.Fatalf("Error running ceslav: %v", err)
	}

	expected := "name,value\nAlbert,3\nChris,2\nDonald,4\nJessie,1\n"
	if output != expected {
		t.Fatalf("Expected output %q but got %q", expected, output)
	}
}

func TestSortRowsWithHeaderRowOrderingByColumnSpecifiedByNameWithStringValueDescending(t *testing.T) {
	cmd := WithCommand("./out/ceslav").
		WithArgs("sort", "-j", "-k", "-b", "-s%name").
		WithStdin("name,value\nJessie,1\nChris,2\nAlbert,3\nDonald,4")

	output, err := cmd.Run()

	if err != nil {
		t.Fatalf("Error running ceslav: %v", err)
	}

	expected := "name,value\nJessie,1\nDonald,4\nChris,2\nAlbert,3\n"
	if output != expected {
		t.Fatalf("Expected output %q but got %q", expected, output)
	}
}

func TestSortRowsWithHeaderRowOrderingByColumnSpecifiedByIndexWithStringValueAscending(t *testing.T) {
	cmd := WithCommand("./out/ceslav").
		WithArgs("sort", "-j", "-k", "-b", "+s#0").
		WithStdin("name,value\nJessie,1\nChris,2\nAlbert,3\nDonald,4")

	output, err := cmd.Run()

	if err != nil {
		t.Fatalf("Error running ceslav: %v", err)
	}

	expected := "name,value\nAlbert,3\nChris,2\nDonald,4\nJessie,1\n"
	if output != expected {
		t.Fatalf("Expected output %q but got %q", expected, output)
	}
}

func TestSortRowsWithHeaderRowOrderingByColumnSpecifiedByIndexWithStringValueDescending(t *testing.T) {
	cmd := WithCommand("./out/ceslav").
		WithArgs("sort", "-j", "-k", "-b", "-s#0").
		WithStdin("name,value\nJessie,1\nChris,2\nAlbert,3\nDonald,4")

	output, err := cmd.Run()

	if err != nil {
		t.Fatalf("Error running ceslav: %v", err)
	}

	expected := "name,value\nJessie,1\nDonald,4\nChris,2\nAlbert,3\n"
	if output != expected {
		t.Fatalf("Expected output %q but got %q", expected, output)
	}
}

func TestSortRowsWithHeaderRowOrderingByColumnSpecifiedByIndexWithDecimalValueAscending(t *testing.T) {
	cmd := WithCommand("./out/ceslav").
		WithArgs("sort", "-j", "-k", "-b", "+d#1").
		WithStdin("name,value\nKarol,99\nStephanie,-1\nAlbert,62\nDonald,4\n")

	output, err := cmd.Run()

	if err != nil {
		t.Fatalf("Error running ceslav: %v", err)
	}

	expected := "name,value\nStephanie,-1\nDonald,4\nAlbert,62\nKarol,99\n"
	if output != expected {
		t.Fatalf("Expected output %q but got %q", expected, output)
	}
}

func TestSortRowsWithHeaderRowOrderingByColumnSpecifiedByIndexWithDecimalValueDescending(t *testing.T) {
	cmd := WithCommand("./out/ceslav").
		WithArgs("sort", "-j", "-k", "-b", "-d#1").
		WithStdin("name,value\nKarol,99\nStephanie,-1\nAlbert,62\nDonald,4\n")

	output, err := cmd.Run()

	if err != nil {
		t.Fatalf("Error running ceslav: %v", err)
	}

	expected := "name,value\nKarol,99\nAlbert,62\nDonald,4\nStephanie,-1\n"
	if output != expected {
		t.Fatalf("Expected output %q but got %q", expected, output)
	}
}

func TestSortRowsWithoutHeaderRowOrderingByColumnSpecifiedByIndexWithDecimalValueDescending(t *testing.T) {
	cmd := WithCommand("./out/ceslav").
		WithArgs("sort", "-b", "-d#1").
		WithStdin("Karol,99\nStephanie,-1\nAlbert,62\nDonald,4\n")

	output, err := cmd.Run()

	if err != nil {
		t.Fatalf("Error running ceslav: %v", err)
	}

	expected := "Karol,99\nAlbert,62\nDonald,4\nStephanie,-1\n"
	if output != expected {
		t.Fatalf("Expected output %q but got %q", expected, output)
	}
}

func TestSortRowsWithoutHeaderRowOrderingByColumnSpecifiedByIndexWithDecimalValueAscending(t *testing.T) {
	cmd := WithCommand("./out/ceslav").
		WithArgs("sort", "-b", "+d#1").
		WithStdin("Karol,99\nStephanie,-1\nAlbert,62\nDonald,4\n")

	output, err := cmd.Run()

	if err != nil {
		t.Fatalf("Error running ceslav: %v", err)
	}

	expected := "Stephanie,-1\nDonald,4\nAlbert,62\nKarol,99\n"
	if output != expected {
		t.Fatalf("Expected output %q but got %q", expected, output)
	}
}
