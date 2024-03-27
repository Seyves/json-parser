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
type Null struct{}

func countSpace(input []rune, start int) (int, int) {
	var newLines int
	var space int

L:
	for start+space < len(input) {
		switch input[start+space] {
		case '\n':
			newLines++
			fallthrough
		case ' ':
			space++
			continue
		default:
			break L
		}
	}

	return space, newLines
}

type boundary struct {
	Value    []rune
	tail     int
	line     int
	lineSize int
}

func (bound boundary) getActualLength() int {
	return len(bound.Value) + bound.tail
}

func (bound boundary) getLineHeight() int {
	return bound.lineSize
}

func (bound boundary) getValue() []rune {
	return bound.Value
}

type arrayBoundary struct {
	boundary
}

func (bound arrayBoundary) getActualLength() int {
	return bound.boundary.getActualLength() + 2
}

func (parentBoundary arrayBoundary) parse() (interface{}, error) {
	var result Array

	line := parentBoundary.line
	parentValue := parentBoundary.Value

	for i := 0; i < len(parentValue); i++ {
		if i > 0 {
			if parentValue[i] != ',' {
				return result, errors.New(fmt.Sprintf("Expected ',', found %q, line %d", parentValue[i], line))
			}
			i++
		}

		boundOffset, newLines := countSpace(parentValue, i)

		i = i + boundOffset
		line += newLines

		var (
			bound parsable
			err   error
		)

		switch parentValue[i] {
		case '{':
			bound, err = getObjectBoundary(parentValue, i, line)
		case '[':
			bound, err = getArrayBoundary(parentValue, i, line)
		default:
			bound, err = getValueBoundary(parentValue, i, line)
		}

		if err != nil {
			return result, err
		}

		fmt.Println("arrayItem bound:", string(bound.getValue()))
		fmt.Println("arrayItem line height:", bound.getLineHeight())

		i = i + bound.getActualLength() - 1
		line += bound.getLineHeight()

		parsed, err := bound.parse()

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
	return bound.boundary.getActualLength() + 2
}

func (parentBoundary objectBoundary) parse() (interface{}, error) {
	result := Object{}

	line := parentBoundary.line
	parentValue := parentBoundary.Value

	for i := 0; i < len(parentValue); {
		if i > 0 {
			if parentValue[i] != ',' {
				return result, errors.New(fmt.Sprintf("Expected ',', found %q, line %d", parentValue[i], line))
			}
			i++
		}

		keyOffset, newLines := countSpace(parentValue, i)

		i = i + keyOffset
		line += newLines

		keyBoundary, err := getStringBoundary(
			parentValue,
			i,
			line,
		)

		if err != nil {
			return result, err
		}

		fmt.Println("i:", i)
		fmt.Println("key:", string(keyBoundary.getValue()))
		fmt.Println("keyOffset:", keyOffset)
		fmt.Println("keyLength:", keyBoundary.getActualLength())

		i = i + keyBoundary.getActualLength()

		if parentValue[i] != ':' {
			return result, errors.New(fmt.Sprintf("Expected ':', found %q, on line %d", parentValue[i], line))
		}

		i++

		boundOffset, newLines := countSpace(parentValue, i)
		i = i + boundOffset
		line += newLines

		var bound parsable

		switch parentValue[i] {
		case '{':
			bound, err = getObjectBoundary(parentValue, i, line)
		case '[':
			bound, err = getArrayBoundary(parentValue, i, line)
		default:
			bound, err = getValueBoundary(parentValue, i, line)
		}

		fmt.Println("value:", string(bound.getValue()))
		fmt.Println("value offset:", boundOffset)
		fmt.Println("value length:", bound.getActualLength())

		if err != nil {
			return result, err
		}

		line += bound.getLineHeight()
		i = i + bound.getActualLength()

		fmt.Println("end i", string(parentValue[i-5:i]))

		parsedKey, err := keyBoundary.parse()

		if err != nil {
			return result, err
		}

		stringKey, ok := parsedKey.(string)

		if !ok {
			panic("Error while parsing key")
		}

		parsed, err := bound.parse()

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

func (bound valueBoundary) parse() (interface{}, error) {
	switch val := string(bound.Value); {
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
	case bound.Value[0] == '"' && bound.Value[len(bound.Value)-1] == '"':
		return string(bound.Value[1 : len(bound.Value)-1]), nil
	default:
		return nil, errors.New(fmt.Sprintf("Invalid value \"%s\", line %d", string(bound.Value), bound.line))
	}
}

type parsable interface {
	getActualLength() int
	getValue() []rune
	getLineHeight() int
	parse() (interface{}, error)
}

var digitRegex = regexp.MustCompile(`^\d+$`)

func getArrayBoundary(input []rune, boundStart int, line int) (parsable, error) {
	var (
		nestedLevel int
		boundEnd    int
		lineSize    int
		tail        int
	)

	for i := boundStart; i < len(input); i++ {
		switch input[i] {
		case '\n':
			lineSize++
		case '[':
			nestedLevel++
		case ']':
			nestedLevel--
		}

		if nestedLevel == 0 {
			boundEnd = i
			break
		}
	}

	if boundEnd == 0 {
		return arrayBoundary{boundary{make([]rune, 0), 0, 0, 0}},
			errors.New(fmt.Sprintf("No closing found for array, that starts at line %d", line))
	}

	tail, newLines := countSpace(input, boundEnd+1)

	lineSize += newLines

	return arrayBoundary{
		boundary{
			input[boundStart+1 : boundEnd],
			tail,
			line,
			lineSize,
		},
	}, nil
}

func getObjectBoundary(input []rune, boundStart int, line int) (parsable, error) {
	var (
		nestedLevel int
		boundEnd    int
		tail        int
		lineSize    int
	)

	for i := boundStart; i < len(input); i++ {
		switch input[i] {
		case '\n':
			lineSize++
		case '{':
			nestedLevel++
		case '}':
			nestedLevel--
		}

		if nestedLevel == 0 {
			boundEnd = i
			break
		}
	}

	if boundEnd == 0 {
		return objectBoundary{boundary{make([]rune, 0), 0, 0, 0}},
			errors.New(fmt.Sprintf("No closing found for object, that starts at line %d", line))
	}

	tail, newLines := countSpace(input, boundEnd+1)

	lineSize += newLines

	return objectBoundary{
		boundary{
			input[boundStart+1 : boundEnd],
			tail,
			line,
			lineSize,
		},
	}, nil
}

func getStringBoundary(input []rune, boundStart int, line int) (parsable, error) {
	var (
		isEscape    bool
		boundEnd    int
		tail        int
		lineSize    int
		placeholder valueBoundary
	)

	if input[boundStart] != '"' {
		return placeholder, errors.New(
			fmt.Sprintf("Expected \", found %q, line %d", input[boundStart], line),
		)
	}

L:
	for i := boundStart + 1; i < len(input); i++ {
		switch input[i] {
		case '"':
			if !isEscape {
				boundEnd = i
				break L
			}
		case '\\':
			isEscape = true
		case '\n':
			return placeholder, errors.New(
				fmt.Sprintf("Unexpected new line, line %d", line),
			)
		default:
			isEscape = false
		}
	}

	if boundEnd == 0 {
		return placeholder, errors.New(
			fmt.Sprintf("No closing found for string, that starts at line %d", line),
		)
	}

	tail, newLines := countSpace(input, boundEnd+1)

	lineSize += newLines

	return valueBoundary{
		boundary{
			input[boundStart : boundEnd+1],
			tail,
			line,
			lineSize,
		},
	}, nil
}

func getValueBoundary(input []rune, boundStart int, line int) (parsable, error) {
	var boundEnd int
	var tail int
	var lineSize int

	if input[boundStart] == '"' {
		return getStringBoundary(input, boundStart, line)
	}

L:
	for i := boundStart + 1; i < len(input); i++ {
		switch input[i] {
		case ',':
			fallthrough
		case '}':
			fallthrough
		case ']':
			fallthrough
		case '\n':
			fallthrough
		case ' ':
			boundEnd = i - 1
			break L
		}
	}

	if boundEnd == 0 {
		return valueBoundary{
			boundary{make([]rune, 0), 0, 0, 0},
		}, errors.New(fmt.Sprintf("Unexpected end of value, line %d", line))
	}

	tail, newLines := countSpace(input, boundEnd+1)

	lineSize += newLines

	return valueBoundary{
		boundary{
			input[boundStart : boundEnd+1],
			tail,
			line,
			lineSize,
		},
	}, nil
}

type JSON struct {
	view   interface{}
	length int
}

func (json JSON) Get(path string) interface{} {
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

			index, err := strconv.Atoi(string(runed[1 : len(key)-1]))

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
	var bound parsable
	var err error

	runed := []rune(input)

	i, initLine := countSpace(runed, 0)

	switch runed[i] {
	case '{':
		bound, err = getObjectBoundary(runed, i, initLine+1)
	case '[':
		bound, err = getArrayBoundary(runed, i, initLine+1)
	default:
		bound, err = getValueBoundary(runed, i, initLine+1)
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
