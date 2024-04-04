package jsonparser

import (
	"fmt"
	"os"
	"testing"
)

func TestValid(t *testing.T) {
    passEntries, err := os.ReadDir("./test-samples/valid")

    if err != nil {
        t.Fatal(err)
    }

    for _, entry := range passEntries {
        file, err := os.ReadFile(fmt.Sprintf("./test-samples/valid/%s", entry.Name()))

        if err != nil {
            t.Fatal(err)
        }

        _, err = Create(string(file))

        if err != nil {
            t.Error(entry.Name(), "\n", err)
        }
    }
}

func TestInvalid(t *testing.T) {
    failEntries, err := os.ReadDir("./test-samples/invalid")

    if err != nil {
        t.Fatal(err)
    }

    for _, entry := range failEntries {
        file, err := os.ReadFile(fmt.Sprintf("./test-samples/invalid/%s", entry.Name()))

        if err != nil {
            t.Fatal(err)
        }

        _, err = Create(string(file))

        if err == nil {
            t.Error(entry.Name(), "\n", "Invalid json passed")
        }
    }
}
