package marshal

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

func LoadJson(filePath string, object interface{}) error {
	contents, err := ioutil.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %v | %w", filePath, err)
	}

	err = json.Unmarshal(contents, object)
	if err != nil {
		return fmt.Errorf("failed to unmarshal config: %v | %w", filePath, err)
	}

	return nil
}