package main

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

type Object map[string]interface{}
type Array []interface{}
type Null struct {}

type boundary struct {
    Value []rune
}

func (bound boundary) getActualLength () int {
    return len(bound.Value)
}

func (bound boundary) getValue() []rune {
    return bound.Value
}

type arrayBoundary struct {
    boundary
}

func (bound arrayBoundary) getActualLength() int {
    return len(bound.Value) + 2
}

func (parentBoundary arrayBoundary) parse () (interface{}, error) {
	var boundaries []parsable
    var result Array

    parentValue := parentBoundary.Value

	for i := 0; i < len(parentValue); i++ {
		if i > 0 && parentValue[i] != ',' {
			return result, errors.New(fmt.Sprintf("Expected ',', found %q", parentValue[i]))
		}

		if i > 0 {
			i++
		}

		var bound parsable
		var err error

		switch parentValue[i] {
		case '{':
			bound, err = getObjectBoundary(parentValue, i)
			break
		case '[':
			bound, err = getArrayBoundary(parentValue, i)
			break
		default:
			bound, err = getValueBoundary(parentValue, i)
			break
		}

		if err != nil {
			return result, err
		}

		boundaries = append(boundaries, bound)

		i = i + bound.getActualLength() - 1
	}

	for _, item := range boundaries {
        parsed, err := item.parse()

        if err != nil {
            return result, err
        }

        sliceParsed, isSlice := parsed.(Array)

        if isSlice {
            result = append(result, sliceParsed...)
        }

        mapParsed, isMap := parsed.(Object)

        if isMap {
            result = append(result, mapParsed)
        }

        if !isMap && !isSlice {
            result = append(result, parsed)
        }
    }


	return result, nil
}

type objectBoundary struct {
    boundary
}

func (bound objectBoundary) getActualLength() int {
    return len(bound.Value) + 2
}

func (parentBoundary objectBoundary) parse () (interface{}, error) {
	var boundaries [][2]parsable
    result := Object{}

    parentValue := parentBoundary.Value

	for i := 0; i < len(parentValue); i++ {
		if i > 0 && parentValue[i] != ',' {
			return result, errors.New(fmt.Sprintf("Expected ',', found %q", parentValue[i]))
		}

        if (i > 0) {
            i++
        }

		keyBoundary, err := getStringBoundary(parentValue, i)

		if err != nil {
			return result, err
		}

		i = i + keyBoundary.getActualLength()

		if parentValue[i] != ':' {
			return result, errors.New(fmt.Sprintf("Expected ':', found %q", parentValue[i]))
		}

		i++

		var bound parsable

		switch parentValue[i] {
		case '{':
			bound, err = getObjectBoundary(parentValue, i)
			break
		case '[':
			bound, err = getArrayBoundary(parentValue, i)
			break
		default:
			bound, err = getValueBoundary(parentValue, i)
			break
		}

		if err != nil {
			return result, err
		}

		boundaries = append(boundaries, [2]parsable{keyBoundary, bound})

		i = i + bound.getActualLength() - 1
	}

	for _, keyValue := range boundaries {
        parsedKey, err := keyValue[0].parse()

        if err != nil {
            return result, err
        }

        stringKey, ok := parsedKey.(string)
        
        if !ok {
            panic("Error while parsing key")
        }

        parsed, err := keyValue[1].parse()

        if err != nil {
            return result, err
        }

        result[stringKey] = parsed
	}

	return result, nil
}

type valueBoundary struct {
    boundary
}

func (parentBoundary valueBoundary) parse () (interface{}, error) {
	switch val := string(parentBoundary.Value); {
	case val == "null":
        return Null{}, nil
	case val == "true":
        return true, nil
	case val == "false":
        return false, nil
	case digitRegex.MatchString(val):
		resultValue, err := strconv.Atoi(val)
		if err != nil {
			panic(err)
		}
        return resultValue, nil
	case parentBoundary.Value[0] == '"' && parentBoundary.Value[parentBoundary.getActualLength()-1] == '"':
		return string(parentBoundary.Value[1 : parentBoundary.getActualLength()-1]), nil
    default:
        return nil, errors.New(fmt.Sprintf("Invalid value: %s", string(parentBoundary.Value)))
	}
}

type parsable interface {
    getActualLength() int
    getValue() []rune
    parse() (interface{}, error)
}

var digitRegex = regexp.MustCompile(`^\d+$`)

func getArrayBoundary(input []rune, startIndex int) (parsable, error) {
    var nestedLevel int

	for endIndex := startIndex; endIndex < len(input); endIndex++ {
		char := input[endIndex]

		if char == '[' {
			nestedLevel++
		}
		if char == ']' {
			nestedLevel--
			if nestedLevel == 0 {
				return arrayBoundary{boundary{input[startIndex+1: endIndex]}}, nil
			}
		}
		if char == ',' {
			if nestedLevel == 0 {
				return arrayBoundary{boundary{input[startIndex+1: endIndex]}}, nil
			}
		}
	}

	return arrayBoundary{boundary{make([]rune, 0)}}, 
        errors.New(fmt.Sprintf("No closing found for array, starting at %d", startIndex))
}

func getObjectBoundary(input []rune, startIndex int) (parsable, error) {
    var nestedLevel int

	for i := startIndex; i < len(input); i++ {
		char := input[i]

		if char == '{' {
			nestedLevel++
		}
		if char == '}' {
			nestedLevel--
			if nestedLevel == 0 {
				return objectBoundary{boundary{input[startIndex+1: i]}}, nil
			}
		}
		if char == ',' {
			if nestedLevel == 0 {
				return objectBoundary{boundary{input[startIndex+1 : i]}}, nil
			}
		}
	}

	return objectBoundary{boundary{make([]rune, 0)}},
		errors.New(fmt.Sprintf("No closing found for object, starting at %d", startIndex))
}

func getStringBoundary(input []rune, startIndex int) (parsable, error) {
    var isEscape bool

	for endIndex := startIndex + 1; endIndex < len(input); endIndex++ {
		char := input[endIndex]

		if char == '"' && !isEscape {
            return valueBoundary{boundary{input[startIndex:endIndex+1]}}, nil
		}

		if char == '\\' {
			isEscape = true
		} else {
			isEscape = false
		}
	}

	return valueBoundary{boundary{make([]rune, 0)}}, 
        errors.New(fmt.Sprintf("No closing found for string, starting at %d", startIndex))
}

func getValueBoundary(input []rune, startIndex int) (parsable, error) {
	if input[startIndex] == '"' {
		return getStringBoundary(input, startIndex)
	}

	for endIndex := startIndex + 1; endIndex < len(input); endIndex++ {
		char := input[endIndex]

		if char == ',' || char == '}' || char == ']' {
            return valueBoundary{boundary{input[startIndex:endIndex]}}, nil
		}
	}

	return valueBoundary{boundary{input[startIndex:]}}, nil
}

type JSON struct {
    view interface{}
    length int
}

func (json JSON) Get(path string) (interface{}) {
    if path == "" {
        return json.view
    }

    keys := strings.Split(path, ".") 

    current := json.view

    for _, key := range keys {
        runed := []rune(key)

        if runed[0] == '[' && runed[len(key)-1] == ']' {
            currentSlice, ok := current.(Array)            

            if !ok {
                return nil
            }

            index, err := strconv.Atoi(string(runed[1:len(key)-1]))

            if err != nil {
                panic(err)
            }

            current = currentSlice[index]
        } else {
            currentMap, ok := current.(Object)            

            if !ok {
                return nil
            }

            current = currentMap[key]
        }
    }

    return current
}

func Create(input string) (JSON, error) {
    replacer := strings.NewReplacer(" ", "", "\n", "")

	processed := replacer.Replace(input)

    var bound parsable
    var err error 

    runed := []rune(processed)

    switch runed[0] {
    case '{':
        bound, err = getObjectBoundary(runed, 0)
        break
    case '[':
        bound, err = getArrayBoundary(runed, 0)
        break
    default:
        bound, err = getValueBoundary(runed, 0)
        break
    }

    if err != nil {
        return JSON{"", 0}, err
    }

    parsed, err := bound.parse()

    if err != nil {
        return JSON{"", 0}, err
    }

    return JSON{parsed, len(runed)}, nil
}

func main() {
	file, err := os.ReadFile("./data.json")

	if err != nil {
		panic(err)
	}

    json, err := Create(string(file))

    if err != nil {
        fmt.Println(err)
    }

    someValue, ok := json.Get("").(Array)

    if ok {
        fmt.Println(someValue)
    }
}
