package jsonparser

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
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

type parseError struct {
	message string
	index   int
}

func (err parseError) convert(input []rune) error {
	var (
		before []rune
		after  []rune
	)

	for i := err.index; i >= 0; {
		if len(before) >= 20 {
			break
		}

		if input[i] == ' ' && len(before) > 1 && before[len(before)-1] == ' ' {
			i--
			continue
		}

		if input[i] == '\n' {
			i--
			continue
		}

		before = append(before, input[i])

		i--
	}

	for i := err.index + 1; i < len(input); {
		if len(after) >= 20 {
			break
		}

		if input[i] == ' ' && len(after) > 1 && after[len(after)-1] == ' ' {
			i++
			continue
		}

		if input[i] == '\n' {
			i++
			continue
		}

		after = append(after, input[i])

		i++
	}
	slices.Reverse(before)

	result := append(before, after...)

	cursorPosition := len(before)

	return errors.New(
		fmt.Sprintf("%s\n%s\n%s^", err.message, string(result), strings.Repeat("-", cursorPosition-1)),
	)
}

type boundary struct {
	Value        []rune
	line         int
	lineSize     int
	globalOffset int
}

func (bound boundary) getActualLength() int {
	return len(bound.Value)
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

func (parentBoundary arrayBoundary) parse() (interface{}, *parseError) {
	var result Array

	line := parentBoundary.line
	parentValue := parentBoundary.Value

	for i := 0; i < len(parentValue); i++ {
		if i > 0 {
			if parentValue[i] != ',' {
				return result, &parseError{
					fmt.Sprintf("Expected ',', found %q, line %d", parentValue[i], line),
					parentBoundary.globalOffset + i,
				}
			}
			i++
		}

		boundOffset, newLines := countSpace(parentValue, i)

		i += boundOffset
		line += newLines

		var (
			bound parsable
			err   *parseError
		)

		globalOffset := i + parentBoundary.globalOffset

		if i >= len(parentValue) {
			return result, &parseError{
				fmt.Sprintf("Expected item, found end of array, line %d", line),
				parentBoundary.globalOffset + i,
			}
		}

		switch parentValue[i] {
		case '{':
			bound, err = getObjectBoundary(parentValue, i, globalOffset, line)
		case '[':
			bound, err = getArrayBoundary(parentValue, i, globalOffset, line)
		default:
			bound, err = getLiteralBoundary(parentValue, i, globalOffset, line)
		}

		if err != nil {
			return result, err
		}

		spaces, newLines := countSpace(parentValue, i+bound.getActualLength())

		i += bound.getActualLength() - 1 + spaces
		line += bound.getLineHeight() + newLines

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

func (parentBoundary objectBoundary) parse() (interface{}, *parseError) {
	result := Object{}

	line := parentBoundary.line
	parentValue := parentBoundary.Value

	for i := 0; i < len(parentValue); {
		if i > 0 {
			if parentValue[i] != ',' {
				return result, &parseError{
					fmt.Sprintf("Expected ',', found %q, line %d", parentValue[i], line),
					parentBoundary.globalOffset + i,
				}
			}
			i++
		}

		keyOffset, newLines := countSpace(parentValue, i)

		i += keyOffset
		line += newLines

		keyBoundary, err := getStringBoundary(
			parentValue,
			i,
			parentBoundary.globalOffset+i,
			line,
		)

		if err != nil {
			return result, err
		}

		spaces, newLines := countSpace(parentValue, i+keyBoundary.getActualLength())

		i += keyBoundary.getActualLength() + spaces
		line += newLines

        if len(parentValue) == i {
			return result, &parseError{
				fmt.Sprintf("Expected ':', found end of object, on line %d", line),
				parentBoundary.globalOffset + i-1,
			}
		}

		if parentValue[i] != ':' {
			return result, &parseError{
				fmt.Sprintf("Expected ':', found %q, on line %d", parentValue[i], line),
				parentBoundary.globalOffset + i,
			}
		}

		i++

		boundOffset, newLines := countSpace(parentValue, i)
		i += boundOffset
		line += newLines

		var bound parsable

		globalOffset := parentBoundary.globalOffset + i

        if len(parentValue) == i {
			return result, &parseError{
				fmt.Sprintf("Expected value found end of object, on line %d", line),
				parentBoundary.globalOffset + i-1,
			}
		}

		switch parentValue[i] {
		case '{':
			bound, err = getObjectBoundary(parentValue, i, globalOffset, line)
		case '[':
			bound, err = getArrayBoundary(parentValue, i, globalOffset, line)
		default:
			bound, err = getLiteralBoundary(parentValue, i, globalOffset, line)
		}

		if err != nil {
			return result, err
		}

		spaces, newLines = countSpace(parentValue, i+bound.getActualLength())

		line += bound.getLineHeight() + newLines
		i += bound.getActualLength() + spaces

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

type literalBoundary struct {
	boundary
}

func (bound literalBoundary) parse() (interface{}, *parseError) {
	switch val := string(bound.Value); {
	case val == "null":
		return Null{}, nil
	case val == "true":
		return true, nil
	case val == "false":
		return false, nil
	case bound.Value[0] == '"' && bound.Value[len(bound.Value)-1] == '"':
		return string(bound.Value[1 : len(bound.Value)-1]), nil
	case intRegex.MatchString(val):
		resultValue, err := strconv.Atoi(val)
		if err != nil {
			panic(err)
		}
		return resultValue, nil
	case floatRegex.MatchString(val):
		resultValue, err := strconv.ParseFloat(val, 64)
		if err != nil {
			panic(err)
		}
		return resultValue, nil
	default:
		return nil, &parseError{
			fmt.Sprintf("Expected literal, found \"%s\", line %d", string(bound.Value), bound.line),
			bound.globalOffset,
		}
	}
}

type parsable interface {
	getActualLength() int
	getValue() []rune
	getLineHeight() int
	parse() (interface{}, *parseError)
}

var intRegex = regexp.MustCompile(`^\d+$`)
var floatRegex = regexp.MustCompile(`^\d+.\d+$`)

func getArrayBoundary(input []rune, boundStart int, globalOffset int, line int) (parsable, *parseError) {
	var (
		nestedLevel int
		lineSize    int
		isEscape    bool
		isInString  bool
	)

	for i := boundStart; i < len(input); i++ {
		switch input[i] {
		case '"':
			if !isEscape {
				isInString = !isInString
			}
		case '\n':
			lineSize++
		case '[':
			if !isInString {
				nestedLevel++
			}
		case ']':
			if !isInString {
				nestedLevel--
			}
		}

		isEscape = input[i] == '\\'

		if nestedLevel == 0 {
			return arrayBoundary{
				boundary{
					input[boundStart+1 : i],
					line,
					lineSize,
					globalOffset + 1,
				},
			}, nil
		}
	}

	return arrayBoundary{boundary{make([]rune, 0), 0, 0, 0}}, &parseError{
		fmt.Sprintf("No closing found for array, line %d", line),
		globalOffset,
	}
}

func getObjectBoundary(input []rune, boundStart int, globalOffset int, line int) (parsable, *parseError) {
	var (
		nestedLevel int
		lineSize    int
		isEscape    bool
		isInString  bool
	)

	for i := boundStart; i < len(input); i++ {
		switch input[i] {
		case '"':
			if !isEscape {
				isInString = !isInString
			}
		case '\n':
			lineSize++
		case '{':
			if !isInString {
				nestedLevel++
			}
		case '}':
			if !isInString {
				nestedLevel--
			}
		}

		isEscape = input[i] == '\\'

		if nestedLevel == 0 {
			return objectBoundary{
				boundary{
					input[boundStart+1 : i],
					line,
					lineSize,
					globalOffset + 1,
				},
			}, nil
		}
	}

	return objectBoundary{boundary{make([]rune, 0), 0, 0, 0}}, &parseError{
		fmt.Sprintf("No closing found for object, line %d", line),
		globalOffset,
	}
}

func getStringBoundary(input []rune, boundStart int, globalOffset int, line int) (parsable, *parseError) {
	var (
		isEscape    bool
		placeholder literalBoundary
	)

	if len(input) == boundStart {
		return placeholder, &parseError{
			fmt.Sprintf("Expected '\"', line %d", line),
			globalOffset,
		}
	}

	if input[boundStart] != '"' {
		return placeholder, &parseError{
			fmt.Sprintf("Expected \", found %q, line %d", input[boundStart], line),
			globalOffset,
		}
	}

	for i := boundStart + 1; i < len(input); i++ {
		switch input[i] {
		case '"':
			if !isEscape {
				return literalBoundary{
					boundary{
						input[boundStart : i+1],
						line,
						0,
						globalOffset,
					},
				}, nil
			}
		case '\n':
			return placeholder, &parseError{
				fmt.Sprintf("Unexpected new line, line %d", line),
				globalOffset + (i - boundStart),
			}
		}

		isEscape = input[i] == '\\'
	}

	return placeholder, &parseError{
		fmt.Sprintf("No closing found for string, line %d", line),
		globalOffset,
	}
}

func getLiteralBoundary(input []rune, boundStart int, globalOffset int, line int) (parsable, *parseError) {
	var placeholder literalBoundary

	if input[boundStart] == '"' {
		return getStringBoundary(input, boundStart, globalOffset, line)
	}

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
			return literalBoundary{
				boundary{
					input[boundStart:i],
					line,
					0,
					globalOffset,
				},
			}, nil
		}
	}

	return placeholder,
		&parseError{
			fmt.Sprintf("Unexpected end of value, line %d", line),
			globalOffset,
		}
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
	var err *parseError
	var placeholder JSON

	runed := []rune(input)

	i, initLine := countSpace(runed, 0)

	switch runed[i] {
	case '{':
		bound, err = getObjectBoundary(runed, i, 0, initLine+1)
	case '[':
		bound, err = getArrayBoundary(runed, i, 0, initLine+1)
	default:
		bound, err = getLiteralBoundary(runed, i, 0, initLine+1)
	}

	if err != nil {
		return placeholder, err.convert(runed)
	}

	line := initLine + bound.getLineHeight() + 1

L:
	for i = bound.getActualLength() + i; i < len(runed); i++ {
		switch runed[i] {
		case '\n':
			line++
		case ' ':
		default:
			err = &parseError{
				fmt.Sprintf("Unexpected continuance input %q, line %d", runed[i], line),
				i,
			}
			break L
		}
	}

	if err != nil {
		return placeholder, err.convert(runed)
	}

	parsed, err := bound.parse()

	if err != nil {
		return placeholder, err.convert(runed)
	}

	return JSON{parsed, len(runed)}, nil
}
