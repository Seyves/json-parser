# json-parser

Json parser I built for learning purposes. It converts arrays to slices and object to maps. 

For type safety it is recommended to use type assertions (lib has builtin **Array**, **Object**, and **Null** types)

Usage example:
```go
    file, err := os.ReadFile("./json.json")

    if err != nil {
        return err
    }

    json, err := jsonparser.Create(string(file))

    if err != nil {
        return err
    }

    value, ok := json.Get("users[0].hobbies").(jsonparser.Array)
```
