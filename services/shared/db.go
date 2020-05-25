package shared

import (
	"errors"
	"os"
)

// GetDBInfo returns the db region and table name from the env vars
func GetDBInfo() (string, string, error) {
	region, exists := os.LookupEnv("table_region")
	if !exists {
		errors.New("table_region env var doesn't exist")
	}
	name, exists := os.LookupEnv("table_name")
	if !exists {
		errors.New("table_name env var doesn't exist")
	}

	return region, name, nil
}
