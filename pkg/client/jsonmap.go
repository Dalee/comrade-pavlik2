package client

//
// Just a lot of small helpers to simplify work
// with Generic JsonMap structure.
//

import (
	"fmt"
)

type (
	JsonMap map[string]interface{}
)

var (
	errFmtKeyIsAbsent = "Key: %s not found in map"
)

// GetString - helper method to extract string value from underlying map
func (m *JsonMap) GetString(key string) (string, error) {
	var valueRaw interface{}
	var value string
	var ok bool

	if valueRaw, ok = (*m)[key]; !ok {
		return "", fmt.Errorf(errFmtKeyIsAbsent, key)
	}

	if value, ok = valueRaw.(string); !ok {
		return "", fmt.Errorf("Value is not a string for key: %s", key)
	}

	return value, nil
}

// GetListInterface - helper method to extract array/slice from underlying map
func (m *JsonMap) GetListInterface(key string, def *[]interface{}) (*[]interface{}, error) {
	var valueRaw interface{}
	var value []interface{}
	var ok bool

	if valueRaw, ok = (*m)[key]; !ok {
		return def, fmt.Errorf(errFmtKeyIsAbsent, key)
	}

	if value, ok = valueRaw.([]interface{}); !ok {
		return def, fmt.Errorf("Value is not an []interface{} type for key: %s", key)
	}

	return &value, nil
}

// GetInterface - helper method to extract any value from underlying map
func (m *JsonMap) GetInterface(key string, def interface{}) (interface{}, error) {
	var value interface{}
	var ok bool

	if value, ok = (*m)[key]; !ok {
		return def, fmt.Errorf(errFmtKeyIsAbsent, key)
	}

	if value, ok = value.([]interface{}); !ok {
		return def, fmt.Errorf("Value is not an interface{} type for key: %s", key)
	}

	return value, nil
}

// GetMapInterface - helper method to extract map from underlying map
func (m *JsonMap) GetMapInterface(key string, def map[string]interface{}) (*map[string]interface{}, error) {
	var valueRaw interface{}
	var value map[string]interface{}
	var ok bool

	if valueRaw, ok = (*m)[key]; !ok {
		return nil, fmt.Errorf(errFmtKeyIsAbsent, key)
	}

	if value, ok = valueRaw.(map[string]interface{}); !ok {
		return nil, fmt.Errorf("Value is not an map[string]interface{} type for key: %s", key)
	}

	return &value, nil
}
