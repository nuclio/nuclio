package dyno_test

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/icza/dyno"
)

// Example shows a few of dyno's features, such as getting, setting and appending
// values to / from a dynamic object.
func Example() {
	person := map[string]interface{}{
		"name": map[string]interface{}{
			"first": "Bob",
			"last":  "Archer",
		},
		"age": 22,
		"fruits": []interface{}{
			"apple", "banana",
		},
	}

	// pp prints the person
	pp := func(err error) {
		json.NewEncoder(os.Stdout).Encode(person) // Output JSON
		if err != nil {
			fmt.Println("ERROR:", err)
		}
	}

	// Print initial person and its first name:
	pp(nil)
	v, err := dyno.Get(person, "name", "first")
	fmt.Printf("First name: %v, error: %v\n", v, err)

	// Change first name:
	pp(dyno.Set(person, "Alice", "name", "first"))

	// Change complete name from map to a single string:
	pp(dyno.Set(person, "Alice Archer", "name"))

	// Print and increment age:
	age, err := dyno.GetInt(person, "age")
	fmt.Printf("Age: %v, error: %v\n", age, err)
	pp(dyno.Set(person, age+1, "age"))

	// Change a fruits slice element:
	pp(dyno.Set(person, "lemon", "fruits", 1))

	// Add a new fruit:
	pp(dyno.Append(person, "melon", "fruits"))

	// Output:
	// {"age":22,"fruits":["apple","banana"],"name":{"first":"Bob","last":"Archer"}}
	// First name: Bob, error: <nil>
	// {"age":22,"fruits":["apple","banana"],"name":{"first":"Alice","last":"Archer"}}
	// {"age":22,"fruits":["apple","banana"],"name":"Alice Archer"}
	// Age: 22, error: <nil>
	// {"age":23,"fruits":["apple","banana"],"name":"Alice Archer"}
	// {"age":23,"fruits":["apple","lemon"],"name":"Alice Archer"}
	// {"age":23,"fruits":["apple","lemon","melon"],"name":"Alice Archer"}
}

// Example_jsonEdit shows a simple example how JSON can be edited.
// The password placed in the JSON is masked out.
func Example_jsonEdit() {
	src := `{"login":{"password":"secret","user":"bob"},"name":"cmpA"}`
	fmt.Printf("Input JSON:  %s\n", src)

	var v interface{}
	if err := json.Unmarshal([]byte(src), &v); err != nil {
		panic(err)
	}

	user, err := dyno.Get(v, "login", "user")
	fmt.Printf("User:        %-6s, error: %v\n", user, err)

	password, err := dyno.Get(v, "login", "password")
	fmt.Printf("Password:    %-6s, error: %v\n", password, err)

	// Edit (mask out) password:
	if err = dyno.Set(v, "xxx", "login", "password"); err != nil {
		fmt.Printf("Failed to set password: %v\n", err)
	}

	edited, err := json.Marshal(v)
	fmt.Printf("Edited JSON: %s, error: %v\n", edited, err)

	// Output:
	// Input JSON:  {"login":{"password":"secret","user":"bob"},"name":"cmpA"}
	// User:        bob   , error: <nil>
	// Password:    secret, error: <nil>
	// Edited JSON: {"login":{"password":"xxx","user":"bob"},"name":"cmpA"}, error: <nil>
}

func ExampleGet() {
	m := map[string]interface{}{
		"a": 1,
		"b": map[interface{}]interface{}{
			3: []interface{}{1, "two", 3.3},
		},
	}

	printValue := func(v interface{}, err error) {
		fmt.Printf("Value: %-5v, Error: %v\n", v, err)
	}

	printValue(dyno.Get(m, "a"))
	printValue(dyno.Get(m, "b", 3, 1))
	printValue(dyno.Get(m, "x"))

	sl, _ := dyno.Get(m, "b", 3) // This is: []interface{}{1, "two", 3.3}
	printValue(dyno.Get(sl, 4))

	// Output:
	// Value: 1    , Error: <nil>
	// Value: two  , Error: <nil>
	// Value: <nil>, Error: missing key: x (path element idx: 0)
	// Value: <nil>, Error: index out of range: 4 (path element idx: 0)
}

func ExampleSet() {
	m := map[string]interface{}{
		"a": 1,
		"b": map[string]interface{}{
			"3": []interface{}{1, "two", 3.3},
		},
	}

	printMap := func(err error) {
		json.NewEncoder(os.Stdout).Encode(m) // Output JSON
		if err != nil {
			fmt.Println("ERROR:", err)
		}
	}

	printMap(dyno.Set(m, 2, "a"))
	printMap(dyno.Set(m, "owt", "b", "3", 1))
	printMap(dyno.Set(m, 1, "x"))

	sl, _ := dyno.Get(m, "b", "3") // This is: []interface{}{1, "owt", 3.3}
	printMap(dyno.Set(sl, 1, 4))

	// Output:
	// {"a":2,"b":{"3":[1,"two",3.3]}}
	// {"a":2,"b":{"3":[1,"owt",3.3]}}
	// {"a":2,"b":{"3":[1,"owt",3.3]},"x":1}
	// {"a":2,"b":{"3":[1,"owt",3.3]},"x":1}
	// ERROR: index out of range: 4 (path element idx: 0)
}

func ExampleAppend() {
	m := map[string]interface{}{
		"a": []interface{}{
			"3", 2, []interface{}{1, "two", 3.3},
		},
	}

	printMap := func(err error) {
		fmt.Println(m)
		if err != nil {
			fmt.Println("ERROR:", err)
		}
	}

	printMap(dyno.Append(m, 4, "a"))
	printMap(dyno.Append(m, 9, "a", 2))
	printMap(dyno.Append(m, 1, "x"))

	// Output:
	// map[a:[3 2 [1 two 3.3] 4]]
	// map[a:[3 2 [1 two 3.3 9] 4]]
	// map[a:[3 2 [1 two 3.3 9] 4]]
	// ERROR: missing key: x (path element idx: 0)
}

func ExampleAppendMore() {
	m := map[string]interface{}{
		"ints": []interface{}{
			1, 2,
		},
	}
	err := dyno.AppendMore(m, []interface{}{3, 4, 5}, "ints")
	fmt.Println(m, err)

	// Output:
	// map[ints:[1 2 3 4 5]] <nil>
}

func ExampleDelete() {
	m := map[string]interface{}{
		"name": "Bob",
		"ints": []interface{}{
			1, 2, 3,
		},
	}

	err := dyno.Delete(m, "name")
	fmt.Println(m, err)

	err = dyno.Delete(m, 1, "ints")
	fmt.Println(m, err)

	err = dyno.Delete(m, "ints")
	fmt.Println(m, err)

	// Output:
	// map[ints:[1 2 3]] <nil>
	// map[ints:[1 3]] <nil>
	// map[] <nil>
}

func ExampleConvertMapI2MapS() {
	m := map[interface{}]interface{}{
		1:         "one",
		"numbers": []interface{}{2, 3, 4.4},
	}

	// m cannot be marshaled using encoding/json:
	data, err := json.Marshal(m)
	fmt.Printf("JSON: %q, error: %v\n", data, err)

	m2 := dyno.ConvertMapI2MapS(m)

	// But m2 can be:
	data, err = json.Marshal(m2)
	fmt.Printf("JSON: %s, error: %v\n", data, err)

	// Output:
	// JSON: "", error: json: unsupported type: map[interface {}]interface {}
	// JSON: {"1":"one","numbers":[2,3,4.4]}, error: <nil>
}
