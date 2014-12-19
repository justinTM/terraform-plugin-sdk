package schema

import (
	"fmt"
	"strings"
)

// MapFieldReader reads fields out of an untyped map[string]string to
// the best of its ability.
type MapFieldReader struct {
	Map map[string]string
}

func (r *MapFieldReader) ReadField(
	address []string, schema *Schema) (interface{}, bool, bool, error) {
	k := strings.Join(address, ".")

	switch schema.Type {
	case TypeBool:
		fallthrough
	case TypeInt:
		fallthrough
	case TypeString:
		return r.readPrimitive(k, schema)
	case TypeList:
		return readListField(r, k, schema)
	case TypeMap:
		return r.readMap(k)
	case TypeSet:
		return r.readSet(k, schema)
	case typeObject:
		return readObjectField(r, k, schema.Elem.(map[string]*Schema))
	default:
		panic(fmt.Sprintf("Unknown type: %#v", schema.Type))
	}
}

func (r *MapFieldReader) readMap(k string) (interface{}, bool, bool, error) {
	result := make(map[string]interface{})
	resultSet := false

	prefix := k + "."
	for k, v := range r.Map {
		if !strings.HasPrefix(k, prefix) {
			continue
		}

		result[k[len(prefix):]] = v
		resultSet = true
	}

	var resultVal interface{}
	if resultSet {
		resultVal = result
	}

	return resultVal, resultSet, false, nil
}

func (r *MapFieldReader) readPrimitive(
	k string, schema *Schema) (interface{}, bool, bool, error) {
	result, ok := r.Map[k]
	if !ok {
		return nil, false, false, nil
	}

	returnVal, err := stringToPrimitive(result, false, schema)
	if err != nil {
		return nil, false, false, err
	}

	return returnVal, true, false, nil
}

func (r *MapFieldReader) readSet(
	k string, schema *Schema) (interface{}, bool, bool, error) {
	// Get the number of elements in the list
	countRaw, countOk, countComputed, err := r.readPrimitive(
		k+".#", &Schema{Type: TypeInt})
	if err != nil {
		return nil, false, false, err
	}
	if !countOk {
		// No count, means we have no list
		countRaw = 0
	}

	// Create the set that will be our result
	set := &Set{F: schema.Set}

	// If we have an empty list, then return an empty list
	if countComputed || countRaw.(int) == 0 {
		return set, true, countComputed, nil
	}

	// Get the schema for the elements
	var elemSchema *Schema
	switch t := schema.Elem.(type) {
	case *Resource:
		elemSchema = &Schema{
			Type: typeObject,
			Elem: t.Schema,
		}
	case *Schema:
		elemSchema = t
	}

	// Go through the map and find all the set items
	prefix := k + "."
	for k, _ := range r.Map {
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		if strings.HasPrefix(k, prefix+"#") {
			// Ignore the count field
			continue
		}

		// Split the key, since it might be a sub-object like "idx.field"
		parts := strings.Split(k[len(prefix):], ".")
		idx := parts[0]

		v, ok, _, err := r.ReadField([]string{prefix + idx}, elemSchema)
		if err != nil {
			return nil, false, false, err
		}
		if !ok {
			// This shouldn't happen because we just verified it does exist
			panic("missing field in set: " + k + "." + idx)
		}

		set.Add(v)
	}

	return set, true, false, nil
}
