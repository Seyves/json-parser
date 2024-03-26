package main

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

type Boundary struct {
    Value []rune
}

func (boundary Boundary) getActualLength () int {
    return len(boundary.Value)
}

func (boundary Boundary) getValue() []rune {
    return boundary.Value
}

type ArrayBoundary struct {
    Boundary
}

func (boundary ArrayBoundary) getActualLength() int {
    return len(boundary.Value) + 2
}

func (parentBoundary ArrayBoundary) parse (parent *Node, key string) ([]Node, error) {
	var boundaries []Parsable
	var nodes []Node

    parentValue := parentBoundary.Value

	for i := 0; i < len(parentValue); i++ {
        fmt.Println("i: ", string(parentValue[i]))
		if i > 0 && parentValue[i] != ',' {
			return nodes, errors.New(fmt.Sprintf("Expected ',', found %q", parentValue[i]))
		}

		if i > 0 {
			i++
		}

		var boundary Parsable
		var err error

		switch parentValue[i] {
		case '{':
			boundary, err = getObjectBoundary(parentValue, i)
			break
		case '[':
			boundary, err = getArrayBoundary(parentValue, i)
			break
		default:
			boundary, err = getValueBoundary(parentValue, i)
			break
		}

		if err != nil {
			return nodes, err
		}

		boundaries = append(boundaries, boundary)

		i = i + boundary.getActualLength() - 1
	}

	parentNode := Node{parent, key, "array", nil}

	nodes = append(nodes, parentNode)

	for i, item := range boundaries {
        parsed, err := item.parse(&parentNode, fmt.Sprint(i))

        if err != nil {
            return nodes, err
        }

        nodes = append(nodes, parsed...)
    }

	return nodes, nil
}

type ObjectBoundary struct {
    Boundary
}

func (boundary ObjectBoundary) getActualLength() int {
    return len(boundary.Value) + 2
}

func (parentBoundary ObjectBoundary) parse (parent *Node, key string) ([]Node, error) {
	var boundaries [][2]Parsable
	var nodes []Node

    parentValue := parentBoundary.Value

	for i := 0; i < len(parentValue); i++ {
		fmt.Println("i:", string(parentValue[i]))

		if i > 0 && parentValue[i] != ',' {
			return nodes, errors.New(fmt.Sprintf("Expected ',', found %q", parentValue[i]))
		}

        if (i > 0) {
            i++
        }

		keyBoundary, err := getStringBoundary(parentValue, i)

		fmt.Println("key:", string(keyBoundary.getValue()))
  
		if err != nil {
			return nodes, err
		}

		i = i + keyBoundary.getActualLength()

		if parentValue[i] != ':' {
			return nodes, errors.New(fmt.Sprintf("Expected ':', found %q", parentValue[i]))
		}

		i++

		var boundary Parsable

		switch parentValue[i] {
		case '{':
			boundary, err = getObjectBoundary(parentValue, i)
			break
		case '[':
			boundary, err = getArrayBoundary(parentValue, i)
			break
		default:
			boundary, err = getValueBoundary(parentValue, i)
			break
		}

		fmt.Println("endIndex: ", string(key), string(boundary.getValue()))
		if err != nil {
			return nodes, err
		}

		boundaries = append(boundaries, [2]Parsable{keyBoundary, boundary})

		i = i + boundary.getActualLength() - 1
	}

	parentNode := Node{parent, key, "object", nil}

	nodes = append(nodes, parentNode)

	for _, keyValue := range boundaries {
        parsed, err := keyValue[1].parse(&parentNode, string(keyValue[0].getValue()))

        if err != nil {
            return nodes, err
        }

        nodes = append(nodes, parsed...)
	}

	return nodes, nil
}

type ValueBoundary struct {
    Boundary
}

func (parentBoundary ValueBoundary) parse (parent *Node, key string) ([]Node, error) {
    var node Node

	switch val := string(parentBoundary.Value); {
	case val == "null":
		node = Node{parent, key, "null", nil}
        break
	case val == "undefined":
		node = Node{parent, key, "undefined", nil}
        break
	case val == "true":
		node = Node{parent, key, "bool", true}
        break
	case val == "false":
		node = Node{parent, key, "bool", false}
	case digitRegex.MatchString(val):
		resultValue, err := strconv.Atoi(val)
		if err != nil {
			panic(err)
		}
		node = Node{parent, key, "digit", resultValue}
	case parentBoundary.Value[0] == '"' && parentBoundary.Value[parentBoundary.getActualLength()-1] == '"':
		node = Node{parent, key, "string", string(parentBoundary.Value[1 : parentBoundary.getActualLength()-1])}
    default:
        return []Node{{parent, key, "", nil}}, errors.New(fmt.Sprintf("Invalid value: %s", string(parentBoundary.Value)))
	}

    return []Node{node}, nil
}

type Parsable interface {
    getActualLength() int
    getValue() []rune
    parse(*Node, string) ([]Node, error)
}

type Node struct {
	parent *Node
	key    string
	kind   string
	value  any
}

var digitRegex = regexp.MustCompile(`^\d+$`)

func getArrayBoundary(input []rune, startIndex int) (Parsable, error) {
    var nestedLevel int

	for endIndex := startIndex; endIndex < len(input); endIndex++ {
		char := input[endIndex]

		if char == '[' {
			nestedLevel++
		}
		if char == ']' {
			nestedLevel--
			if nestedLevel == 0 {
				return ArrayBoundary{Boundary{input[startIndex+1: endIndex]}}, nil
			}
		}
		if char == ',' {
			if nestedLevel == 0 {
				return ArrayBoundary{Boundary{input[startIndex+1: endIndex]}}, nil
			}
		}
	}

	return ArrayBoundary{Boundary{make([]rune, 0)}}, 
        errors.New(fmt.Sprintf("No closing found for array, starting at %d", startIndex))
}

func getObjectBoundary(input []rune, startIndex int) (Parsable, error) {
    var nestedLevel int

	for i := startIndex; i < len(input); i++ {
		char := input[i]

		if char == '{' {
			nestedLevel++
		}
		if char == '}' {
			nestedLevel--
			if nestedLevel == 0 {
				return ObjectBoundary{Boundary{input[startIndex+1: i]}}, nil
			}
		}
		if char == ',' {
			if nestedLevel == 0 {
				return ObjectBoundary{Boundary{input[startIndex+1 : i]}}, nil
			}
		}
	}

	return ObjectBoundary{Boundary{make([]rune, 0)}},
		errors.New(fmt.Sprintf("No closing found for object, starting at %d", startIndex))
}

func getStringBoundary(input []rune, startIndex int) (Parsable, error) {
    var isEscape bool

	for endIndex := startIndex + 1; endIndex < len(input); endIndex++ {
		char := input[endIndex]

		if char == '"' && !isEscape {
            return ValueBoundary{Boundary{input[startIndex:endIndex+1]}}, nil
		}

		if char == '\\' {
			isEscape = true
		} else {
			isEscape = false
		}
	}

	return ValueBoundary{Boundary{make([]rune, 0)}}, 
        errors.New(fmt.Sprintf("No closing found for string, starting at %d", startIndex))
}

func getValueBoundary(input []rune, startIndex int) (Parsable, error) {
	if input[startIndex] == '"' {
		return getStringBoundary(input, startIndex)
	}

	fmt.Println("Not string value")

	for endIndex := startIndex + 1; endIndex < len(input); endIndex++ {
		char := input[endIndex]

		if char == ',' || char == '}' || char == ']' {
            return ValueBoundary{Boundary{input[startIndex:endIndex]}}, nil
		}
	}

	return ValueBoundary{Boundary{input[startIndex:]}}, nil
}

func main() {
	file, err := os.ReadFile("./data.json")

	if err != nil {
		panic(err)
	}

	processed := strings.ReplaceAll(string(file), "\n", "")
	processed = strings.ReplaceAll(string(processed), " ", "")

	processed = processed[1 : len(processed)-1]

    boundary := ArrayBoundary{Boundary{[]rune(processed)}}

	fmt.Println(boundary.parse(nil, "", ))
}
